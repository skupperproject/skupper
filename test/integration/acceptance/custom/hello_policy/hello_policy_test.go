//go:build policy
// +build policy

// TODO:
// - link and policy
//   - policy destroys link on source - rebuild
//   - policy destroys link on dest - ??
// - "Not authorized service" on skupper service status
// - t.Run names (standard, Polarion)
// - Enhance to include annotation-based exposing
// - Console

package hello_policy

import (
	"fmt"
	"os"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/link"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/token"
)

// TestHelloPolicy is a test that runs the hello-world-example
// scenario using just the "skupper" binary, which must be available
// in the PATH.
// It is a copy of the test at test/integration/examples/custom/helloworld/,
// adapted for Policy testing
func testHelloPolicy(t *testing.T, pub1, pub2 *base.ClusterContext) {

	fmt.Println("pub1", pub1)
	fmt.Println("pub2", pub2)

	// Creating a local directory for storing the token
	testPath := "./tmp/"
	_ = os.Mkdir(testPath, 0755)

	// These test scenarios allow defining a set of skupper cli
	// commands to be executed as a workflow, against specific
	// clusters. Each execution is validated accordingly by its
	// SkupperCommandTester implementation.
	//
	// The idea is to cover most of the main skupper commands
	// as we run the hello-world-example so that all manipulation
	// is performed just by the skupper binary, while each
	// SkupperCommandTester implementation validates necessary
	// output or resources in the cluster to certify the command
	// was executed correctly.
	initSteps := []cli.TestScenario{
		skupperInitInteriorTestScenario(pub1, "", false),
		skupperInitEdgeTestScenario(pub2, "", false),
	}

	connectSteps := cli.TestScenario{
		Name: "connect-sites",
		Tasks: []cli.SkupperTask{
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
				// skupper token create - verify token has been created
				&token.CreateTester{
					Name:     "public",
					FileName: testPath + "public-hello-world-1.token.yaml",
				},
			}},
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
				// skupper link create - connect to public and verify connection created
				&link.CreateTester{
					TokenFile: testPath + "public-hello-world-1.token.yaml",
					Name:      "public",
					Cost:      1,
				},
			}},
		},
	}

	//validateConnSteps Steps to confirm a link exists from the private namespace to the public one
	validateConnSteps := cli.TestScenario{
		Name: "validate-sites-connected",
		Tasks: []cli.SkupperTask{
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
				// skupper status - verify sites are connected
				&cli.StatusTester{
					RouterMode:          "interior",
					ConnectedSites:      1,
					ConsoleEnabled:      true,
					ConsoleAuthInternal: true,
				},
			}},
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
	}

	serviceCreateBindSteps := cli.TestScenario{
		Name: "service-create-bind",
		Tasks: []cli.SkupperTask{
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
	}

	serviceUnbindDeleteSteps := cli.TestScenario{
		Name: "service-unbind-delete",
		Tasks: []cli.SkupperTask{
			// unbinding frontend and validating service status for public cluster
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
	}

	exposeSteps := cli.TestScenario{
		Name: "expose",
		Tasks: []cli.SkupperTask{
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
	}

	unexposeSteps := cli.TestScenario{
		Name: "unexpose",
		Tasks: []cli.SkupperTask{
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
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
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
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
	}

	versionSteps := cli.TestScenario{
		Name: "version",
		Tasks: []cli.SkupperTask{
			// skupper version - verify version is being reported accordingly
			{Ctx: pub1, Commands: []cli.SkupperCommandTester{
				&cli.VersionTester{},
			}},
			// skupper version - verify version is being reported accordingly
			{Ctx: pub2, Commands: []cli.SkupperCommandTester{
				&cli.VersionTester{},
			}},
		},
	}

	mainSteps := []cli.TestScenario{
		connectSteps,
		validateConnSteps,
		serviceCreateBindSteps,
		serviceUnbindDeleteSteps,
		exposeSteps,
		unexposeSteps,
		versionSteps,
	}

	checkStuffCameBackUp := []cli.TestScenario{
		validateConnSteps,
	}

	checkStuffCameDown := []cli.TestScenario{
		{
			Name: "validate-sites-disconnected",
			Tasks: []cli.SkupperTask{
				{Ctx: pub1, Commands: []cli.SkupperCommandTester{
					// skupper status - verify sites are connected
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      0,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
						PolicyEnabled:       true,
					},
				}},
				{Ctx: pub2, Commands: []cli.SkupperCommandTester{
					// skupper status - verify sites are connected
					&cli.StatusTester{
						RouterMode:     "edge",
						SiteName:       "private",
						ConnectedSites: 0,
						PolicyEnabled:  true,
					},
					// skupper link status - testing all links
					&link.StatusTester{
						Name:   "public",
						Active: false,
					},
					// skupper link status - now using link name and a 10 secs wait
					&link.StatusTester{
						Name:   "public",
						Active: false,
						Wait:   10,
					},
				}},
			},
		}, {
			Name: "services-destroyed",
			Tasks: []cli.SkupperTask{
				{Ctx: pub1, Commands: []cli.SkupperCommandTester{
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
						},
						Absent: true,
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      0,
						ExposedServices:     0,
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
						PolicyEnabled:       true,
					},
				}},
				{Ctx: pub2, Commands: []cli.SkupperCommandTester{
					// skupper service status - validate status of the two created services without targets
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
						Absent: true,
					},
					// skupper status - verify two services are now exposed
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  0,
						ExposedServices: 0,
						PolicyEnabled:   true,
					},
				}},
				// Binding the services
				{Ctx: pub1, Commands: []cli.SkupperCommandTester{
					// skupper service bind - bind service to deployment and validate target has been defined
					&service.BindTester{
						ServiceName:     "hello-world-frontend",
						TargetType:      "deployment",
						TargetName:      "hello-world-frontend",
						Protocol:        "http",
						TargetPort:      8080,
						ExpectAuthError: true,
					},
					// skupper service status - validate status expecting frontend now has a target
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: "hello-world-frontend", TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-frontend"},
							}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}},
						},
						Absent: true,
					},
				}},
				{Ctx: pub2, Commands: []cli.SkupperCommandTester{
					// skupper service bind - bind service to deployment and validate target has been defined
					&service.BindTester{
						ServiceName:     "hello-world-backend",
						TargetType:      "deployment",
						TargetName:      "hello-world-backend",
						Protocol:        "http",
						TargetPort:      8080,
						ExpectAuthError: true,
					},
					// skupper service status - validate backend service now has a target
					&service.StatusTester{
						ServiceInterfaces: []types.ServiceInterface{
							{Address: "hello-world-frontend", Protocol: "http", Ports: []int{8080}},
							{Address: "hello-world-backend", Protocol: "http", Ports: []int{8080}, Targets: []types.ServiceInterfaceTarget{
								{Name: "hello-world-backend", TargetPorts: map[int]int{8080: 8080}, Service: "hello-world-backend"},
							}},
						},
						Absent: true,
					},
				}},
			},
		},
	}

	deleteSteps := append([]cli.TestScenario{}, deleteSkupperTestScenario(pub1, ""), deleteSkupperTestScenario(pub2, ""))

	//	scenarios := append(append(initSteps, mainSteps...), deleteSteps...)

	// Running the scenarios
	t.Run("init", func(t *testing.T) { cli.RunScenarios(t, initSteps) })
	//	mainSteps = mainSteps
	t.Run("No CRD, all works", func(t *testing.T) { cli.RunScenarios(t, mainSteps) })
	t.Run("Re-expose service, for next test", func(t *testing.T) { cli.RunScenarios(t, []cli.TestScenario{exposeSteps}) })
	applyCrd(t, pub1)
	t.Run("CRD added and no policy, all comes down", func(t *testing.T) { cli.RunScenarios(t, checkStuffCameDown) })
	t.Log("Removing CRD again, some resources should come back up")
	removeCrd(t, pub1)
	t.Run("CRD removed, link should come back up", func(t *testing.T) { cli.RunScenarios(t, checkStuffCameBackUp) })
	t.Run("closing", func(t *testing.T) { cli.RunScenarios(t, deleteSteps) })

}
