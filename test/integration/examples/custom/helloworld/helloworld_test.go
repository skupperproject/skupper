//go:build integration || cli || examples
// +build integration cli examples

package helloworld

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
	"gotest.tools/assert"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestHelloWorldCLI is a test that runs the hello-world-example
// scenario using just the "skupper" binary, which must be available
// in the PATH.
// If the binary is not available, test will be skipped.
func TestHelloWorldCLI(t *testing.T) {

	// First, validate if skupper binary is in the PATH, or skip test
	log.Printf("Running 'skupper --help' to determine if skupper binary is available")
	_, _, err := cli.RunSkupperCli([]string{"--help"})
	if err != nil {
		t.Skipf("skupper binary is not available")
	}

	needs := base.ClusterNeeds{
		NamespaceId:     "hello-world",
		PublicClusters:  1,
		PrivateClusters: 1,
	}
	runner := &base.ClusterTestRunnerBase{}
	if err := runner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err = runner.Build(needs, nil)
	assert.Assert(t, err)

	// getting public and private contexts
	pub, err := runner.GetPublicContext(1)
	assert.Assert(t, err)
	prv, err := runner.GetPrivateContext(1)
	assert.Assert(t, err)

	// creating namespaces
	assert.Assert(t, pub.CreateNamespace())
	assert.Assert(t, prv.CreateNamespace())

	// teardown once test completes
	tearDownFn := func() {
		if t.Failed() {
			runner.DumpTestInfo(needs.NamespaceId)
		}
		log.Println("entering teardown")
		_ = pub.DeleteNamespace()
		_ = prv.DeleteNamespace()
	}
	defer tearDownFn()
	base.HandleInterruptSignal(func() {
		tearDownFn()
	})

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	// deploying frontend and backend services
	assert.Assert(t, deployResources(pub, prv))

	var runAsUser = "1000"

	// OpenShift requires container user IDs to exist within a range; we try to satisfy it here.
	namespace, err := pub.VanClient.KubeClient.CoreV1().Namespaces().Get(context.Background(), pub.Namespace, metav1.GetOptions{})
	if err != nil {
		log.Printf("Unable to get namespace %q; using pre-defined runAsUser value %v", pub.Namespace, runAsUser)
		// We do not fail here; we just try the test with the pre-defined value
	} else {
		// On this block, we just ignore any errors and use the pre-existing value
		ns_annotations := namespace.GetAnnotations()
		if users, ok := ns_annotations["openshift.io/sa.scc.uid-range"]; ok {
			log.Printf("OpenShift UID range annotation found: %q", users)
			// format is like 1000860000/10000, where the first number is the
			// range start, and the second its length
			split_users := strings.Split(users, "/")
			if split_users[0] != "" {
				if _, err := strconv.Atoi(split_users[0]); err == nil {
					runAsUser = split_users[0]
				}
			}
		}
	}

	// These test scenarios allow defining a set of skupper cli
	// commands to be executed as a workflow, against specific
	// clusters. Each execution is validated accordingly by its
	// SkupperCommandTester implementation.
	//
	// The idea is to cover most of the main skupper commands
	// as we run the hello-world-example so that all manipulation
	// is performed just by the skupper binnary, while each
	// SkupperCommandTester implementation validates necessary
	// output or resources in the cluster to certify the command
	// was executed correctly.
	scenarios := []cli.TestScenario{
		{
			Name: "initialize",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper init - interior mode, enabling console and internal authentication
					&cli.InitTester{
						ConsoleAuth:         "internal",
						ConsoleUser:         "internal",
						ConsolePassword:     "internal",
						RouterMode:          "interior",
						EnableConsole:       false,
						EnableFlowCollector: true,
						RunAsUser:           runAsUser,
						RunAsGroup:          "2000",
					},
					// skupper status - verify initialized as interior
					&cli.StatusTester{
						RouterMode:          "interior",
						ConsoleEnabled:      false,
						CollectorEnabled:    true,
						ConsoleAuthInternal: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper init - edge mode, no console and unsecured
					&cli.InitTester{
						ConsoleAuth:           "unsecured",
						ConsoleUser:           "admin",
						ConsolePassword:       "admin",
						Ingress:               "none",
						RouterLogging:         "trace",
						RouterMode:            "edge",
						SiteName:              "private",
						EnableConsole:         false,
						EnableFlowCollector:   false,
						RouterCPU:             "100m",
						RouterMemory:          "32Mi",
						ControllerCPU:         "50m",
						ControllerMemory:      "16Mi",
						RouterCPULimit:        "600m",
						RouterMemoryLimit:     "500Mi",
						ControllerCPULimit:    "600m",
						ControllerMemoryLimit: "500Mi",
						// ConsoleIngress:      "none",
					},
					// skupper status - verify initialized as edge
					&cli.StatusTester{
						RouterMode: "edge",
						SiteName:   "private",
					},
				}},
			},
		}, {
			Name: "connect-sites",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						Name:     "public",
						FileName: testPath + "public-hello-world-1.token.yaml",
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: testPath + "public-hello-world-1.token.yaml",
						Name:      "public",
						Cost:      1,
					},
				}},
			},
		}, {
			Name: "validate-sites-connected",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper status - verify sites are connected
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper status - verify sites are connected
					&cli.StatusTester{
						RouterMode:     "edge",
						SiteName:       "private",
						ConnectedSites: 1,
					},
					// skupper link status - testing all links
					&link.StatusTester{
						Name:   "public",
						Active: true,
					},
					// skupper link status - now using link name and a 10 secs wait
					&link.StatusTester{
						Name:   "public",
						Active: true,
						Wait:   10,
					},
				}},
			},
		}, {
			Name: "service-create-bind",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the frontend service and verify
					&service.CreateTester{
						Name:    "hello-world-frontend",
						Port:    8080,
						Mapping: "http",
					},
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
						},
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     1,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the backend service and verify
					&service.CreateTester{
						Name:    "hello-world-backend",
						Port:    8080,
						Mapping: "http",
					},
					// skupper service status - validate status of the two created services without targets
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
					},
					// skupper status - verify two services are now exposed
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: 2,
					},
				}},
				// Binding the services
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service bind - bind service to deployment and validate target has been defined
					&service.BindTester{
						ServiceName: "hello-world-frontend",
						TargetType:  "deployment",
						TargetName:  "hello-world-frontend",
						TargetPort:  8080,
					},
					// skupper service status - validate status expecting frontend now has a target
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: "hello-world-frontend", TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-frontend"},
							}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service bind - bind service to deployment and validate target has been defined
					&service.BindTester{
						ServiceName: "hello-world-backend",
						TargetType:  "deployment",
						TargetName:  "hello-world-backend",
						TargetPort:  8080,
					},
					// skupper service status - validate backend service now has a target
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: "hello-world-backend", TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-backend"},
							}},
						},
					},
				}},
			},
		}, {
			Name: "service-unbind-delete",
			Tasks: []cli.SkupperTask{
				// unbinding frontend and validating service status for public cluster
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service unbind - unbind and verify service no longer has a target
					&service.UnbindTester{
						ServiceName: "hello-world-frontend",
						TargetType:  "deployment",
						TargetName:  "hello-world-frontend",
					},
					// skupper service status - validates no more target for frontend service
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service unbind - unbind and verify service no longer has a target
					&service.UnbindTester{
						ServiceName: "hello-world-backend",
						TargetType:  "deployment",
						TargetName:  "hello-world-backend",
					},
					// skupper service status - validates no more target for frontend service
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
					},
				}},
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-frontend",
					},
					// skupper service status - verify only backend is available
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-backend",
					},
					// skupper status - verify there is no exposed service
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: 0,
					},
				}},
			},
		}, {Name: "expose",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper expose - expose and ensure service is available
					&cli.ExposeTester{
						TargetType: "deployment",
						TargetName: "hello-world-frontend",
						Address:    "hello-world-frontend",
						Port:       8080,
						Protocol:   "http",
						TargetPort: 8080,
					},
					// skupper status - asserts that 1 service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     1,
						ConsoleEnabled:      false,
						CollectorEnabled:    true,
						ConsoleAuthInternal: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper expose - exposes backend and certify it is available
					&cli.ExposeTester{
						TargetType: "deployment",
						TargetName: "hello-world-backend",
						Address:    "hello-world-backend",
						Port:       8080,
						Protocol:   "http",
						TargetPort: 8080,
					},
					// skupper status - asserts that there are 2 exposed services
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: 2,
					},
				}},
			},
		}, {
			Name: "unexpose",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper unexpose - unexpose and verify it has been removed
					&cli.UnexposeTester{
						TargetType: "deployment",
						TargetName: "hello-world-frontend",
						Address:    "hello-world-frontend",
					},
					// skupper status - verify only 1 service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     1,
						ConsoleEnabled:      false,
						CollectorEnabled:    true,
						ConsoleAuthInternal: true,
					},
				}},
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper unexpose - unexpose and verify it has been removed
					&cli.UnexposeTester{
						TargetType: "deployment",
						TargetName: "hello-world-backend",
						Address:    "hello-world-backend",
					},
					// skupper status - verify there is no exposed services
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: 0,
					},
				}},
			},
		}, {
			Name: "version",
			Tasks: []cli.SkupperTask{
				// skupper version - verify version is being reported accordingly
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					&cli.VersionTester{},
				}},
				// skupper version - verify version is being reported accordingly
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					&cli.VersionTester{},
				}},
			},
		}, {
			Name: "delete",
			Tasks: []cli.SkupperTask{
				// skupper delete - delete and verify resources have been removed
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					&cli.DeleteTester{},
					&cli.StatusTester{
						NotEnabled: true,
					},
				}},
				// skupper delete - delete and verify resources have been removed
				{Ctx: prv, Commands: []cli.SkupperCommandTester{
					&cli.DeleteTester{},
					&cli.StatusTester{
						NotEnabled: true,
					},
				}},
			},
		},
	}

	// Running the scenarios
	cli.RunScenarios(t, scenarios)

}

// deployResources Deploys the hello-world-frontend and hello-world-backend
// pods and validate they are available
func deployResources(pub *base.ClusterContext, prv *base.ClusterContext) error {
	var backend *appsv1.Deployment
	frontend, _ := k8s.NewDeployment("hello-world-frontend", pub.Namespace, k8s.DeploymentOpts{
		Image:         "quay.io/skupper/hello-world-frontend",
		Labels:        map[string]string{"app": "hello-world-frontend"},
		RestartPolicy: v1.RestartPolicyAlways,
	})
	if prv != nil {
		backend, _ = k8s.NewDeployment("hello-world-backend", prv.Namespace, k8s.DeploymentOpts{
			Image:         "quay.io/skupper/hello-world-backend",
			Labels:        map[string]string{"app": "hello-world-backend"},
			RestartPolicy: v1.RestartPolicyAlways,
		})
	}

	// Creating deployments
	if _, err := pub.VanClient.KubeClient.AppsV1().Deployments(pub.Namespace).Create(context.TODO(), frontend, metav1.CreateOptions{}); err != nil {
		return err
	}
	if prv != nil {
		if _, err := prv.VanClient.KubeClient.AppsV1().Deployments(prv.Namespace).Create(context.TODO(), backend, metav1.CreateOptions{}); err != nil {
			return err
		}
	}

	// Waiting for deployments to be ready
	if _, err := kube.WaitDeploymentReady("hello-world-frontend", pub.Namespace, pub.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return err
	}
	if prv != nil {
		if _, err := kube.WaitDeploymentReady("hello-world-backend", prv.Namespace, prv.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
			return err
		}
	}

	return nil
}
