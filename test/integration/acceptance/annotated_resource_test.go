//go:build integration || acceptance || annotation
// +build integration acceptance annotation

package acceptance

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"testing"

	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/data"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/integration/acceptance/annotation"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
)

// TestAnnotatedResources deploy resources with Skupper annotations before
// Skupper is available in the respective namespaces. Then we run a test
// table that starts verifying initial state and then applies modifications
// to validate if Skupper is reacting as expected.
func TestAnnotatedResources(t *testing.T) {
	pub, _ := testRunner.GetPublicContext(1)
	prv, _ := testRunner.GetPrivateContext(1)

	// test allows defining the matrix to run the test table
	type test struct {
		name                  string
		doc                   string
		modification          func(*testing.T, base.ClusterTestRunner)
		expectedSites         int
		expectedServicesProto map[string]string
	}

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
				"nginx-1-ss-web":           "tcp",
				"nginx-2-ss-web":           "tcp",
				"nginx-1-ds-web":           "tcp",
				"nginx-2-ds-web":           "tcp",
			},
		},
		{
			name:          "services-protocol-switch",
			doc:           "Switches the protocol for all services and deployment and validate updated service list",
			modification:  annotation.SwitchProtocols,
			expectedSites: 2,
			expectedServicesProto: map[string]string{
				"nginx-1-dep-web":          "http",
				"nginx-2-dep-web":          "http",
				"nginx-1-svc-exp-notarget": "http",
				"nginx-2-svc-exp-notarget": "http",
				"nginx-1-svc-exp-target":   "tcp",
				"nginx-2-svc-exp-target":   "tcp",
				"nginx-1-ss-web":           "http",
				"nginx-2-ss-web":           "http",
				"nginx-1-ds-web":           "http",
				"nginx-2-ds-web":           "http",
			},
		},
		{
			name:                  "services-annotation-removed",
			doc:                   "Remove Skupper annotations from services and deployment and validate available service list",
			modification:          annotation.RemoveAnnotation,
			expectedSites:         2,
			expectedServicesProto: map[string]string{},
		},
		{
			name:          "services-annotated",
			doc:           "Services are annotated after Skupper is deployed and exposed service list matches",
			modification:  annotation.AddAnnotation,
			expectedSites: 2,
			expectedServicesProto: map[string]string{
				"nginx-1-dep-web":          "tcp",
				"nginx-2-dep-web":          "tcp",
				"nginx-1-svc-exp-notarget": "tcp",
				"nginx-2-svc-exp-notarget": "tcp",
				"nginx-1-svc-exp-target":   "http",
				"nginx-2-svc-exp-target":   "http",
				"nginx-1-ss-web":           "tcp",
				"nginx-2-ss-web":           "tcp",
				"nginx-1-ds-web":           "tcp",
				"nginx-2-ds-web":           "tcp",
			},
		},
		{
			name:                  "services-annotation-cleanup",
			doc:                   "Cleanup Skupper annotations from services and deployment and validate available service list",
			modification:          annotation.RemoveAnnotation,
			expectedSites:         2,
			expectedServicesProto: map[string]string{},
		},
	}

	// Iterate through test table and run each test
	for _, test := range testTable {
		testResult := t.Run(test.name, func(t *testing.T) {
			var err error

			// 4.1. If test expects modifications to be performed, run them
			if test.modification != nil {
				test.modification(t, testRunner)
			}
			log.Printf("modification has been applied")

			// 4.2 Validate services from DATA endpoint (with retries)
			var consoleData data.ConsoleData
			backoff := constants.DefaultRetry
			ctx, cancelFn := context.WithTimeout(context.Background(), constants.ImagePullingAndResourceCreationTimeout)
			defer cancelFn()

			err = utils.RetryWithContext(ctx, backoff.Duration, func() (bool, error) {
				log.Printf("Trying to retrieve ConsoleData...")
				// 4.2.1. retrieve ConsoleData and eventual error
				consoleData, err = base.GetConsoleData(pub, "admin", "admin")
				if err != nil {
					// error is not expected (but needs to retry as per issue #295)
					log.Printf("error retrieving console data: %v", err)
					return false, nil
					// return true, err // after #295 is fixed, we must return these values.
				}

				// 4.2.2. retry till expected amount of sites and services match
				if test.expectedSites != len(consoleData.Sites) || len(test.expectedServicesProto) != len(consoleData.Services) {
					log.Printf("ConsoleData do not match yet")
					log.Printf("Number of sites  [expected: %d - found: %d]", test.expectedSites, len(consoleData.Sites))
					log.Printf("Exposed services [expected: %d - found: %d]", len(test.expectedServicesProto), len(consoleData.Services))
					return false, nil
				}

				// 4.2.3. populating map of services/protocols returned
				servicesFound := map[string]string{}
				for _, svc := range consoleData.Services {

					var protocol string
					var address string

					switch svcType := svc.(type) {
					// HTTP Service
					case data.HttpService:
						protocol = svc.(data.HttpService).Protocol
						address = svc.(data.HttpService).Address
					// TCP Service
					case data.TcpService:
						protocol = svc.(data.TcpService).Protocol
						address = svc.(data.TcpService).Address
					// Unknown Service
					default:
						log.Printf("Unable to identify Service type for : %s", svcType)
					}

					servicesFound[address] = protocol
				}

				// 4.2.4. only consider as done if all match (otherwise it will timeout)
				res := reflect.DeepEqual(test.expectedServicesProto, servicesFound)
				if !res {
					log.Printf("services and protocols do not match yet expected: %s - got: %s", test.expectedServicesProto, servicesFound)
					annotation.DebugAnnotatedResources(t, testRunner)
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
					// reaching service through service-controller's pod (with some attempts to make sure bridge is connected)
					var lastErr error
					err = utils.Retry(backoff.Duration, backoff.Steps, func() (bool, error) {
						endpoint := fmt.Sprintf("http://%s:8080", svc)
						resp, lastErr = tools.Curl(cluster.KubeClient, cluster.RestConfig, cluster.Namespace, "", endpoint, tools.CurlOpts{Timeout: 10})
						if lastErr != nil {
							return false, nil
						}
						return resp.StatusCode == 200, nil
					})
					assert.Assert(t, err, "unable to reach service %s through %s: %s", svc, cluster.Namespace, lastErr)
					assert.Equal(t, resp.StatusCode, 200, "bad response received from service %s through %s", svc, cluster.Namespace)
					assert.Assert(t, resp.Body != "", "empty response body received from service %s through %s", svc, cluster.Namespace)
					log.Printf("successfully communicated with service %s through %s", svc, cluster.Namespace)
				}
			}
		})
		if !testResult {
			log.Printf("Test %s failed: gathering info dump", test.name)
			testRunner.DumpTestInfo(filepath.Join(t.Name(), test.name))
		}

	}

	// Undeploying resources
	assert.Assert(t, annotation.UndeployResources(testRunner))

}
