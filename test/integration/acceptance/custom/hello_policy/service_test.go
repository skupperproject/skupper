package hello_policy

import (
	"testing"
	"time"

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
	unauthInterfaces := genInterfaceList(unauthServices)

	scenario = cli.TestScenario{

		Name: "service-check-front",
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub, Commands: []cli.SkupperCommandTester{
					// skupper service status - verify frontend service is exposed
					&service.StatusTester{
						ServiceInterfaces:             serviceInterfaces,
						UnauthorizedServiceInterfaces: unauthInterfaces,
					},
					// skupper status - verify frontend service is exposed
					&cli.StatusTester{
						RouterMode:          "interior",
						ConnectedSites:      1,
						ExposedServices:     len(serviceInterfaces) + len(unauthServices),
						ConsoleEnabled:      true,
						ConsoleAuthInternal: true,
						PolicyEnabled:       cli.Boolp(true),
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
						PolicyEnabled:   cli.Boolp(true),
					},
				}},
		},
	}
	return scenario
}

func serviceBindTestScenario(pub, prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: "bind-services",
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

// This is the main test in this file
//
// Note that the testing on what happens for bindings that are being denied
// due to AllowedExposedResources configuration, go on the resources tests, not here
func testServicePolicy(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "skupper-init",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					cliScenarios: []cli.TestScenario{
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
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", false),
						serviceCreateBackTestScenario(prv, "", false),
					},
				},
			},
		}, {
			name: "all-hello-world-works",
			steps: []policyTestStep{
				{
					name: "allow-and-wait",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{}, // The first two policies are created empty
						{},
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world.*"},
						},
					},
					sleep: 10 * time.Second,
				}, {
					name:     "create-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				}, {
					name:     "check-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
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
						}, {
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world-frontend"},
						}, {
							Namespaces: []string{},
						},
					},
					cliScenarios: []cli.TestScenario{
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
					cliScenarios: []cli.TestScenario{
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
					cliScenarios: []cli.TestScenario{
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
					cliScenarios: []cli.TestScenario{
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
						serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
					},
				}, {
					name:     "create-services-fail",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", false),
						serviceCreateBackTestScenario(prv, "", false),
					},
				},
			},
		}, {
			name: "reinstating-and-gone",
			steps: []policyTestStep{
				{
					name:     "check-services-gone",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world-.*"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
						serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
					},
				}, {
					name:     "create-services-work",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				},
				{
					name:     "check-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
					},
				},
			},
		}, {
			name: "allow-but-not-this",
			steps: []policyTestStep{
				{
					name:     "disallow-by-allowing-only-others",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"non-existing-service"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
						serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
					},
				},
			},
		}, {
			name: "init-for-binding",
			steps: []policyTestStep{
				{
					name: "allow-specific-and-wait",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{pub.Namespace},
							AllowedServices: []string{".*-frontend"},
						}, {
							Namespaces:      []string{prv.Namespace},
							AllowedServices: []string{".*-backend"},
						},
					},
					sleep: 10 * time.Second,
				}, {
					name:     "create-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				}, {
					name:     "check-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend"}, []string{"hello-world-backend"}),
						serviceCheckBackTestScenario(prv, "", []string{"hello-world-backend"}, []string{"hello-world-frontend"}),
					},
				},
			},
		}, {
			name: "first-binding",
			steps: []policyTestStep{
				{
					name:     "bind-both-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceBindTestScenario(pub, prv, ""),
					},
				},
			},
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "delete",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						deleteSkupperTestScenario(pub, "pub"),
						deleteSkupperTestScenario(prv, "prv"),
					},
				},
			},
		},
	}

	policyTestRunner{
		testCases:    testTable,
		keepPolicies: true,
		pubPolicies: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:                    []string{"*"},
				AllowIncomingLinks:            true,
				AllowedOutgoingLinksHostnames: []string{"*"},
				AllowedExposedResources:       []string{"*"},
			},
		},
		// Add background policies; policies that are not removed across
		// runs
	}.run(t, pub, prv)

}

// If a service is up and running, then we remove all policies and immediatelly reallow them: will
// the services be removed?
//
// The result may depend on cluster load and other factors, so we run it multiple times (runs+1)
//
// The test removes the policy in two different ways: one is actual removal, the other is reusing the
// policy, but removing only the target namespace from its list
//
// TODO: test with changing the setting on AllowedServices, instead?
//
// This test expects the service to be removed.  See removeReallowKeep for the alternative.  Only
// one of them should be part of the test set, but which is unclear.
func removeReallowRemove(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	baseTestSteps := []policyTestStep{
		{
			name:     "add-policy--check-services-absent",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		}, {
			name:     "create-service",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			name:     "check-services-created",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
				serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
			},
		}, {
			name: "remove-policy",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing-namespace"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			// We do not run anything on this policy, as we want to the next step (recreating the
			// policy) to be executed right away
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		allTestSteps = append(allTestSteps, baseTestSteps...)
	}

	return
}

// If a service is up and running, then we remove all policies and immediatelly reallow them: will
// the services be removed?
//
// The result may depend on cluster load and other factors, so we run it multiple times (runs+1)
//
// The test removes the policy in two different ways: one is actual removal, the other is reusing the
// policy, but removing only the target namespace from its list
//
// TODO:
// - test with changing the setting on AllowedServices, instead
// - test with switching the policy: remove existing policy but add another that allows
//
// This test expects the service to be kept.  See removeReallowKeep for the alternative.  Only
// one of them should be part of the test set, but which is unclear.
func removeReallowKeep(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	// TODO: IDEA: add an ever increasing sleep between removal and recreation; check
	// at what point it changes

	allTestSteps = append(allTestSteps, policyTestStep{
		name:     "initial-service-creation",
		parallel: true,
		pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:      []string{pub.Namespace},
				AllowedServices: []string{"*"},
			}, {
				Namespaces:      []string{prv.Namespace},
				AllowedServices: []string{"*"},
			},
		},
		cliScenarios: []cli.TestScenario{
			serviceCreateFrontTestScenario(pub, "", true),
			serviceCreateBackTestScenario(prv, "", true),
		},
	})

	baseTestSteps := []policyTestStep{
		{
			name:     "add-policy--check-services-present",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
				serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
			},
		}, {
			name: "remove-policy",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing-namespace"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			// We do not run anything on this policy, as we want to the next step (recreating the
			// policy) to be executed right away
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return
}

// Can a service be created immediatelly after a policy allowing it?  If not, do we get a
// proper error message for that?
func allowAndCreate(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	allTestSteps = append(allTestSteps, policyTestStep{
		// These policies are created just so the first cycle of the
		// baseTestSteps has something to remove
		name: "initial-dummy-policies",
		pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
			{
				Namespaces:      []string{pub.Namespace},
				AllowedServices: []string{"*"},
			}, {
				Namespaces:      []string{prv.Namespace},
				AllowedServices: []string{"*"},
			},
		},
	})

	baseTestSteps := []policyTestStep{
		{
			name:     "remove-policy--check-services-absent",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		}, {
			name: "add-policy--create-service",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			// We try to add services right after giving policy permission.  Should that work, should that fail?
			// If it fails, it should give a failure response.
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			// This is the crux of this testing.  If we got to this point, the service should have been created
			name:     "check-services-created",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
				serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
			},
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		// s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return

}

// As the name says: if the last policy that allows a service to exist is removed, the
// service needs to be removed by the system
func removePolicyRemoveServices(pub, prv *base.ClusterContext, runs int) (allTestSteps []policyTestStep) {

	baseTestSteps := []policyTestStep{
		{
			// We're just adding the policy, so there should be no services
			// around.  Neither at start of test, nor at start of cycle
			// (that's what we're testing, that policy removal got rid of
			// running services)
			name: "add-policy--check-services-absent",
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{pub.Namespace},
					AllowedServices: []string{"*"},
				}, {
					Namespaces:      []string{prv.Namespace},
					AllowedServices: []string{"*"},
				},
			},
			// We wait a few seconds to ensure we do not fall on
			// https://github.com/skupperproject/skupper/issues/728
			sleep: time.Duration(10) * time.Second,
		}, {
			name:     "create-services",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCreateFrontTestScenario(pub, "", true),
				serviceCreateBackTestScenario(prv, "", true),
			},
		}, {
			name:     "check-services",
			parallel: true,
			cliScenarios: []cli.TestScenario{
				serviceCheckFrontTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
				serviceCheckBackTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}),
			},
		}, {
			// This is the crux of this testing.  If we remove policies, the services
			// must be removed
			name:     "remove-policy--check-services-removed",
			parallel: true,
			pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
				{
					Namespaces:      []string{"non-existing"},
					AllowedServices: []string{"*"},
				}, {
					Namespaces: []string{"REMOVE"},
				},
			},
			cliScenarios: []cli.TestScenario{
				serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend", "hello-world-backend"}),
				serviceCheckAbsentTestScenario(prv, "backend", []string{"hello-world-frontend", "hello-world-backend"}),
			},
		},
	}

	// The first execution is just preparation; we need to run at the very least two full cycles
	for i := 0; i < runs+1; i++ {
		s := baseTestSteps
		// s[1].sleep = time.Duration(5000/runs*i) * time.Millisecond
		allTestSteps = append(allTestSteps, s...)
	}

	return

}

// This is a good candidate to remove on t.Short(), or to skip by default
func testServicePolicyTransitions(t *testing.T, pub, prv *base.ClusterContext) {

	testTable := []policyTestCase{
		{
			name: "initialize",
			steps: []policyTestStep{
				{
					name:     "skupper-init",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						skupperInitInteriorTestScenario(pub, "", true),
						skupperInitEdgeTestScenario(prv, "", true),
					},
				}, {
					name: "connect",
					cliScenarios: []cli.TestScenario{
						connectSitesTestScenario(pub, prv, "", "service"),
					},
				},
			},
		}, {
			// This is testing for https://github.com/skupperproject/skupper/issues/728
			name:  "allow-policy--and--immediatelly-create-service",
			steps: allowAndCreate(pub, prv, 100),
		}, {
			// This is testing for https://github.com/skupperproject/skupper/issues/727
			name:  "remove-policy-reallow--check-service-removed",
			steps: removeReallowKeep(pub, prv, 500),
		}, {
			// This is testing for ???
			name:  "remove-policy--remove-service",
			steps: removePolicyRemoveServices(pub, prv, 400),
		}, {
			name: "cleanup",
			steps: []policyTestStep{
				{
					name:     "delete",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						deleteSkupperTestScenario(pub, "pub"),
						deleteSkupperTestScenario(prv, "prv"),
					},
				},
			},
		},
	}

	policyTestRunner{
		testCases: testTable,
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
