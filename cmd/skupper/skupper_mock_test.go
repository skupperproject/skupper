package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

type siteConfigInspectCallArgs struct {
	targetName string
	protocol   string
	address    string
	port       int
}

type serviceInterfaceAndErrorReturns struct {
	serviceInterface *types.ServiceInterface
	err              error
}

type siteConfigAndErrorReturns struct {
	siteConfig *types.SiteConfig
	err        error
}

type vanClientMockInjectedReturnValues struct {
	serviceInterfaceUnbind          error
	serviceInterfaceBind            error
	serviceInterfaceInspect         serviceInterfaceAndErrorReturns
	serviceInterfaceUpdate          error
	getHeadlessServiceConfiguration serviceInterfaceAndErrorReturns
	siteConfigInspect               siteConfigAndErrorReturns
	siteConfigCreate                siteConfigAndErrorReturns
	routerCreate                    error
}

type vanClientMock struct {
	serviceInterfaceUnbindCalledWith          []serviceInterfaceUnbindCallArgs
	serviceInterfaceBindCalledWith            []serviceInterfaceBindCallArgs
	serviceInterfaceInspectCalledWith         []string
	getHeadlessServiceConfigurationCalledWith []getHeadlessServiceConfigurationCallArgs
	serviceInterfaceUpdateCalledWith          []*types.ServiceInterface
	siteConfigInspectCalledWith               []*corev1.ConfigMap
	routerCreateCalledWith                    []types.SiteConfig
	siteConfigCreateCalledWith                []types.SiteConfigSpec
	injectedReturns                           vanClientMockInjectedReturnValues
}

func (v *vanClientMock) ResetCallHistory() {
	v.serviceInterfaceBindCalledWith = nil
	v.serviceInterfaceUnbindCalledWith = nil
	v.serviceInterfaceInspectCalledWith = nil
	v.getHeadlessServiceConfigurationCalledWith = nil
	v.serviceInterfaceUpdateCalledWith = nil
}

func (v *vanClientMock) RouterCreate(ctx context.Context, options types.SiteConfig) error {
	v.routerCreateCalledWith = append(v.routerCreateCalledWith, options)
	return v.injectedReturns.routerCreate
}
func (v *vanClientMock) RouterInspect(ctx context.Context) (*types.RouterInspectResponse, error) {
	return nil, nil
}
func (v *vanClientMock) RouterRemove(ctx context.Context) error {
	return nil
}
func (v *vanClientMock) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorCreateSecretFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	return nil, nil
}

func (v *vanClientMock) ConnectorCreate(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions) error {
	return nil
}

func (v *vanClientMock) ConnectorInspect(ctx context.Context, name string) (*types.ConnectorInspectResponse, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorList(ctx context.Context) ([]*types.Connector, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	return nil
}
func (v *vanClientMock) ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	return nil, false, nil
}
func (v *vanClientMock) ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error {
	return nil
}
func (v *vanClientMock) ServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface) error {
	return nil
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

func (v *vanClientMock) SiteConfigCreate(ctx context.Context, spec types.SiteConfigSpec) (*types.SiteConfig, error) {
	v.siteConfigCreateCalledWith = append(v.siteConfigCreateCalledWith, spec)
	return v.injectedReturns.siteConfigCreate.siteConfig, v.injectedReturns.siteConfigCreate.err
}

func (v *vanClientMock) SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*types.SiteConfig, error) {
	v.siteConfigInspectCalledWith = append(v.siteConfigInspectCalledWith, input)
	return v.injectedReturns.siteConfigInspect.siteConfig, v.injectedReturns.siteConfigInspect.err
}

func (v *vanClientMock) SiteConfigRemove(ctx context.Context) error {
	return nil
}

func (v *vanClientMock) SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) error {
	return nil
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

func (v *vanClientMock) ServiceInterfaceList(ctx context.Context) ([]*types.ServiceInterface, error) {
	//return []*ServiceInterface{}, nil
	return nil, nil
}

func (v *vanClientMock) ServiceInterfaceRemove(ctx context.Context, address string) error {
	return nil
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

func TestCmdUnexposeRun(t *testing.T) {
	cmd := NewCmdUnexpose(nil)
	test := func(targetType, targetName, address string) {

		cli := cli.(*vanClientMock)

		unexposeAddress = address

		args := []string{targetType}

		//supporting "targetType TargetName" and "targetType/targetName" notations
		if targetName != "" {
			args = append(args, targetName)
		} else {
			parts := strings.Split(targetType, "/")
			targetType = parts[0]
			targetName = parts[1]
		}

		err := cmd.RunE(&cobra.Command{}, args)

		if cli.injectedReturns.serviceInterfaceUnbind != nil {
			assert.Error(t, err, "Unable to unbind skupper service: "+cli.injectedReturns.serviceInterfaceUnbind.Error())
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
		cli = &vanClientMock{}
		test(targetType, targetName, address)
	}

	testError := func(targetType, targetName, address string, errorString string) {
		cli = &vanClientMock{
			injectedReturns: vanClientMockInjectedReturnValues{
				serviceInterfaceUnbind: fmt.Errorf("%s", errorString),
			},
		}
		test(targetType, targetName, address)
	}

	testSuccess("depl", "Name", "theService:8080")
	testSuccess("depl/Name", "", "theService:8080")

	testError("depl", "Name", "theService:8080", "some error")
	testError("depl/Name", "", "theService:8080", "other error")
}

func TestCmdInit(t *testing.T) {
	cmd := NewCmdInit(nil)
	var lcli (*vanClientMock)
	args := []string{}
	resetCli := func() {
		cli = &vanClientMock{}
		lcli = cli.(*vanClientMock)
	}

	t.Run("SiteConfigInspectReturnsError",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigInspect.err = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "some error")
			assert.Assert(t, lcli.siteConfigInspectCalledWith[0] == nil)
		})

	t.Run("SiteConfigInspectReturns nil, and SiteConfigCreateFails",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigCreate.err = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "some error")
			assert.Assert(t, lcli.siteConfigCreateCalledWith[0] == routerCreateOpts)
		})

	t.Run("routerCreateFails",
		func(t *testing.T) {
			resetCli()

			siteConfig := types.SiteConfig{
				Spec: types.SiteConfigSpec{
					SkupperName: "TheName",
				},
			}
			lcli.injectedReturns.siteConfigInspect.siteConfig = &siteConfig
			lcli.injectedReturns.routerCreate = fmt.Errorf("a error")
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "a error")
			assert.Assert(t, cmp.Equal(lcli.routerCreateCalledWith[0], siteConfig))
		})

	t.Run("routerCreateSucceeds",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigInspect.siteConfig = &types.SiteConfig{}
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Assert(t, err)
			assert.Assert(t, len(lcli.siteConfigInspectCalledWith) == 1)
			assert.Assert(t, len(lcli.siteConfigCreateCalledWith) == 0)
			assert.Assert(t, len(lcli.routerCreateCalledWith) == 1)
		})
}

func TestExpose_NotBinding(t *testing.T) {
	var err error
	ctx := context.Background()
	options := ExposeOptions{
		Protocol:   "",
		Address:    "",
		Port:       0,
		TargetPort: 0,
		Headless:   false,
	}

	t.Run("ServiceInterfaceInspect returns error",
		func(t *testing.T) {
			options.Address = "ServiceName"
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
			err := expose(cli, ctx, "deployment", "name", options)
			assert.Error(t, err, "some error")
			assert.Equal(t, cli.serviceInterfaceInspectCalledWith[0], "ServiceName")
		})

	t.Run("service not existent, headless option set, and targetType != statefulset ",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = nil
			cli.injectedReturns.serviceInterfaceInspect.err = nil

			options.Headless = true

			err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "The headless option is only supported for statefulsets")
		})

	t.Run("service not existent, headless option set, and targetType == statefulset ",
		func(t *testing.T) {
			cli := &vanClientMock{}
			aService := &types.ServiceInterface{}
			cli.injectedReturns.getHeadlessServiceConfiguration.serviceInterface = aService

			options.Protocol = "theprotocol"
			options.Port = 123

			err = expose(cli, ctx, "statefulset", "name", options)
			assert.Assert(t, err)

			assert.Equal(t, len(cli.getHeadlessServiceConfigurationCalledWith), 1)
			assert.Equal(t, len(cli.serviceInterfaceUpdateCalledWith), 1)

			expectedGetHead := getHeadlessServiceConfigurationCallArgs{
				targetName: "name",
				protocol:   options.Protocol,
				address:    "ServiceName",
				port:       options.Port,
			}

			assert.Assert(t, cmp.Equal(cli.getHeadlessServiceConfigurationCalledWith[0], expectedGetHead, cmp.AllowUnexported(getHeadlessServiceConfigurationCallArgs{})))
			assert.Assert(t, cli.serviceInterfaceUpdateCalledWith[0] == aService)
		})

	t.Run("serviceInterfaceInspect returns an existent service and options are wrong",
		func(t *testing.T) {

			cli := &vanClientMock{}
			test_protocol := "protocol"
			options.Headless = true
			options.Protocol = test_protocol + "diff"
			injectedService := &types.ServiceInterface{
				Protocol: test_protocol,
				Headless: &types.Headless{
					Name: "NotNil",
				},
			}
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService

			err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "Service already exposed as headless")

			injectedService.Headless = nil
			err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "Service already exposed, cannot reconfigure as headless")

			options.Headless = false
			err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, fmt.Sprintf("Invalid protocol %s for service with mapping %s", options.Protocol, injectedService.Protocol))
		})

}

func TestExpose_Binding(t *testing.T) {
	ctx := context.Background()

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
	options := ExposeOptions{}

	test_protocol := "protocol"
	options.Address = "TheService"
	options.Headless = false
	options.Protocol = test_protocol
	options.Port = 123
	options.TargetPort = 234

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
			cli := &vanClientMock{}

			err := expose(cli, ctx, "any", "name", options)
			assert.Assert(t, err)
			assert.Equal(t, len(cli.serviceInterfaceBindCalledWith), 1)
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)

		})

	t.Run("service exists and Bind is successfull",
		func(t *testing.T) {
			cli := &vanClientMock{}
			aService := &types.ServiceInterface{
				Address:  "TheOtherService",
				Port:     options.Port,
				Protocol: options.Protocol,
			}
			expectedBindCall := expectedBindCall
			expectedBindCall.service = aService
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = aService

			err := expose(cli, ctx, "any", "name", options)
			assert.Assert(t, err)

			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: any Error",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = fmt.Errorf("some error")
			err := expose(cli, ctx, "any", "name", options)
			assert.Error(t, err, "Unable to create skupper service: some error")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: isNotFound",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = errors.NewNotFound(schema.GroupResource{}, "name")
			err := expose(cli, ctx, "any", "name", options)
			assert.Error(t, err, "Skupper is not installed in Namespace: 'MockNamespace`")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})
}

func TestCmdExposeRun(t *testing.T) {
	cmd := NewCmdExpose(nil)
	cli = &vanClientMock{} //the global cli is used by the "RunE" func
	cli := cli.(*vanClientMock)

	args := []string{"service", "name"}
	exposeOpts.Address = ""

	err := cmd.RunE(&cobra.Command{}, args)
	assert.Error(t, err, "--address option is required for target type 'service'")

	//hack: forcing a expose function call error
	args = []string{"pods", "name"}
	cli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
	err = cmd.RunE(&cobra.Command{}, args)
	assert.Error(t, err, "some error")
}

func TestCmdBind(t *testing.T) {
	cmd := NewCmdBind(nil)
	var lcli *vanClientMock
	args := []string{}
	resetCli := func() {
		cli = &vanClientMock{}
		lcli = cli.(*vanClientMock)
	}

	t.Run("invalidProtocol",
		func(t *testing.T) {
			resetCli()
			protocol = "invalidProtocol"
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "invalidProtocol is not a valid protocol. Choose 'tcp', 'http' or 'http2'.")
		})

	t.Run("serviceNotFound",
		func(t *testing.T) {
			resetCli()
			protocol = "tcp"
			args = []string{"TheService", "type", "name"}
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "Service TheService not found")
		})

	t.Run("ServiceInterfaceInspect_fails",
		func(t *testing.T) {
			resetCli()
			protocol = "tcp"
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "some error")
		})

	injectedService := &types.ServiceInterface{
		Protocol: "tcp",
		Headless: &types.Headless{
			Name: "NotNil",
		},
	}

	t.Run("Success",
		func(t *testing.T) {
			resetCli()
			protocol = "tcp"
			targetPort = 567
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Assert(t, err)
			assert.Assert(t, len(lcli.serviceInterfaceBindCalledWith) == 1)
			c := lcli.serviceInterfaceBindCalledWith[0]
			assert.Assert(t, c.protocol == "tcp")
			assert.Assert(t, c.targetType == "type")
			assert.Assert(t, c.targetName == "name")
			assert.Assert(t, c.targetPort == 567)
			assert.Assert(t, c.service == injectedService)

		})

	t.Run("ServiceInterfaceBindFails",
		func(t *testing.T) {
			resetCli()
			protocol = "tcp"
			targetPort = 567
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService
			lcli.injectedReturns.serviceInterfaceBind = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)

			assert.Error(t, err, "some error")
		})
}
