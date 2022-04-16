package hello_policy

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/skupper/cli"
	"github.com/skupperproject/skupper/test/utils/skupper/cli/service"
)

func serviceCreateFront(pub *base.ClusterContext, prefix string, allowed bool) (scenario cli.TestScenario) {

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

func serviceCreateBack(prv *base.ClusterContext, prefix string, allowed bool) (scenario cli.TestScenario) {
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

func serviceCreateFrontBack(pub, prv *base.ClusterContext, prefix string, pubAllowed, prvAllowed bool) (scenario cli.TestScenario) {
	scenario = serviceCreateFront(pub, prefix, pubAllowed)

	scenario.AppendTasks(serviceCreateBack(prv, prefix, prvAllowed))

	scenario.Name = prefixName(prefix, "service-create-front-and-back")

	return scenario
}

func serviceCheckFront(pub *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: "service-check-front",
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub, Commands: []cli.SkupperCommandTester{
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
				},
			},
		},
	}
	return scenario
}

func serviceCheckBack(prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {
	scenario = cli.TestScenario{

		Name: "service-check-back",
		Tasks: []cli.SkupperTask{
			{
				Ctx: prv, Commands: []cli.SkupperCommandTester{
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
		},
	}
	return scenario
}

func serviceCheckFrontBack(pub, prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = serviceCheckFront(pub, prefix)
	scenario.AppendTasks(serviceCheckBack(prv, prefix))
	scenario.Name = "service-check-front-and-back"
	return
}

func serviceBind(pub, prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

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
						serviceCreateFront(pub, "", false),
						serviceCreateBack(prv, "", false),
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
		scenarios: testTable,
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
