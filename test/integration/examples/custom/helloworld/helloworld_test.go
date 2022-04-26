//go:build integration || cli || examples
// +build integration cli examples

package helloworld

import (
	"log"
	"os"
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
	v1 "k8s.io/api/core/v1"
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
						EnableConsole:       true,
						EnableRouterConsole: true,
					},
					// skupper status - verify initialized as interior
					&cli.StatusTester{
						RouterMode:          "interior",
						ConsoleEnabled:      true,
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
						RouterDebugMode:       "gdb",
						RouterLogging:         "trace",
						RouterMode:            "edge",
						SiteName:              "private",
						EnableConsole:         false,
						EnableRouterConsole:   false,
						RouterCPU:             "100m",
						RouterMemory:          "32Mi",
						ControllerCPU:         "50m",
						ControllerMemory:      "16Mi",
						RouterCPULimit:        "600m",
						RouterMemoryLimit:     "500Mi",
						ControllerCPULimit:    "600m",
						ControllerMemoryLimit: "500Mi",
						//ConsoleIngress:      "none",
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
						Protocol:    "http",
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
						Protocol:    "http",
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
						ConsoleEnabled:      true,
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
						ConsoleEnabled:      true,
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
	frontend, _ := k8s.NewDeployment("hello-world-frontend", pub.Namespace, k8s.DeploymentOpts{
		Image:         "quay.io/skupper/hello-world-frontend",
		Labels:        map[string]string{"app": "hello-world-frontend"},
		RestartPolicy: v1.RestartPolicyAlways,
	})
	backend, _ := k8s.NewDeployment("hello-world-backend", prv.Namespace, k8s.DeploymentOpts{
		Image:         "quay.io/skupper/hello-world-backend",
		Labels:        map[string]string{"app": "hello-world-backend"},
		RestartPolicy: v1.RestartPolicyAlways,
	})

	// Creating deployments
	if _, err := pub.VanClient.KubeClient.AppsV1().Deployments(pub.Namespace).Create(frontend); err != nil {
		return err
	}
	if _, err := prv.VanClient.KubeClient.AppsV1().Deployments(prv.Namespace).Create(backend); err != nil {
		return err
	}

	// Waiting for deployments to be ready
	if _, err := kube.WaitDeploymentReady("hello-world-frontend", pub.Namespace, pub.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return err
	}
	if _, err := kube.WaitDeploymentReady("hello-world-backend", prv.Namespace, prv.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return err
	}

	return nil
}
