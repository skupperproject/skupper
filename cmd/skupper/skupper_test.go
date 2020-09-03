package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

func TestMain(m *testing.M) {
	silenceCobra()
	os.Exit(m.Run())
}

func Test_requiredArg(t *testing.T) {
	r := func(args []string) error {
		return requiredArg("testArg")(nil, args)
	}

	assert.Error(t, r([]string{}), "testArg must be specified")
	assert.Error(t, r([]string{"too", "many"}), "illegal argument: many")
	assert.Error(t, r([]string{"too", "many", "more"}), "illegal argument: many")

	assert.Assert(t, r([]string{"OneArgument"}))
}

func Test_bindArgs(t *testing.T) {
	genericError := "Service name, target type and target name must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-name>')"
	b := func(args []string) error {
		return bindArgs(nil, args)
	}

	assert.Error(t, b([]string{}), genericError)
	assert.Error(t, b([]string{"oneArg"}), genericError)
	assert.Error(t, b([]string{"one/Arg"}), genericError)
	assert.Error(t, b([]string{"one", "resource"}), genericError)

	//must this fail?
	//assert.Error(t, b([]string{"one/two", "resource/name"}), genericError)

	assert.Assert(t, b([]string{"one", "resource/name"}))
	//note  illegal vs extra
	assert.Error(t, b([]string{"one", "resource/name", "three"}), "extra argument: three")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource/name", "three", "four", "five"}), "illegal argument: four")

	assert.Assert(t, b([]string{"one", "resource", "name"}))
	assert.Error(t, b([]string{"one", "resource", "name", "four"}), "illegal argument: four")
	assert.Error(t, b([]string{"one", "resource", "name", "four", "five"}), "illegal argument: four")
}

func Test_createServiceArgs(t *testing.T) {
	c := func(args []string) error {
		return createServiceArgs(nil, args)
	}

	assert.Error(t, c([]string{}), "Name and port must be specified")
	assert.Error(t, c([]string{"noport"}), "Name and port must be specified")

	assert.Assert(t, c([]string{"service:port"}))

	assert.Error(t, c([]string{"service:port", "other"}), "extra argument: other")
	assert.Error(t, c([]string{"service:port", "other", "arg"}), "illegal argument: arg")

	assert.Assert(t, c([]string{"service", "port"}))
	assert.Error(t, c([]string{"service", "port", "other"}), "illegal argument: other")
	assert.Error(t, c([]string{"service", "port", "other", "arg"}), "illegal argument: other")
}

func Test_unexposeTargetArgs(t *testing.T) {
	genericError := "expose target and name must be specified (e.g. 'skupper expose deployment <name>'"
	targetError := "expose target type must be one of: [deployment, statefulset, pods, service]"

	u := func(args []string) error {
		return unexposeTargetArgs(nil, args)
	}

	assert.Error(t, u([]string{}), genericError)
	assert.Error(t, u([]string{"depl/name"}), targetError)

	assert.Error(t, u([]string{"depl/name", "two"}), "extra argument: two")
	assert.Error(t, u([]string{"depl/name", "two", "three"}), "illegal argument: three")
	assert.Error(t, u([]string{"depl/name", "two", "three", "four"}), "illegal argument: three")

	assert.Error(t, u([]string{"depl/name"}), targetError)
	assert.Error(t, u([]string{"anything", "name"}), targetError)

	assert.Error(t, u([]string{"deployment"}), genericError)

	assert.Assert(t, u([]string{"deployment", "name"}))

	assert.Error(t, u([]string{"deployment", "name", "three"}), "illegal argument: three")
	assert.Error(t, u([]string{"deployment", "name", "three", "four"}), "illegal argument: three")

	for _, target := range validExposeTargets {
		assert.Assert(t, u([]string{target, "name"}))
	}
}

func Test_exposeTargetArgs(t *testing.T) {
	err := exposeTargetArgs(nil, []string{"service/name"})
	assert.Error(t, err, "The --address option is required for target type 'service'")

	options.expose.Address = "someAddress"
	err = exposeTargetArgs(nil, []string{"service/name"})
	assert.Assert(t, err)
}

type serviceInterfaceUnbindCallArgs struct {
	targetType, targetName, address string
	deleteIfNoTargets               bool
}

type serviceInterfaceBindCallArgs struct {
	service    *types.ServiceInterface
	targetType string
	targetName string
	protocol   string
	targetPort int
}

type getHeadlessServiceConfigurationCallArgs struct {
	targetName string
	protocol   string
	address    string
	port       int
}

type serviceInterfaceAndErrorReturns struct {
	serviceInterface *types.ServiceInterface
	err              error
}

type vanClientMockInjectedReturnValues struct {
	serviceInterfaceUnbind          error
	serviceInterfaceBind            error
	serviceInterfaceInspect         serviceInterfaceAndErrorReturns
	serviceInterfaceUpdate          error
	getHeadlessServiceConfiguration serviceInterfaceAndErrorReturns
}

type vanClientMock struct {
	serviceInterfaceUnbindCalledWith          []serviceInterfaceUnbindCallArgs
	serviceInterfaceBindCalledWith            []serviceInterfaceBindCallArgs
	serviceInterfaceInspectCalledWith         []string
	getHeadlessServiceConfigurationCalledWith []getHeadlessServiceConfigurationCallArgs
	serviceInterfaceUpdateCalledWith          []*types.ServiceInterface
	injectedReturns                           vanClientMockInjectedReturnValues
}

func (v *vanClientMock) ResetCallHistory() {
	v.serviceInterfaceBindCalledWith = nil
	v.serviceInterfaceUnbindCalledWith = nil
	v.serviceInterfaceInspectCalledWith = nil
	v.getHeadlessServiceConfigurationCalledWith = nil
	v.serviceInterfaceUpdateCalledWith = nil
}

func (v *vanClientMock) ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool) error {
	var calledWith = serviceInterfaceUnbindCallArgs{
		targetType:        targetType,
		targetName:        targetName,
		address:           address,
		deleteIfNoTargets: deleteIfNoTargets,
	}
	v.serviceInterfaceUnbindCalledWith = append(v.serviceInterfaceUnbindCalledWith, calledWith)

	return v.injectedReturns.serviceInterfaceUnbind
}

func (v *vanClientMock) ServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, protocol string, targetPort int) error {
	var calledWith = serviceInterfaceBindCallArgs{
		service:    service,
		targetType: targetType,
		targetName: targetName,
		protocol:   protocol,
		targetPort: targetPort,
	}
	v.serviceInterfaceBindCalledWith = append(v.serviceInterfaceBindCalledWith, calledWith)

	return v.injectedReturns.serviceInterfaceBind
}

func (v *vanClientMock) ServiceInterfaceInspect(ctx context.Context, address string) (*types.ServiceInterface, error) {
	v.serviceInterfaceInspectCalledWith = append(v.serviceInterfaceInspectCalledWith, address)
	return v.injectedReturns.serviceInterfaceInspect.serviceInterface, v.injectedReturns.serviceInterfaceInspect.err
}

func (v *vanClientMock) ServiceInterfaceUpdate(ctx context.Context, service *types.ServiceInterface) error {
	v.serviceInterfaceUpdateCalledWith = append(v.serviceInterfaceUpdateCalledWith, service)
	return v.injectedReturns.serviceInterfaceUpdate
}

func (v *vanClientMock) GetHeadlessServiceConfiguration(targetName string, protocol string, address string, port int) (*types.ServiceInterface, error) {
	var calledWith = getHeadlessServiceConfigurationCallArgs{
		targetName: targetName,
		protocol:   protocol,
		address:    address,
		port:       port,
	}
	v.getHeadlessServiceConfigurationCalledWith = append(v.getHeadlessServiceConfigurationCalledWith, calledWith)
	return v.injectedReturns.getHeadlessServiceConfiguration.serviceInterface, v.injectedReturns.getHeadlessServiceConfiguration.err
}

func (cli *vanClientMock) GetNamespace() string {
	return "MockNamespace"
}

func Test_cmdUnexpose(t *testing.T) {
	test := func(targetType, targetName, address string, cli *vanClientMock) {
		options := Options{
			unexposeAddress: address,
		}

		args := []string{targetType}

		//supporting "targetType TargetName" and "targetType/targetName" notations
		if targetName != "" {
			args = append(args, targetName)
		} else {
			parts := strings.Split(targetType, "/")
			targetType = parts[0]
			targetName = parts[1]
		}

		err := unexposeRun(nil, args, options, cli)

		if cli.injectedReturns.serviceInterfaceUnbind != nil {
			assert.Error(t, err, "Error, unable to skupper service: "+cli.injectedReturns.serviceInterfaceUnbind.Error())
		} else {
			assert.Assert(t, err)
		}

		assert.Equal(t, len(cli.serviceInterfaceUnbindCalledWith), 1)

		expected := serviceInterfaceUnbindCallArgs{
			targetType:        targetType,
			targetName:        targetName,
			address:           address,
			deleteIfNoTargets: true}

		assert.Assert(t, cmp.Equal(cli.serviceInterfaceUnbindCalledWith[0], expected, cmp.AllowUnexported(serviceInterfaceUnbindCallArgs{})))
	}

	testSuccess := func(targetType, targetName, address string) {
		cli := &vanClientMock{}
		test(targetType, targetName, address, cli)
	}

	testError := func(targetType, targetName, address string, errorString string) {
		cli := &vanClientMock{
			injectedReturns: vanClientMockInjectedReturnValues{
				serviceInterfaceUnbind: fmt.Errorf("%s", errorString),
			},
		}
		test(targetType, targetName, address, cli)
	}

	testSuccess("depl", "Name", "theService:8080")
	testSuccess("depl/Name", "", "theService:8080")

	testError("depl", "Name", "theService:8080", "some error")
	testError("depl/Name", "", "theService:8080", "other error")
}

func Test_cmdUnexposeParseArgs(t *testing.T) {
	cmd_args := []string{"unexpose", "deployment/name", "--address", "theAddress"}
	expected_subcmd_args := cmd_args[1:]
	command, subcommand_args, err := rootCmd.Find(cmd_args)
	assert.Assert(t, err)
	assert.Assert(t, cmp.Equal(expected_subcmd_args, subcommand_args))

	assert.Assert(t, command.ParseFlags([]string{}))
	assert.Equal(t, options.unexposeAddress, "")

	assert.Assert(t, command.ParseFlags(expected_subcmd_args))
	assert.Equal(t, options.unexposeAddress, "theAddress")

	//Probably this is excessive testing, as we are testing the cobra library
	//itself, but, it is free!
	assert.Error(t, command.ParseFlags([]string{"--address"}),
		"flag needs an argument: --address")
}

func Test_cmdExpose_Binding(t *testing.T) {

	compare := func(a, b *serviceInterfaceBindCallArgs) {
		t.Helper()
		assert.Assert(t, a.targetType == b.targetType)
		assert.Assert(t, a.targetName == b.targetName)
		assert.Assert(t, a.protocol == b.protocol)
		assert.Assert(t, a.targetPort == b.targetPort)
		assert.Assert(t, a.service.Address == b.service.Address)
		assert.Assert(t, a.service.Protocol == b.service.Protocol)
		assert.Assert(t, a.service.Port == b.service.Port)
	}
	options := Options{}

	test_protocol := "protocol"
	options.expose.Address = "TheService"
	options.expose.Headless = false
	options.expose.Protocol = test_protocol
	options.expose.Port = 123
	options.expose.TargetPort = 234

	expectedBindCall := serviceInterfaceBindCallArgs{
		service: &types.ServiceInterface{
			Address:  "TheService",
			Protocol: test_protocol,
			Port:     123,
		},
		targetType: "any",
		targetName: "name",
		protocol:   test_protocol,
		targetPort: 234,
	}

	t.Run("service not existent and options.expose.headless == false",
		func(t *testing.T) {
			cli := vanClientMock{}

			err := exposeRun(nil, []string{"any/name"}, options, &cli)
			assert.Assert(t, err)
			assert.Equal(t, len(cli.serviceInterfaceBindCalledWith), 1)
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)

		})

	t.Run("service exists and Bind is successfull",
		func(t *testing.T) {
			cli := vanClientMock{}
			aService := &types.ServiceInterface{
				Address:  "TheOtherService",
				Port:     options.expose.Port,
				Protocol: options.expose.Protocol,
			}
			expectedBindCall := expectedBindCall
			expectedBindCall.service = aService
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = aService

			err := exposeRun(nil, []string{"any/name"}, options, &cli)
			assert.Assert(t, err)

			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: isNotFound",
		func(t *testing.T) {
			cli := vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = fmt.Errorf("some error")
			err := exposeRun(nil, []string{"any/name"}, options, &cli)
			assert.Error(t, err, "Error, unable to create skupper service: some error")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: anyError",
		func(t *testing.T) {
			cli := vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = errors.NewNotFound(schema.GroupResource{}, "name")
			err := exposeRun(nil, []string{"any/name"}, options, &cli)
			assert.Error(t, err, "Skupper is not installed in Namespace 'MockNamespace`")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

}

func Test_cmdExpose(t *testing.T) {

	var err error
	options := Options{}
	options.expose.Address = "ServiceName"

	t.Run("ServiceInterfaceInspect returns error",
		func(t *testing.T) {
			cli := vanClientMock{}
			cli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
			err := exposeRun(nil, []string{"deployment/name"}, options, &cli)
			assert.Error(t, err, "some error")
			assert.Equal(t, cli.serviceInterfaceInspectCalledWith[0], "ServiceName")
		})

	options.expose.Headless = true

	t.Run("service not existent, headless option set, and targetType != statefulset ",
		func(t *testing.T) {
			cli := vanClientMock{}
			cli = vanClientMock{
				injectedReturns: vanClientMockInjectedReturnValues{
					serviceInterfaceInspect: serviceInterfaceAndErrorReturns{
						serviceInterface: nil,
						err:              nil,
					},
				},
			}

			err = exposeRun(nil, []string{"service/name"}, options, &cli)
			assert.Error(t, err, "The headless option is only supported for statefulsets")
		})

	options.expose.Protocol = "theprotocol"
	options.expose.Port = 123

	t.Run("service not existent, headless option set, and targetType == statefulset ",
		func(t *testing.T) {
			cli := vanClientMock{}
			aService := &types.ServiceInterface{}
			cli.injectedReturns.getHeadlessServiceConfiguration.serviceInterface = aService

			err = exposeRun(nil, []string{"statefulset/name"}, options, &cli)
			assert.Assert(t, err)

			assert.Equal(t, len(cli.getHeadlessServiceConfigurationCalledWith), 1)
			assert.Equal(t, len(cli.serviceInterfaceUpdateCalledWith), 1)

			expectedGetHead := getHeadlessServiceConfigurationCallArgs{
				targetName: "name",
				protocol:   options.expose.Protocol,
				address:    "ServiceName",
				port:       options.expose.Port,
			}

			assert.Assert(t, cmp.Equal(cli.getHeadlessServiceConfigurationCalledWith[0], expectedGetHead, cmp.AllowUnexported(getHeadlessServiceConfigurationCallArgs{})))
			assert.Assert(t, cli.serviceInterfaceUpdateCalledWith[0] == aService)
		})

	t.Run("serviceInterfaceInspect returns an existent service and options are wrong",
		func(t *testing.T) {

			cli := vanClientMock{}
			test_protocol := "protocol"
			options.expose.Headless = true
			options.expose.Protocol = test_protocol + "diff"
			injectedService := &types.ServiceInterface{
				Protocol: test_protocol,
				Headless: &types.Headless{
					Name: "NotNil",
				},
			}
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService

			err = exposeRun(nil, []string{"service/name"}, options, &cli)
			assert.Error(t, err, "Service already exposed as headless")

			injectedService.Headless = nil
			err = exposeRun(nil, []string{"service/name"}, options, &cli)
			assert.Error(t, err, "Service already exposed, cannot reconfigure as headless")

			options.expose.Headless = false
			err = exposeRun(nil, []string{"service/name"}, options, &cli)
			assert.Error(t, err, fmt.Sprintf("Invalid protocol %s for service with mapping %s", options.expose.Protocol, injectedService.Protocol))
		})
}

func Test_parseTargetTypeAndName(t *testing.T) {
	targetType, targetName := parseTargetTypeAndName([]string{"type", "name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")

	targetType, targetName = parseTargetTypeAndName([]string{"type/name"})
	assert.Equal(t, targetType, "type")
	assert.Equal(t, targetName, "name")
}
