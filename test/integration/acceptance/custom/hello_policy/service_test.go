//go:build policy
// +build policy

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

// Creates the frontend service, following the Hello World specification: always on
// port 8080, named hello-world-frontend, using http, checking whether it works
// and responding taking the 'allowed' configuration into consideration.
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

// Creates the backend service, following the Hello World specification: always on
// port 8080, named hello-world-backend, using http, checking whether it works
// and responding taking the 'allowed' configuration into consideration.
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

// Checks that a list of services is absent.  Different from serviceCheck{Front,Back}TestScenario, this one does not
// run cli.StatusTester, because we just want to ensure some services are not there, so we do not make any claims as
// to what _is_ there.  Therefore, we cannot populate cli.StatusTester.ExposedServices
func serviceCheckAbsentTestScenario(cluster *base.ClusterContext, prefix string, services []string) (scenario cli.TestScenario) {

	serviceInterfaces := genInterfaceList(services, false)

	scenario = cli.TestScenario{

		Name: prefixName(prefix, "service-check-absent"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: cluster, Commands: []cli.SkupperCommandTester{
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

// A helper to transform a list of interface names into []types.ServiceInterface
// with the name, protocol http and port 8080
//
// If bound is true, the ServiceInterface will include a ServiceInterfaceTarget
// whose name and service entries are the same as the given service string.
func genInterfaceList(services []string, bound bool) (interfaces []types.ServiceInterface) {
	for _, service := range services {
		var target = []types.ServiceInterfaceTarget{}

		// For this testing, addresses, names and services are always the same string
		if bound {
			target = []types.ServiceInterfaceTarget{
				{
					Name:        service,
					TargetPorts: map[int]int{8080: 8080},
					Service:     service,
				},
			}
		}

		interfaces = append(
			interfaces,
			types.ServiceInterface{
				Address:  service,
				Protocol: "http",
				Ports:    []int{8080},
				Targets:  target,
			})
	}
	return interfaces
}

// Returns a single &service.StatusTester command; the list of services to be checked
// are given as a simple []string slices, and then transformed to
// []types.ServiceInterface{} internally.
func serviceCheckTestCommand(unboundServices, unauthServices, boundServices []string) (scenario *service.StatusTester) {
	serviceInterfaces := genInterfaceList(unboundServices, false)
	serviceInterfaces = append(serviceInterfaces, genInterfaceList(boundServices, true)...)
	unauthInterfaces := genInterfaceList(unauthServices, false)

	command := &service.StatusTester{
		ServiceInterfaces:             serviceInterfaces,
		UnauthorizedServiceInterfaces: unauthInterfaces,
		CheckAuthorization:            true,
	}
	return command
}

// Returns a test scenario for checking that a set of lists of services are bound or
// authorized as expected.  The lists of services are given as simple []string slices
// containing just their names.
func serviceCheckTestScenario(pub *base.ClusterContext, prefix string, unboundServices, unauthServices, boundServices []string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: prefixName(prefix, "service-status"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub,
				Commands: []cli.SkupperCommandTester{
					serviceCheckTestCommand(unboundServices, unauthServices, boundServices),
				},
			},
		},
	}
	return scenario
}

// skupper service status + skupper status
//
// All services that are to be exposed on the VAN need to be declared on either
// boundServices or unauthServices, as their lenghths are summed in order to provide
// the StatusTester with its ExposedServices field.
//
// TODO: On callers, replace this single call by two calls?  This way, these could be
// run in parallel
func serviceCheckFrontStatusTestScenario(pub *base.ClusterContext, prefix string, unboundServices, unauthServices, boundServices []string) (scenario cli.TestScenario) {

	serviceInterfaces := genInterfaceList(unboundServices, false)
	serviceInterfaces = append(serviceInterfaces, genInterfaceList(boundServices, true)...)
	unauthInterfaces := genInterfaceList(unauthServices, false)

	tasks := []cli.SkupperTask{
		{
			Ctx: pub,
			Commands: []cli.SkupperCommandTester{
				// skupper status - verify frontend service is exposed
				&cli.StatusTester{
					RouterMode:          "interior",
					ConnectedSites:      1,
					ExposedServices:     len(serviceInterfaces) + len(unauthInterfaces),
					ConsoleEnabled:      true,
					ConsoleAuthInternal: true,
					PolicyEnabled:       cli.Boolp(true),
				},
			},
		},
	}

	tasks = append(
		serviceCheckTestScenario(pub, prefixName("front", prefix), unboundServices, unauthServices, boundServices).Tasks,
		tasks...,
	)

	scenario = cli.TestScenario{
		Name:  "service-check-front",
		Tasks: tasks,
	}

	return scenario
}

// skupper service status + skupper status
//
// All services that are to be exposed on the VAN need to be declared on either
// boundServices or unauthServices, as their lenghths are summed in order to provide
// the StatusTester with its ExposedServices field.
//
// TODO: On callers, replace this single call by two calls?  This way, these could be
// run in parallel
func serviceCheckBackStatusTestScenario(prv *base.ClusterContext, prefix string, unboundServices, unauthServices, boundServices []string) (scenario cli.TestScenario) {
	serviceInterfaces := genInterfaceList(unboundServices, false)
	serviceInterfaces = append(serviceInterfaces, genInterfaceList(boundServices, true)...)
	unauthInterfaces := genInterfaceList(unauthServices, false)

	tasks := []cli.SkupperTask{
		{
			Ctx: prv, Commands: []cli.SkupperCommandTester{
				// skupper status - verify two services are now exposed
				&cli.StatusTester{
					RouterMode:      "edge",
					SiteName:        "private",
					ConnectedSites:  1,
					ExposedServices: len(serviceInterfaces) + len(unauthInterfaces),
					PolicyEnabled:   cli.Boolp(true),
				},
			}},
	}

	tasks = append(
		serviceCheckTestScenario(prv, prefixName("back", prefix), unboundServices, unauthServices, boundServices).Tasks,
		tasks...,
	)

	scenario = cli.TestScenario{

		Name:  "service-check-back",
		Tasks: tasks,
	}
	return scenario
}

// This is straight from HelloWorld, it can configure only the scenario name
//
// It does both BindTester and ensuing service.StatusTester
func frontendServiceBindTestScenario(pub *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: prefixName(prefix, "bind-frontend-service"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: pub, Commands: []cli.SkupperCommandTester{
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
							{
								Address:  "hello-world-frontend",
								Protocol: "http", Ports: []int{8080},
								Targets: []types.ServiceInterfaceTarget{
									{
										Name:        "hello-world-frontend",
										TargetPorts: map[int]int{8080: 8080},
										Service:     "hello-world-frontend",
									},
								},
							}, {
								Address:  "hello-world-backend",
								Protocol: "http",
								Ports:    []int{8080},
							},
						},
					},
				},
			},
		},
	}
	return
}

// This is straight from HelloWorld, it can configure only the scenario name
//
// It does both BindTester and ensuing service.StatusTester
func backendServiceBindTestScenario(prv *base.ClusterContext, prefix string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{

		Name: prefixName(prefix, "bind-backend-service"),
		Tasks: []cli.SkupperTask{
			{
				Ctx: prv, Commands: []cli.SkupperCommandTester{
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
							{
								Address:  "hello-world-frontend",
								Protocol: "http",
								Ports:    []int{8080},
							}, {
								Address:  "hello-world-backend",
								Protocol: "http", Ports: []int{8080},
								Targets: []types.ServiceInterfaceTarget{
									{
										Name:        "hello-world-backend",
										TargetPorts: map[int]int{8080: 8080},
										Service:     "hello-world-backend",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return
}

// Returns a test scenario to unbind a specific service, using UnbindTester
func unbindServiceTestScenario(ctx *base.ClusterContext, prefix, svc string) (scenario cli.TestScenario) {

	scenario = cli.TestScenario{
		Name: prefixName(prefix, "service-unbind-"+svc),
		Tasks: []cli.SkupperTask{
			{
				Ctx: ctx,
				Commands: []cli.SkupperCommandTester{
					&service.UnbindTester{
						ServiceName: svc,
						TargetType:  "deployment",
						TargetName:  svc,
					},
				},
			},
		},
	}
	return scenario
}

// This is the main test in this file
//
// Note that the testing on what happens for bindings that are being denied
// due to AllowedExposedResources configuration, go on the resources tests, not here
//
// This test uses some cluster-wise policies ("*" as namespaces) from which it
// checks for effects between public and private namespaces.  For that reason, it
// cannot be used in a multicluster environment
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
					name:     "allow-and-create",
					parallel: true,
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{}, // The first two policies are created empty
						{},
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"^hello-world.*"},
						},
					},
					getChecks: []policyGetCheck{
						{
							cluster:         pub,
							allowedServices: []string{"hello-world-anything"},
						}, {
							cluster:         prv,
							allowedServices: []string{"hello-world-anything"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				}, {
					name:     "check-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
							// Removing the policy by assigning it an empty Namespaces list
							Namespaces: []string{},
						},
					},
					getChecks: []policyGetCheck{
						{
							cluster:            pub,
							disallowedServices: []string{"hello-world-anything"},
							allowedServices:    []string{"hello-world-backend", "hello-world-frontend"},
						}, {
							cluster:            prv,
							disallowedServices: []string{"hello-world-anything"},
							allowedServices:    []string{"hello-world-backend", "hello-world-frontend"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
					getChecks: []policyGetCheck{
						{
							cluster:            prv,
							disallowedServices: []string{"hello-world-anything", "hello-world-frontend"},
							allowedServices:    []string{"hello-world-backend"},
						}, {
							cluster:            pub,
							disallowedServices: []string{"hello-world-backend", "hello-world-anything"},
							allowedServices:    []string{"hello-world-frontend"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend"}, []string{"hello-world-backend"}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-backend"}, []string{"hello-world-frontend"}, []string{}),
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
					getChecks: []policyGetCheck{
						{
							cluster:            prv,
							disallowedServices: []string{"hello-world-anything"},
							allowedServices:    []string{"hello-world-backend", "hello-world-frontend"},
						}, {
							cluster:            pub,
							disallowedServices: []string{"hello-world-anything"},
							allowedServices:    []string{"hello-world-backend", "hello-world-frontend"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
					getChecks: []policyGetCheck{
						{
							cluster:            prv,
							disallowedServices: []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
						}, {
							cluster:            pub,
							disallowedServices: []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
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
					getChecks: []policyGetCheck{
						{
							cluster:            prv,
							allowedServices:    []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
							disallowedServices: []string{"somethingelse"},
						}, {
							cluster:            pub,
							allowedServices:    []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
							disallowedServices: []string{"somethingelse"},
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
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
					getChecks: []policyGetCheck{
						{
							cluster:            prv,
							disallowedServices: []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
						}, {
							cluster:            pub,
							disallowedServices: []string{"hello-world-anything", "hello-world-backend", "hello-world-frontend"},
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
					name: "allow-specific-and-create-services",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{pub.Namespace},
							AllowedServices: []string{".*-frontend"},
						}, {
							Namespaces:      []string{prv.Namespace},
							AllowedServices: []string{".*-backend"},
						},
					},
					getChecks: []policyGetCheck{
						{
							allowedServices:    []string{"asdf-frontend"},
							disallowedServices: []string{"asdf-backend"},
							cluster:            pub,
						}, {
							allowedServices:    []string{"asdf-backend"},
							disallowedServices: []string{"asdf-frontend"},
							cluster:            prv,
						},
					},
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
						serviceCreateBackTestScenario(prv, "", true),
					},
				}, {
					name:     "check-services",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend"}, []string{"hello-world-backend"}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-backend"}, []string{"hello-world-frontend"}, []string{}),
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
						frontendServiceBindTestScenario(pub, ""),
						backendServiceBindTestScenario(prv, ""),
					},
				},
			},
		}, {
			name: "show-on-both",
			steps: []policyTestStep{
				{
					name: "bind-both-services",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{".*-frontend"},
						}, {
							Namespaces:      []string{"*"},
							AllowedServices: []string{".*-backend"},
						},
					},
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-backend"}, []string{}, []string{"hello-world-frontend"}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend"}, []string{}, []string{"hello-world-backend"}),
					},
				},
			},
		}, {
			name: "reorganize--no-effect",
			steps: []policyTestStep{
				{
					name: "reorganize-policies",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{".*-.*"},
						}, {
							Namespaces: []string{"REMOVE"},
						},
					},
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-backend"}, []string{}, []string{"hello-world-frontend"}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend"}, []string{}, []string{"hello-world-backend"}),
					},
				},
			},
		}, {
			name: "unbind",
			steps: []policyTestStep{
				{
					name:     "unbind",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						unbindServiceTestScenario(pub, "", "hello-world-frontend"),
						unbindServiceTestScenario(prv, "", "hello-world-backend"),
					},
				}, {
					name:     "check-unbound",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-backend", "hello-world-frontend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
					},
				},
			},
		}, {
			name: "re-bind",
			steps: []policyTestStep{
				{
					name: "bind-both-services",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces: []string{"KEEP"},
						}, {

							Namespaces:      []string{prv.Namespace},
							AllowedServices: []string{".*-backend"},
						},
					},
					parallel: true,
					cliScenarios: []cli.TestScenario{
						frontendServiceBindTestScenario(pub, ""),
						backendServiceBindTestScenario(prv, ""),
					},
				},
			},
		}, {
			name: "partial-allow",
			steps: []policyTestStep{
				{
					name: "remove-policy--check-services",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces: []string{"REMOVE"},
						},
						// Keep the second one
					},
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{}, []string{"hello-world-backend"}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{}, []string{}, []string{"hello-world-backend"}),
						serviceCheckAbsentTestScenario(pub, "frontend", []string{"hello-world-frontend"}),
					},
				},
			},
		}, {
			name: "re-add--re-create--not-bound",
			steps: []policyTestStep{
				{
					name: "readd-allow-all-services-policy",
					pubPolicy: []v1alpha1.SkupperClusterPolicySpec{
						{
							Namespaces:      []string{"*"},
							AllowedServices: []string{"*"},
						},
					},
					cliScenarios: []cli.TestScenario{
						serviceCreateFrontTestScenario(pub, "", true),
					},
				}, {
					name:     "services-there--but-not-bound",
					parallel: true,
					cliScenarios: []cli.TestScenario{
						serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-backend", "hello-world-backend"}, []string{}, []string{}),
						serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-backend"}, []string{}, []string{"hello-world-backend"}),
					},
				}, {
					name: "todo-skip",
					skip: func() string {
						// Note that it is services-there--but-not-bound that needs fixing; this one is here just to document, and
						// should be removed once that one is fixed.  TODO
						return "TODO: services.StatusTester needs refactored for proper non-bound check"
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
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
				serviceCheckFrontStatusTestScenario(pub, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
				serviceCheckBackStatusTestScenario(prv, "", []string{"hello-world-frontend", "hello-world-backend"}, []string{}, []string{}),
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
			//		}, {
			//			// This is testing for https://github.com/skupperproject/skupper/issues/728
			//			name:  "allow-policy--and--immediatelly-create-service",
			//			steps: allowAndCreate(pub, prv, 100),
			//		}, {
			//			// This is testing for https://github.com/skupperproject/skupper/issues/727
			//			name:  "remove-policy-reallow--check-service-removed",
			////			steps: removeReallowKeep(pub, prv, 500),
			//		}, {
			//			// This is testing for ???
			//			name:  "remove-policy--remove-service",
			//			steps: removePolicyRemoveServices(pub, prv, 400),
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
