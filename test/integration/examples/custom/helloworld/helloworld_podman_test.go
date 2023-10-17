//go:build (podman && cli) || (podman && examples) || (podman && integration)

package helloworld

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	clientpodman "github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/verifier"
	"github.com/skupperproject/skupper/test/utils/tools"
	"gotest.tools/assert"
)

const (
	PodmanNetwork = "skupper"
)

// TestHelloWorldCLIOnPodman is a test that runs the hello-world-example
// scenario using just the "skupper" binary, which must be available
// in the PATH.
// If the binary is not available, test will be skipped.
// It uses the kubernetes cluster as a public cluster and a podman site
// as the private one.
func TestHelloWorldCLIOnPodman(t *testing.T) {
	stopCh := make(chan interface{}, 1)
	go restartOnLinkDisconnected(stopCh, t)
	defer close(stopCh)

	// First, validate if skupper binary is in the PATH, or skip test
	log.Printf("Running 'skupper --help' to determine if skupper binary is available")
	_, _, err := cli.RunSkupperCli([]string{"--help"})
	if err != nil {
		t.Skipf("skupper binary is not available")
	}

	needs := base.ClusterNeeds{
		NamespaceId:    "hello-world-podman",
		PublicClusters: 1,
	}
	runner := &base.ClusterTestRunnerBase{}
	if err := runner.Validate(needs); err != nil {
		t.Skipf("%s", err)
	}
	_, err = runner.Build(needs, nil)
	assert.Assert(t, err)

	// getting public context
	pub, err := runner.GetPublicContext(1)
	assert.Assert(t, err)

	// Validating if podman site exists
	prvSiteHandler, err := podman.NewSitePodmanHandler("")
	assert.Assert(t, err)
	prvSite, _ := prvSiteHandler.Get()
	assert.Assert(t, prvSite == nil, "podman site already exists")

	// creating namespaces
	assert.Assert(t, pub.CreateNamespace())

	// teardown once test completes
	tearDownFn := func() {
		log.Println("entering teardown")
		_ = pub.DeleteNamespace()
		_ = prvSiteHandler.Delete()
		_ = cleanupPodmanResources()
	}
	defer tearDownFn()
	base.HandleInterruptSignal(func() {
		tearDownFn()
	})

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	// deploying frontend services
	// backend will run as a podman container
	assert.Assert(t, deployResources(pub, nil))
	assert.Assert(t, deployPodmanResources())

	// host port to bind for hello-world-frontend service
	frontendServiceHostPort, err := utils.TcpPortNextFree(8080)
	assert.Assert(t, err, "unable to determing next free TCP port - %w", err)
	// host port to bind for hello-world-backend service
	backendServiceHostPort, err := utils.TcpPortNextFree(8081)
	assert.Assert(t, err, "unable to determing next free TCP port - %w", err)

	// Curl verifiers to run within the cluster
	pubCurlFEVerifier := &verifier.CurlVerifier{
		Url: "http://hello-world-frontend:8080",
		Opts: tools.CurlOpts{
			Silent: true,
		},
	}
	pubCurlBEVerifier := &verifier.CurlVerifier{
		Url: "http://hello-world-backend:8080/api/hello",
		Opts: tools.CurlOpts{
			Silent: true,
		},
	}

	// HTTP verifier to run against podman host
	localhostFEVerifier := &verifier.HttpVerifier{
		Url:          fmt.Sprintf("http://127.0.0.1:%d", frontendServiceHostPort),
		Method:       "GET",
		ExpectedCode: 200,
	}
	localhostBEVerifier := &verifier.HttpVerifier{
		Url:          fmt.Sprintf("http://127.0.0.1:%d/api/hello", backendServiceHostPort),
		Method:       "GET",
		ExpectedCode: 200,
	}
	localhostConsoleVerifier := &verifier.HttpVerifier{
		Url:          fmt.Sprintf("https://127.0.0.1:8010/"),
		Method:       "GET",
		ExpectedCode: 200,
		User:         "internal",
		Password:     "internal",
	}
	localhostCollectorVerifier := &verifier.HttpVerifier{
		Url:          fmt.Sprintf("https://127.0.0.1:8010/api/v1alpha1/routers/"),
		Method:       "GET",
		ExpectedCode: 200,
		User:         "internal",
		Password:     "internal",
	}

	// These test scenarios allow defining a set of skupper cli
	// commands to be executed as a workflow, against specific
	// cluster and local podman site. Each execution is validated
	// accordingly by its SkupperCommandTester implementation.
	//
	// The idea is to cover most of the main skupper commands
	// as we run the hello-world-example so that all manipulation
	// is performed just by the skupper binary, while each
	// SkupperCommandTester implementation validates necessary
	// output or resources in the cluster and podman site to certify
	// the command was executed correctly.
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
						EnableFlowCollector: true,
						RunAsUser:           getRunAsUserOrDefault("1000", pub),
						RunAsGroup:          "2000",
					},
					// skupper status - verify initialized as interior
					&cli.StatusTester{
						RouterMode:          "interior",
						ConsoleEnabled:      true,
						CollectorEnabled:    true,
						ConsoleAuthInternal: true,
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper init - interior mode
					&cli.InitTester{
						Ingress:             "none",
						RouterLogging:       "info",
						RouterMode:          "interior",
						ConsoleAuth:         "internal",
						ConsoleUser:         "internal",
						ConsolePassword:     "internal",
						EnableConsole:       true,
						EnableFlowCollector: true,
						SiteName:            "private",
					},
					// skupper status - verify initialized as interior with the appropriate site name
					&cli.StatusTester{
						RouterMode:          "interior",
						SiteName:            "private",
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
						CollectorEnabled:    true,
					},
				},
					PostVerifiers: []cli.CommandVerifier{
						localhostConsoleVerifier.Request,
						localhostCollectorVerifier.Request,
					},
				},
			},
		}, {
			Name: "connect-sites",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper token create - verify token has been created
					&token.CreateTester{
						Name:     "public",
						FileName: testPath + "public-hello-world-podman-1.token.yaml",
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper link create - connect to public and verify connection created
					&link.CreateTester{
						TokenFile: testPath + "public-hello-world-podman-1.token.yaml",
						Name:      "public",
						Cost:      1,
					},
				}},
			},
		},
		{
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
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper status - verify sites are connected
					&cli.StatusTester{
						RouterMode:     "interior",
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
		},
		{
			Name: "service-create-bind",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the frontend service and verify
					&service.CreateTester{
						Name:    "hello-world-frontend",
						Port:    8080,
						Mapping: "tcp",
					},
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}},
						},
					},
					// skupper service create - creates the backend service and verify
					&service.CreateTester{
						Name:    "hello-world-backend",
						Port:    8080,
						Mapping: "tcp",
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     2,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the backend service and verify
					&service.CreateTester{
						Name:    "hello-world-backend",
						Port:    8080,
						Mapping: "tcp",
						Podman: service.PodmanCreateOptions{
							ContainerName: "hello-world-backend-proxy",
							HostPorts:     []string{fmt.Sprintf("8080:%d", backendServiceHostPort)},
						},
					},
					// create the hello-world-frontend service in the podman site, binding host port 8080
					&service.CreateTester{
						Name:    "hello-world-frontend",
						Port:    8080,
						Mapping: "tcp",
						Podman: service.PodmanCreateOptions{
							HostPorts: []string{fmt.Sprintf("8080:%d", frontendServiceHostPort)},
						},
					},
					// skupper service status - validate status of the two created services without targets
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}},
						},
						Podman: service.StatusPodman{
							ServiceHostPort: map[string]service.HostPortBinding{
								"hello-world-frontend": {HostPorts: map[int]int{8080: frontendServiceHostPort}},
								"hello-world-backend":  {HostPorts: map[int]int{8080: backendServiceHostPort}},
							},
						},
					},
					// skupper status - verify two services are now exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						SiteName:            "private",
						ConnectedSites:      1,
						ExposedServices:     2,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
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
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: "hello-world-frontend", TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-frontend"},
							}},
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}},
						},
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper service bind - bind service to deployment and validate target has been defined
					&service.BindTester{
						ServiceName: "hello-world-backend",
						TargetType:  "host",
						TargetName:  "hello-world-backend",
						TargetPort:  8080,
					},
					// skupper service status - validate backend service now has a target
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: `*domain.EgressResolverHost={"host":"hello-world-backend","ports":{"8080":8080}}`,
									TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-backend"},
							}},
						},
						Podman: service.StatusPodman{
							ServiceHostPort: map[string]service.HostPortBinding{
								"hello-world-frontend": {HostPorts: map[int]int{8080: frontendServiceHostPort}},
								"hello-world-backend":  {HostPorts: map[int]int{8080: backendServiceHostPort}},
							},
						},
					},
				}},
			},
		},
		{
			Name: "verify-services-after-bind",
			Tasks: []cli.SkupperTask{
				{
					Ctx: pub,
					PostVerifiers: []cli.CommandVerifier{
						pubCurlFEVerifier.GetRequest,
						pubCurlBEVerifier.GetRequest,
					},
				},
				{
					Platform: types.PlatformPodman,
					PostVerifiers: []cli.CommandVerifier{
						localhostBEVerifier.Request,
						localhostFEVerifier.Request,
					},
				},
			},
		},
		{
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
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}},
						},
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper service unbind - unbind and verify service no longer has a target
					&service.UnbindTester{
						ServiceName: "hello-world-backend",
						TargetType:  "host",
						TargetName:  "hello-world-backend",
					},
					// skupper service status - validates no more target for backend service
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "tcp", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}},
						},
						Podman: service.StatusPodman{
							ServiceHostPort: map[string]service.HostPortBinding{
								"hello-world-frontend": {HostPorts: map[int]int{8080: frontendServiceHostPort}},
								"hello-world-backend":  {HostPorts: map[int]int{8080: backendServiceHostPort}},
							},
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
							{Address: "hello-world-backend", Protocol: "tcp", Ports: []int{8080}},
						},
					},
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-backend",
					},
					// skupper service status - verify only backend is available
					&service.StatusTester{},
					// skupper status - verify there is no exposed service
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     0,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-backend",
					},
					// skupper status - verify there is no exposed service
					&cli.StatusTester{
						RouterMode:          "interior",
						SiteName:            "private",
						ConnectedSites:      1,
						ExposedServices:     1,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-frontend",
					},
					// skupper status - verify there is no exposed service
					&cli.StatusTester{
						RouterMode:          "interior",
						SiteName:            "private",
						ConnectedSites:      1,
						ExposedServices:     0,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
			},
		},
		{Name: "expose",
			Tasks: []cli.SkupperTask{
				{Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper expose - expose and ensure service is available
					&cli.ExposeTester{
						TargetType: "deployment",
						TargetName: "hello-world-frontend",
						Address:    "hello-world-frontend",
						Port:       8080,
						Protocol:   "tcp",
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
					// skupper service create - creates the backend service and verify
					&service.CreateTester{
						Name:    "hello-world-backend",
						Port:    8080,
						Mapping: "tcp",
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     2,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper expose - exposes backend and certify it is available
					&cli.ExposeTester{
						TargetType: "host",
						TargetName: "hello-world-backend",
						Address:    "hello-world-backend",
						Port:       8080,
						Protocol:   "tcp",
						TargetPort: 8080,
						Podman: cli.PodmanExposeOptions{
							ContainerName: "hello-world-backend-proxy",
							Labels: map[string]string{
								"app": "hello-world-backend",
							},
							HostPorts: []string{fmt.Sprintf("8080:%d", backendServiceHostPort)},
						},
					},
					// create the hello-world-frontend service in the podman site, binding host port 8080
					&service.CreateTester{
						Name:    "hello-world-frontend",
						Port:    8080,
						Mapping: "tcp",
						Podman: service.PodmanCreateOptions{
							HostPorts: []string{fmt.Sprintf("8080:%d", frontendServiceHostPort)},
						},
					},
					// skupper status - asserts that there are 2 exposed services
					&cli.StatusTester{
						RouterMode:          "interior",
						SiteName:            "private",
						ConnectedSites:      1,
						ExposedServices:     2,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
					},
				}},
			},
		},
		{
			Name: "verify-services-after-expose",
			Tasks: []cli.SkupperTask{
				{
					Ctx: pub,
					PostVerifiers: []cli.CommandVerifier{
						pubCurlFEVerifier.GetRequest,
						pubCurlBEVerifier.GetRequest,
					},
				},
				{
					Platform: types.PlatformPodman,
					PostVerifiers: []cli.CommandVerifier{
						localhostBEVerifier.Request,
						localhostFEVerifier.Request,
					},
				},
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
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-backend",
					},
					// skupper status - verify only 1 service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     0,
						ConsoleEnabled:      false,
						CollectorEnabled:    true,
						ConsoleAuthInternal: true,
					},
				}},
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
					// skupper unexpose - unexpose and verify it has been removed
					&cli.UnexposeTester{
						TargetType: "host",
						TargetName: "hello-world-backend",
						Address:    "hello-world-backend",
					},
					// skupper status - verify there is no exposed services
					&cli.StatusTester{
						RouterMode:      "interior",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: 1,
					},
					// skupper service delete - removes exposed service and certify it is removed
					&service.DeleteTester{
						Name: "hello-world-frontend",
					},
					// skupper status - verify there is no exposed services
					&cli.StatusTester{
						RouterMode:      "interior",
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
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
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
				{Platform: types.PlatformPodman, Commands: []cli.SkupperCommandTester{
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

// restartOnLinkDisconnected On CircleCI external network connectivity
// is being lost this routine will restart the skupper-podman user service
// to bypass this issue.
// TODO: this can be removed once CI is fixed
func restartOnLinkDisconnected(stopCh chan interface{}, t *testing.T) {
	if os.Getenv("USER") != "circleci" {
		return
	}
	// Preparing the command to run
	tick := time.NewTicker(time.Second * 5)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
			stdout := new(bytes.Buffer)
			cmd := exec.Command("skupper", "--platform=podman", "link", "status")
			cmd.Stdout = stdout
			_ = cmd.Run()
			if strings.Contains(stdout.String(), " not connected") {
				t.Logf("[Link Disconnected] - Restarting skupper-podman (user) service")
				restartCmd := exec.Command("systemctl", "--user", "restart", "skupper-podman")
				_ = restartCmd.Run()
			}
		case <-stopCh:
			return
		}
	}
}

// deployPodmanResources runs the hello-world-backend as a podman container
func deployPodmanResources() error {
	podmanCli, err := clientpodman.NewPodmanClient("", "")
	if err != nil {
		return fmt.Errorf("error creating podman client - %w", err)
	}

	// create the podman network to be used
	var net *container.Network
	net, err = podmanCli.NetworkInspect(PodmanNetwork)
	if err != nil {
		// network does not yet exist, creating
		_, err = podmanCli.NetworkCreate(&container.Network{
			Name: PodmanNetwork,
			DNS:  true,
		})
		if err != nil {
			return err
		}
	} else if net.DNS == false {
		// podman network exists, but DNS is not enabled
		return fmt.Errorf("podman network %s already exists, but DNS is not enabled", PodmanNetwork)
	}
	err = podmanCli.ImagePull("quay.io/skupper/hello-world-backend")
	if err != nil {
		return err
	}
	be := &container.Container{
		Name:   "hello-world-backend",
		Image:  "quay.io/skupper/hello-world-backend",
		Labels: map[string]string{"app": "hello-world-backend"},
		Networks: map[string]container.ContainerNetworkInfo{
			PodmanNetwork: {ID: PodmanNetwork},
		},
		RestartPolicy: "always",
	}
	if err = podmanCli.ContainerCreate(be); err != nil {
		return err
	}

	return nil
}

func cleanupPodmanResources() error {
	podmanCli, err := clientpodman.NewPodmanClient("", "")
	if err != nil {
		return fmt.Errorf("error creating podman client - %w", err)
	}

	if err = podmanCli.ContainerRemove("hello-world-backend"); err != nil {
		return err
	}

	return nil
}
