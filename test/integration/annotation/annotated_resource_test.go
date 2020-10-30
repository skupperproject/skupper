// +build integration annotated

package annotation

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"testing"

	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
)

var (
	needs = base.ClusterNeeds{
		NamespaceId:     "annotated",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
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
	testRunner := &base.ClusterTestRunnerBase{}
	testRunner.BuildOrSkip(t, needs, nil)
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)
	defer TearDown(t, testRunner)
	base.HandleInterruptSignal(t, func(t *testing.T) {
		cancelFn()
		TearDown(t, testRunner)
	})

	// 1. Initialize Namespaces
	Setup(t, testRunner)

	// 2. Deploying the Pre-annotated resources
	DeployResources(t, testRunner)

	// 3. Deploys Skupper and create a VAN between both clusters
	CreateVan(t, testRunner)

	// 4. Iterate through test table and run each test
	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			var err error

			// 4.1. If test expects modifications to be performed, run them
			if test.modification != nil {
				test.modification(t, testRunner)
			}
			log.Printf("modification has been applied")

			// 4.2 Validate services from DATA endpoint (with retries)
			var consoleData base.ConsoleData
			backoff := constants.DefaultRetry
			ctx, cancelFn = context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)

			err = utils.RetryWithContext(ctx, backoff.Duration, func() (bool, error) {
				log.Printf("Trying to retrieve ConsoleData...")
				// 4.2.1. retrieve ConsoleData and eventual error
				consoleData, err = base.GetConsoleData(pub, "admin", "admin")
				if err != nil {
					// error is not expected (but needs to retry as per issue #295)
					log.Printf("error retrieving console data: %v", err)
					return false, nil
					//return true, err // after #295 is fixed, we must return these values.
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
				if !res {
					log.Printf("services and protocols do not match yet expected: %s - got: %s", test.expectedServicesProto, servicesFound)
				}
				return res, nil
			})
			assert.Assert(t, err, "timed out waiting for expected services/protocol list to match")
			log.Printf("console data has been validated")

			// 4.3. Validating all exposed services are reachable across clusters
			for _, cluster := range []*vanClient.VanClient{pub.VanClient, prv.VanClient} {
				for svc := range test.expectedServicesProto {
					var resp *tools.CurlResponse
					log.Printf("validating communication with service %s through %s", svc, cluster.Namespace)
					// reaching service through proxy-controller's pod (with some attempts to make sure bridge is connected)
					err = utils.Retry(backoff.Duration, backoff.Steps, func() (bool, error) {
						endpoint := fmt.Sprintf("http://%s:8080", svc)
						resp, err = tools.Curl(cluster.KubeClient, cluster.RestConfig, cluster.Namespace, "", endpoint, tools.CurlOpts{Timeout: 10})
						if err != nil {
							return false, nil
						}
						return resp.StatusCode == 200, nil
					})
					assert.Assert(t, err, "unable to reach service %s through %s", svc, cluster.Namespace)
					assert.Equal(t, resp.StatusCode, 200, "bad response received from service %s through %s", svc, cluster.Namespace)
					assert.Assert(t, resp.Body != "", "empty response body received from service %s through %s", svc, cluster.Namespace)
					log.Printf("successfully communicated with service %s through %s", svc, cluster.Namespace)
				}
			}
		})
	}
}
