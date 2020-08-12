// +build integration annotated

package annotation

import (
	"encoding/json"
	"fmt"
	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
	"reflect"
	"testing"
)

const (
	// runs inside skupper-controller's pod
	dataEndpoint = "http://127.0.0.1:8080/DATA"
)

var (
	curlOpts = tools.CurlOpts{
		Silent:   true,
		Username: "admin",
		Password: "admin",
		Timeout:  10,
	}
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags(m)
}

// TestAnnotatedResources deploy resources with Skupper annotations before
// Skupper is available in the respective namespaces. Then we run a test
// table that starts verifying initial state and then applies modifications
// to validate if Skupper is reacting as expected.
func TestAnnotatedResources(t *testing.T) {

	testTable := []test{
		{
			name:          "services-pre-annotated",
			doc:           "Services are pre-annotated before Skupper is deployed and exposed service list matches",
			modification:  nil,
			expectedSites: 2,
			expectedServicesProto: map[string]string{
				"nginx-1-dep-web":          "tcp",
				"nginx-2-dep-web":          "tcp",
				"nginx-1-svc-exp-notarget": "tcp",
				"nginx-2-svc-exp-notarget": "tcp",
				"nginx-1-svc-exp-target":   "http",
				"nginx-2-svc-exp-target":   "http",
			},
		},
		{
			name:          "services-protocol-switch",
			doc:           "Switches the protocol for all services and deployment and validate updated service list",
			modification:  switchProtocols,
			expectedSites: 2,
			expectedServicesProto: map[string]string{
				"nginx-1-dep-web":          "http",
				"nginx-2-dep-web":          "http",
				"nginx-1-svc-exp-notarget": "http",
				"nginx-2-svc-exp-notarget": "http",
				"nginx-1-svc-exp-target":   "tcp",
				"nginx-2-svc-exp-target":   "tcp",
			},
		},
		{
			name:                  "services-annotation-removed",
			doc:                   "Remove Skupper annotations from services and deployment and validate available service list",
			modification:          removeAnnotation,
			expectedSites:         2,
			expectedServicesProto: map[string]string{},
		},
		{
			name:          "services-annotated",
			doc:           "Services are annotated after Skupper is deployed and exposed service list matches",
			modification:  addAnnotation,
			expectedSites: 2,
			expectedServicesProto: map[string]string{
				"nginx-1-dep-web":          "tcp",
				"nginx-2-dep-web":          "tcp",
				"nginx-1-svc-exp-notarget": "tcp",
				"nginx-2-svc-exp-notarget": "tcp",
				"nginx-1-svc-exp-target":   "http",
				"nginx-2-svc-exp-target":   "http",
			},
		},
	}

	// Test flow
	defer TearDown(t)
	base.HandleInterruptSignal(t, TearDown)

	// 1. Initialize Namespaces
	Setup(t)

	// 2. Deploying the Pre-annotated resources
	DeployResources(t)

	// 3. Deploys Skupper and create a VAN between both clusters
	CreateVan(t)

	// 4. Iterate through test table and run each test
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			var err error

			// 4.1. If test expects modifications to be performed, run them
			if test.modification != nil {
				test.modification(t)
			}

			// 4.2 Validate services from DATA endpoint (with retries)
			var resp *tools.CurlResponse
			var consoleData ConsoleData
			err = utils.Retry(interval, 10, func() (bool, error) {
				resp, err = tools.Curl(cluster1.KubeClient, cluster1.RestConfig, cluster1.Namespace, "", dataEndpoint, curlOpts)
				if err != nil {
					return false, nil
				}

				// 4.2.1. Parsing ConsoleData response
				if err = json.Unmarshal([]byte(resp.Body), &consoleData); err != nil {
					return false, nil
				}

				// 4.2.2. retry till expected amount of sites and services match
				if test.expectedSites != len(consoleData.Sites) || len(test.expectedServicesProto) != len(consoleData.Services) {
					return false, nil
				}

				// 4.2.3. populating map of services/protocols returned
				servicesFound := map[string]string{}
				for _, svc := range consoleData.Services {
					svcMap, ok := svc.(map[string]interface{})
					assert.Assert(t, ok)
					address, ok := svcMap["address"].(string)
					assert.Assert(t, ok)
					protocol, ok := svcMap["protocol"].(string)
					assert.Assert(t, ok)
					servicesFound[address] = protocol
				}

				// 4.2.4. only consider as done if all match (otherwise it will timeout)
				res := reflect.DeepEqual(test.expectedServicesProto, servicesFound)
				return res, nil
			})
			assert.Assert(t, err, "timed out waiting for expected services/protocol list to match")

			// 4.3. Validating all exposed services are reachable across clusters
			for _, cluster := range []*vanClient.VanClient{cluster1, cluster2} {
				for svc, _ := range test.expectedServicesProto {
					t.Logf("validating communication with service %s through %s", svc, cluster.Namespace)
					// reaching service through proxy-controller's pod (with some attempts to make sure bridge is connected)
					err = utils.Retry(interval, 10, func() (bool, error) {
						endpoint := fmt.Sprintf("http://%s:8080", svc)
						resp, err = tools.Curl(cluster.KubeClient, cluster.RestConfig, cluster.Namespace, "", endpoint, tools.CurlOpts{})
						if err != nil {
							return false, nil
						}
						return true, nil
					})
					assert.Assert(t, err, "unable to reach service %s through %s", svc, cluster.Namespace)
					assert.Equal(t, resp.StatusCode, 200, "bad response received from service %s through %s", svc, cluster.Namespace)
					assert.Assert(t, resp.Body != "", "empty response body received from service %s through %s", svc, cluster.Namespace)
				}
			}
		})
	}
}

// TODO ConsoleData and Site here for now but ideally those types
//      should be moved from main to a separate package (see: issue #203)
type ConsoleData struct {
	Sites    []Site        `json:"sites"`
	Services []interface{} `json:"services"`
}

type Site struct {
	SiteName  string   `json:"site_name"`
	SiteId    string   `json:"site_id"`
	Connected []string `json:"connected"`
	Namespace string   `json:"namespace"`
	Url       string   `json:"url"`
	Edge      bool     `json:"edge"`
}
