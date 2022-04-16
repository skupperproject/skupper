package hello_policy

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
)

func serviceCreateFrontTestScenario(pub *base.ClusterContext, prefix string, allowed bool) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: "service-create-frontend",
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the frontend service and verify
					&service.CreateTester{
						Name:            "hello-world-frontend",
						Port:            8080,
						Mapping:         "http",
						PolicyProhibits: !allowed,
					},
				},
			},
		},
	}
	return
}

func serviceCreateBackTestScenario(prv *base.ClusterContext, prefix string, allowed bool) (scenario cli.TestScenario) {
	scenario = cli.TestScenario{

		Name: "service-create-backend",
		Tasks: []cli.SkupperTask{
			{
				Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service create - creates the backend service and verify
					&service.CreateTester{
						Name:            "hello-world-backend",
						Port:            8080,
						Mapping:         "http",
						PolicyProhibits: !allowed,
					},
				},
			},
		},
	}
	return scenario
}

// A helper to transform a list of interface names into []types.ServiceInterface
// with the name, protocol http and port 8080
func genInterfaceList(services []string) (interfaces []types.ServiceInterface) {
	for _, service := range services {
		interfaces = append(
			interfaces,
			types.ServiceInterface{
				Address:  service,
				Protocol: "http",
				Ports:    []int{8080},
			})
	}
	return interfaces
}

func serviceCheckFrontTestScenario(pub *base.ClusterContext, prefix string, services []string, unauthServices []string) (scenario cli.TestScenario) {

	serviceInterfaces := genInterfaceList(services)
	//unauthInterfaces := getInterfaceList(unauthServices)

	scenario = cli.TestScenario{

		Name: "service-check-front",
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces: serviceInterfaces,
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     len(serviceInterfaces) + len(unauthServices),
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
						PolicyEnabled:       true,
					},
				},
			},
		},
	}
	return scenario
}

// Checks that a list of services is absent.  Different from serviceCheck{Front,Back}TestScenario, this one does not
// run cli.StatusTester, because we just want to ensure some services are not there, so we do not make any claims as
// for what _is_ there.  Therefore, we cannot populate cli.StatusTester.ExposedServices
func serviceCheckAbsentTestScenario(cluster *base.ClusterContext, prefix string, services []string) (scenario cli.TestScenario) {

	serviceInterfaces := genInterfaceList(services)
	//unauthInterfaces := getInterfaceList(unauthServices)

	scenario = cli.TestScenario{

		Name: prefixName(prefix, "service-check-absent"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: cluster, Commands: []cli.SkupperCommandTester{
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces: serviceInterfaces,
						Absent:            true,
					},
				},
			},
		},
	}
	return
}

func serviceCheckBackTestScenario(prv *base.ClusterContext, prefix string, services []string, unauthServices []string) (scenario cli.TestScenario) {
	serviceInterfaces := genInterfaceList(services)

	scenario = cli.TestScenario{

		Name: "service-check-back",
		Tasks: []cli.SkupperTask{
			{
				Ctx: prv, Commands: []cli.SkupperCommandTester{
					// skupper service status - validate status of the two created services without targets
					&service.StatusTester{
						ServiceInterfaces: serviceInterfaces,
					},
					// skupper status - verify two services are now exposed
					&cli.StatusTester{
						RouterMode:      "edge",
						SiteName:        "private",
						ConnectedSites:  1,
						ExposedServices: len(serviceInterfaces) + len(unauthServices),
						PolicyEnabled:   true,
					},
				}},
		},
	}
	return scenario
}

func serviceBindTestScenario(pub, prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: "service-check-back",
		Tasks: []cli.SkupperTask{
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
	}
	return
}

func testServicePolicy(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "skupper-init",
					parallel: true,
					commands: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					commands: []cli.TestScenario{
						connectSitesTestScenario(pub, prv, "", "service"),
					},
				},
			},
		}, {
			name: "no-policy-service-creation-fails",
			steps: []policyTestStep{
				{
					name:     "create-services",
					parallel: true,
					commands: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", false),
						serviceCreateBackTestScenario(prv, "", false),
					},
				},
			},
		}, {
			name: "all-hello-world-works",
			steps: []policyTestStep{
				{
					name:     "create-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{}, // The first two policies are created empty
						{},
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world.*"},
						},
					},
					commands: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				}, {
					name:     "check-services",
					parallel: true,
					commands: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
					},
				},
			},
		}, {
			name: "add-specific-policies--remove-general--no-changes",
			steps: []policyTestStep{
				{
					name:     "check-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world-backend"},
						},
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world-frontend"},
						},
						{
							Namespaces: []string{},
						},
					},
					commands: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
					},
				},
			},
		}, {
			name: "make-policies-specific-to-namespace",
			steps: []policyTestStep{
				{
					name:     "check-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{prv.Namespace},
							AllowedServices: []string{"^hello-world-backend"},
						},
						{
							Namespaces:      []string{pub.Namespace},
							AllowedServices: []string{"^hello-world-frontend"},
						},
					},
					commands: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend"}, []string{"hello-world-backend"}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-backend"}, []string{"hello-world-frontend"}),
					},
				},
			},
		}, {
			name: "policies-list-both-services",
			steps: []policyTestStep{
				{
					name:     "check-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{prv.Namespace},
							AllowedServices: []string{"^hello-world-backend", "^hello-world-frontend"},
						},
						{
							Namespaces:      []string{pub.Namespace},
							AllowedServices: []string{"^hello-world-backend", "^hello-world-frontend"},
						},
					},
					commands: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
					},
				},
			},
		}, {
			name: "policy-removals",
			steps: []policyTestStep{
				{
					name:     "check-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"some-other-namespace"},
							AllowedServices: []string{"^hello-world-backend", "^hello-world-frontend"},
						},
						{
							Namespaces: []string{"REMOVE"},
						},
					},
					commands: []cli.TestScenario{
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
						serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
					},
				}, {
					name:     "create-services-fail",
					parallel: true,
					commands: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", false),
						serviceCreateBackTestScenario(prv, "", false),
					},
				},
			},
		}, {
			name: "reinstating-and-gone",
			steps: []policyTestStep{
				{
					name:     "check-services",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world-.*"},
						},
					},
					commands: []cli.TestScenario{
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
						serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
					},
				}, {
					name:     "create-services-work",
					parallel: true,
					commands: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				},
				{
					name:     "check-services",
					parallel: true,
					commands: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
					},
				},
			},
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "delete",
					parallel: true,
					commands: []cli.TestScenario{
						deleteSkupperTestScenario(pub, "pub"),
						deleteSkupperTestScenario(prv, "prv"),
					},
				},
			},
		},
	}

	policyTestRunner{
		scenarios:    testTable,
		keepPolicies: true,
		pubPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
			},
		},
		// Add background policies; policies that are not removed across
		// runs
	}.run(t, pub, prv)

}
