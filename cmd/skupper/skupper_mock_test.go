package main

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type serviceInterfaceUnbindCallArgs struct {
	targetType, targetName, address, namespace string
	deleteIfNoTargets                          bool
}

type serviceInterfaceBindCallArgs struct {
	service     *types.ServiceInterface
	targetType  string
	targetName  string
	protocol    string
	targetPorts map[int]int
	namespace   string
}

type getHeadlessServiceConfigurationCallArgs struct {
	targetName string
	protocol   string
	address    string
	ports      []int
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

func (v *vanClientMock) GetIngressDefault() string {
	return types.IngressRouteString
}

func (v *vanClientMock) RouterCreate(ctx context.Context, options types.SiteConfig) error {
	v.routerCreateCalledWith = append(v.routerCreateCalledWith, options)
	return v.injectedReturns.routerCreate
}
func (v *vanClientMock) RouterInspect(ctx context.Context) (*types.RouterInspectResponse, error) {
	return nil, nil
}
func (v *vanClientMock) RouterInspectNamespace(ctx context.Context, namespace string) (*types.RouterInspectResponse, error) {
	return nil, nil
}
func (v *vanClientMock) RouterRemove(ctx context.Context) error {
	return nil
}
func (v *vanClientMock) RouterUpdateVersion(ctx context.Context, hup bool) (bool, error) {
	return true, nil
}
func (v *vanClientMock) RouterUpdateVersionInNamespace(ctx context.Context, hup bool, namespace string) (bool, error) {
	return true, nil
}
func (v *vanClientMock) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorCreateSecretFromData(ctx context.Context, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	return nil, nil
}

func (v *vanClientMock) ConnectorCreate(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions) error {
	return nil
}

func (v *vanClientMock) ConnectorInspect(ctx context.Context, name string) (*types.LinkStatus, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorList(ctx context.Context) ([]types.LinkStatus, error) {
	return nil, nil
}
func (v *vanClientMock) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	return nil
}
func (v *vanClientMock) ConnectorTokenCreateFromTemplate(ctx context.Context, tokenName string, templateName string) (*corev1.Secret, bool, error) {
	return nil, false, nil
}
func (v *vanClientMock) ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	return nil, false, nil
}
func (v *vanClientMock) ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error {
	return nil
}
func (v *vanClientMock) TokenClaimCreate(ctx context.Context, name string, password []byte, expiry time.Duration, uses int) (*corev1.Secret, bool, error) {
	return nil, true, nil
}
func (v *vanClientMock) TokenClaimCreateFile(ctx context.Context, subject string, password []byte, expiry time.Duration, uses int, secretFile string) error {
	return nil
}
func (v *vanClientMock) ServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface) error {
	return nil
}

func (v *vanClientMock) ServiceInterfaceUnbind(ctx context.Context, targetType string, targetName string, address string, deleteIfNoTargets bool, namespace string) error {
	var calledWith = serviceInterfaceUnbindCallArgs{
		targetType:        targetType,
		targetName:        targetName,
		address:           address,
		deleteIfNoTargets: deleteIfNoTargets,
		namespace:         namespace,
	}
	v.serviceInterfaceUnbindCalledWith = append(v.serviceInterfaceUnbindCalledWith, calledWith)

	return v.injectedReturns.serviceInterfaceUnbind
}

func (v *vanClientMock) GatewayBind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return nil
}

func (v *vanClientMock) GatewayUnbind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return nil
}

func (v *vanClientMock) GatewayExpose(ctx context.Context, gatewayName string, gatewayType string, endpoint types.GatewayEndpoint) (string, error) {
	return "", nil
}

func (v *vanClientMock) GatewayUnexpose(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint, deleteLast bool) error {
	return nil
}

func (v *vanClientMock) GatewayForward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return nil
}

func (v *vanClientMock) GatewayUnforward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return nil
}

func (v *vanClientMock) GatewayInit(ctx context.Context, gatewayName string, gatewayType string, configFile string) (string, error) {
	return "", nil
}

func (v *vanClientMock) GatewayDownload(ctx context.Context, gatewayName string, downloadPath string) (string, error) {
	return "", nil
}

func (v *vanClientMock) GatewayExportConfig(ctx context.Context, targetGatewayName string, exportGatewayName string, exportPath string) (string, error) {
	return "", nil
}

func (cli *vanClientMock) GatewayGenerateBundle(ctx context.Context, configFile string, bundlePath string) (string, error) {
	return "", nil
}

func (v *vanClientMock) GatewayInspect(ctx context.Context, gatewayName string) (*types.GatewayInspectResponse, error) {
	return &types.GatewayInspectResponse{}, nil
}

func (v *vanClientMock) GatewayList(ctx context.Context) ([]*types.GatewayInspectResponse, error) {
	return nil, nil
}

func (v *vanClientMock) GatewayRemove(ctx context.Context, gatewayName string) error {
	return nil
}

func (v *vanClientMock) SiteConfigCreate(ctx context.Context, spec types.SiteConfigSpec) (*types.SiteConfig, error) {
	v.siteConfigCreateCalledWith = append(v.siteConfigCreateCalledWith, spec)
	return v.injectedReturns.siteConfigCreate.siteConfig, v.injectedReturns.siteConfigCreate.err
}

func (v *vanClientMock) SiteConfigUpdate(ctx context.Context, spec types.SiteConfigSpec) ([]string, error) {
	return nil, nil
}

func (v *vanClientMock) SiteConfigInspect(ctx context.Context, input *corev1.ConfigMap) (*types.SiteConfig, error) {
	v.siteConfigInspectCalledWith = append(v.siteConfigInspectCalledWith, input)
	return v.injectedReturns.siteConfigInspect.siteConfig, v.injectedReturns.siteConfigInspect.err
}

func (v *vanClientMock) SiteConfigRemove(ctx context.Context) error {
	return nil
}

func (v *vanClientMock) SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) (string, error) {
	return "", nil
}

func (v *vanClientMock) SkupperEvents(verbose bool) (*bytes.Buffer, error) {
	return nil, nil
}

func (v *vanClientMock) SkupperCheckService(service string, verbose bool) (*bytes.Buffer, error) {
	return nil, nil
}

func (v *vanClientMock) ServiceInterfaceBind(ctx context.Context, service *types.ServiceInterface, targetType string, targetName string, targetPorts map[int]int, namespace string) error {
	var calledWith = serviceInterfaceBindCallArgs{
		service:     service,
		targetType:  targetType,
		targetName:  targetName,
		targetPorts: targetPorts,
		namespace:   namespace,
	}
	v.serviceInterfaceBindCalledWith = append(v.serviceInterfaceBindCalledWith, calledWith)

	return v.injectedReturns.serviceInterfaceBind
}

func (v *vanClientMock) ServiceInterfaceInspect(ctx context.Context, address string) (*types.ServiceInterface, error) {
	v.serviceInterfaceInspectCalledWith = append(v.serviceInterfaceInspectCalledWith, address)
	return v.injectedReturns.serviceInterfaceInspect.serviceInterface, v.injectedReturns.serviceInterfaceInspect.err
}

func (v *vanClientMock) ServiceInterfaceList(ctx context.Context) ([]*types.ServiceInterface, error) {
	// return []*ServiceInterface{}, nil
	return nil, nil
}

func (v *vanClientMock) ServiceInterfaceRemove(ctx context.Context, address string) error {
	return nil
}

func (v *vanClientMock) ServiceInterfaceUpdate(ctx context.Context, service *types.ServiceInterface) error {
	v.serviceInterfaceUpdateCalledWith = append(v.serviceInterfaceUpdateCalledWith, service)
	return v.injectedReturns.serviceInterfaceUpdate
}

func (v *vanClientMock) GetHeadlessServiceConfiguration(targetName string, protocol string, address string, ports []int, publishNotReadyAddresses bool, namespace string) (*types.ServiceInterface, error) {
	var calledWith = getHeadlessServiceConfigurationCallArgs{
		targetName: targetName,
		protocol:   protocol,
		address:    address,
		ports:      ports,
	}
	v.getHeadlessServiceConfigurationCalledWith = append(v.getHeadlessServiceConfigurationCalledWith, calledWith)
	return v.injectedReturns.getHeadlessServiceConfiguration.serviceInterface, v.injectedReturns.getHeadlessServiceConfiguration.err
}

func (cli *vanClientMock) GetNamespace() string {
	return "MockNamespace"
}

func (cli *vanClientMock) GetVersion(component string, name string) string {
	return "not-found"
}

func (v *vanClientMock) RevokeAccess(ctx context.Context) error {
	return nil
}

func (v *vanClientMock) NetworkStatus(ctx context.Context) ([]*types.SiteInfo, error) {
	return nil, nil
}

func (v *vanClientMock) GetRemoteLinks(ctx context.Context, siteConfig *types.SiteConfig) ([]*types.RemoteLinkInfo, error) {
	return nil, nil
}

func TestCmdUnexposeRun(t *testing.T) {
	skupperClient := NewSkupperTestClient()
	cmd := NewCmdUnexpose(skupperClient.Service())
	test := func(targetType, targetName, address string) {
		cli := skupperClient.Cli.(*vanClientMock)
		unexposeAddress = address

		args := []string{targetType}

		// supporting "targetType TargetName" and "targetType/targetName" notations
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
		skupperClient.Cli = &vanClientMock{}
		test(targetType, targetName, address)
	}

	testError := func(targetType, targetName, address string, errorString string) {
		skupperClient.Cli = &vanClientMock{
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
	skupperCli := NewSkupperTestClient()
	skupperCli.Cli = &vanClientMock{}
	cmd := NewCmdInit(skupperCli.Site())
	var lcli *vanClientMock
	args := []string{}
	resetCli := func() {
		lcli = &vanClientMock{}
		skupperCli.Cli = lcli
	}

	t.Run("SiteConfigInspectReturnsError",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigInspect.err = fmt.Errorf("some error")
			err := cmd.RunE(cmd, args)
			assert.Error(t, err, "some error")
			assert.Assert(t, lcli.siteConfigInspectCalledWith[0] == nil)
		})

	t.Run("SiteConfigInspectReturns nil, and SiteConfigCreateFails",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigCreate.err = fmt.Errorf("some error")
			err := cmd.RunE(cmd, args)
			assert.Error(t, err, "some error")
			assert.Assert(t, reflect.DeepEqual(lcli.siteConfigCreateCalledWith[0], routerCreateOpts))
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
			err := cmd.RunE(cmd, args)
			assert.Error(t, err, "a error")
			assert.Assert(t, cmp.Equal(lcli.routerCreateCalledWith[0], siteConfig))
		})

	t.Run("routerCreateSucceeds",
		func(t *testing.T) {
			resetCli()
			lcli.injectedReturns.siteConfigInspect.siteConfig = &types.SiteConfig{}
			err := cmd.RunE(cmd, args)
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
		Protocol:    "",
		Address:     "",
		Ports:       []int{},
		TargetPorts: []string{},
		Headless:    false,
	}

	t.Run("ServiceInterfaceInspect returns error",
		func(t *testing.T) {
			options.Address = "ServiceName"
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
			_, err := expose(cli, ctx, "deployment", "name", options)
			assert.Error(t, err, "some error")
			assert.Equal(t, cli.serviceInterfaceInspectCalledWith[0], "ServiceName")
		})

	t.Run("service not existent, headless option set, and targetType != statefulset ",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = nil
			cli.injectedReturns.serviceInterfaceInspect.err = nil

			options.Headless = true

			_, err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "The headless option is only supported for statefulsets")
		})

	t.Run("service not existent, headless option set, and targetType == statefulset ",
		func(t *testing.T) {
			cli := &vanClientMock{}
			aService := &types.ServiceInterface{}
			cli.injectedReturns.getHeadlessServiceConfiguration.serviceInterface = aService

			options.Protocol = "theprotocol"
			options.Ports = []int{123}

			_, err = expose(cli, ctx, "statefulset", "name", options)
			assert.Assert(t, err)

			assert.Equal(t, len(cli.getHeadlessServiceConfigurationCalledWith), 1)
			assert.Equal(t, len(cli.serviceInterfaceUpdateCalledWith), 1)

			expectedGetHead := getHeadlessServiceConfigurationCallArgs{
				targetName: "name",
				protocol:   options.Protocol,
				address:    "ServiceName",
				ports:      options.Ports,
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

			_, err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "Service already exposed as headless")

			injectedService.Headless = nil
			_, err = expose(cli, ctx, "service", "name", options)
			assert.Error(t, err, "Service already exposed, cannot reconfigure as headless")

			options.Headless = false
			_, err = expose(cli, ctx, "service", "name", options)
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
		assert.Assert(t, reflect.DeepEqual(a.targetPorts, b.targetPorts))
		assert.Assert(t, a.service.Address == b.service.Address)
		assert.Assert(t, a.service.Protocol == b.service.Protocol)
		assert.Assert(t, reflect.DeepEqual(a.service.Ports, b.service.Ports))
	}
	options := ExposeOptions{}

	test_protocol := "protocol"
	options.Address = "TheService"
	options.Headless = false
	options.Protocol = test_protocol
	options.Ports = []int{123}
	options.TargetPorts = []string{"123:234"}

	expectedBindCall := serviceInterfaceBindCallArgs{
		service: &types.ServiceInterface{
			Address:  "TheService",
			Protocol: test_protocol,
			Ports:    []int{123},
		},
		targetType:  "any",
		targetName:  "name",
		targetPorts: map[int]int{123: 234},
	}

	t.Run("service not existent and options.expose.headless == false",
		func(t *testing.T) {
			cli := &vanClientMock{}

			fmt.Println("TARGET PORTS =", bindOptions.TargetPorts)
			fmt.Println("OPTIONS =", options)
			exposedAs, err := expose(cli, ctx, "any", "name", options)
			assert.Assert(t, err)
			assert.Equal(t, exposedAs, "TheService")
			assert.Equal(t, len(cli.serviceInterfaceBindCalledWith), 1)
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)

		})

	t.Run("service exists and Bind is successfull",
		func(t *testing.T) {
			cli := &vanClientMock{}
			aService := &types.ServiceInterface{
				Address:  "TheOtherService",
				Ports:    options.Ports,
				Protocol: options.Protocol,
			}
			expectedBindCall := expectedBindCall
			expectedBindCall.service = aService
			cli.injectedReturns.serviceInterfaceInspect.serviceInterface = aService

			exposedAs, err := expose(cli, ctx, "any", "name", options)
			assert.Assert(t, err)
			assert.Equal(t, exposedAs, "TheService")

			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: any Error",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = fmt.Errorf("some error")
			_, err := expose(cli, ctx, "any", "name", options)
			assert.Error(t, err, "Unable to create skupper service: some error")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})

	t.Run("Bind fails: isNotFound",
		func(t *testing.T) {
			cli := &vanClientMock{}
			cli.injectedReturns.serviceInterfaceBind = errors.NewNotFound(schema.GroupResource{}, "name")
			_, err := expose(cli, ctx, "any", "name", options)
			assert.Error(t, err, "Skupper is not installed in Namespace: 'MockNamespace`")
			compare(&cli.serviceInterfaceBindCalledWith[0], &expectedBindCall)
		})
}

func TestCmdExposeRun(t *testing.T) {
	skupperCli := NewSkupperTestClient()
	cmd := NewCmdExpose(skupperCli.Service())
	cli := &vanClientMock{} // the global cli is used by the "RunE" func
	skupperCli.Cli = cli

	args := []string{"service", "name"}
	exposeOpts.Address = ""

	err := cmd.RunE(&cobra.Command{}, args)
	assert.Error(t, err, "--address option is required for target type 'service'")

	// hack: forcing a expose function call error
	args = []string{"pods", "name"}
	cli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
	err = cmd.RunE(&cobra.Command{}, args)
	assert.Error(t, err, "some error")
	assert.Assert(t, exposeOpts.Address == "name")
}

func TestCmdBind(t *testing.T) {
	skupperCli := NewSkupperTestClient()
	var lcli *vanClientMock

	cmd := NewCmdBind(skupperCli.Service())
	args := []string{}
	resetCli := func() {
		lcli = &vanClientMock{}
		skupperCli.Cli = lcli
	}

	t.Run("serviceNotFound",
		func(t *testing.T) {
			resetCli()
			args = []string{"TheService", "type", "name"}
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "Service TheService not found")
		})

	t.Run("ServiceInterfaceInspect_fails",
		func(t *testing.T) {
			resetCli()
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.err = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Error(t, err, "some error")
		})

	injectedService := &types.ServiceInterface{
		Protocol: "tcp",
		Ports:    []int{567},
		Headless: &types.Headless{
			Name: "NotNil",
		},
	}

	t.Run("Success",
		func(t *testing.T) {
			resetCli()
			targetPorts = []string{"567:567"}
			expectedTargetPorts := map[int]int{567: 567}
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService
			err := cmd.RunE(&cobra.Command{}, args)
			assert.Assert(t, err)
			assert.Assert(t, len(lcli.serviceInterfaceBindCalledWith) == 1)
			c := lcli.serviceInterfaceBindCalledWith[0]
			assert.Assert(t, c.targetType == "type")
			assert.Assert(t, c.targetName == "name")
			assert.Assert(t, reflect.DeepEqual(c.targetPorts, expectedTargetPorts))
			assert.Assert(t, c.service == injectedService)

		})

	t.Run("ServiceInterfaceBindFails",
		func(t *testing.T) {
			resetCli()
			targetPorts = []string{"567"}
			args = []string{"TheService", "type", "name"}
			lcli.injectedReturns.serviceInterfaceInspect.serviceInterface = injectedService
			lcli.injectedReturns.serviceInterfaceBind = fmt.Errorf("some error")
			err := cmd.RunE(&cobra.Command{}, args)

			assert.Error(t, err, "some error")
		})
}
