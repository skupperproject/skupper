package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/skupperproject/skupper/pkg/version"
	"github.com/spf13/cobra/doc"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SkupperClientManager interface {
	Create(cmd *cobra.Command, args []string) error
	CreateFlags(cmd *cobra.Command)
	Delete(cmd *cobra.Command, args []string) error
	DeleteFlags(cmd *cobra.Command)
	List(cmd *cobra.Command, args []string) error
	ListFlags(cmd *cobra.Command)
	Status(cmd *cobra.Command, args []string) error
	StatusFlags(cmd *cobra.Command)
	SkupperClientCommon
}

type SkupperSiteClient interface {
	SkupperClientManager
	Update(cmd *cobra.Command, args []string) error
	UpdateFlags(cmd *cobra.Command)
	Version(cmd *cobra.Command, args []string) error
	RevokeAccess(cmd *cobra.Command, args []string) error
}

type SkupperServiceClient interface {
	SkupperClientManager
	Label(cmd *cobra.Command, args []string) error
	Bind(cmd *cobra.Command, args []string) error
	BindArgs(cmd *cobra.Command, args []string) error
	BindFlags(cmd *cobra.Command)
	Unbind(cmd *cobra.Command, args []string) error
	Expose(cmd *cobra.Command, args []string) error
	ExposeArgs(cmd *cobra.Command, args []string) error
	ExposeFlags(cmd *cobra.Command)
	Unexpose(cmd *cobra.Command, args []string) error
	UnexposeFlags(cmd *cobra.Command) error
}

type SkupperDebugClient interface {
	Dump(cmd *cobra.Command, args []string) error
	Events(cmd *cobra.Command, args []string) error
	Service(cmd *cobra.Command, args []string) error
	SkupperClientCommon
}

type SkupperLinkClient interface {
	SkupperClientManager
	LinkHandler() domain.LinkHandler
}

type SkupperTokenClient interface {
	Create(cmd *cobra.Command, args []string) error
	CreateFlags(cmd *cobra.Command)
	SkupperClientCommon
}

type SkupperNetworkClient interface {
	Status(cmd *cobra.Command, args []string) error
	StatusFlags(cmd *cobra.Command)
	SkupperClientCommon
}

type SkupperClientCommon interface {
	NewClient(cmd *cobra.Command, args []string)
	Platform() types.Platform
}

type SkupperClient interface {
	Options(cmd *cobra.Command)
	SupportedCommands() []string
	Site() SkupperSiteClient
	Service() SkupperServiceClient
	Debug() SkupperDebugClient
	Link() SkupperLinkClient
	Token() SkupperTokenClient
	Network() SkupperNetworkClient
}

type ExposeOptions struct {
	Protocol                 string
	Address                  string
	Ports                    []int
	TargetPorts              []string
	Headless                 bool
	ProxyTuning              types.Tuning
	GeneratedCerts           bool
	TlsCredentials           string
	TlsCertAuthority         string
	PublishNotReadyAddresses bool
	IngressMode              string
	BridgeImage              string
	Aggregate                string
	EventChannel             bool
	Namespace                string
}

type BindOptions struct {
	TargetPorts              []string
	PublishNotReadyAddresses bool
	tlsCertAuthority         string
	Namespace                string
}

func SkupperNotInstalledError(namespace string) error {
	return fmt.Errorf("Skupper is not installed in Namespace: '" + namespace + "`")

}

func parseTargetTypeAndName(args []string) (string, string) {
	// this functions assumes it is called with the right arguments, wrong
	// argument verification is done on the "Args:" functions
	targetType := args[0]
	var targetName string
	if len(args) == 2 {
		targetName = args[1]
	} else {
		parts := strings.Split(args[0], "/")
		targetType = parts[0]
		targetName = parts[1]
	}
	return targetType, targetName
}

func configureHeadlessProxy(spec *types.Headless, options *types.Tuning) error {
	var err error
	if options.Affinity != "" {
		spec.Affinity = utils.LabelToMap(options.Affinity)
	}
	if options.AntiAffinity != "" {
		spec.AntiAffinity = utils.LabelToMap(options.AntiAffinity)
	}
	if options.NodeSelector != "" {
		spec.NodeSelector = utils.LabelToMap(options.NodeSelector)
	}
	if options.Cpu != "" {
		cpuQuantity, err := resource.ParseQuantity(options.Cpu)
		if err == nil {
			spec.CpuRequest = &cpuQuantity
		} else {
			err = fmt.Errorf("Invalid value for cpu: %s", err)
		}
	}
	if options.Memory != "" {
		memoryQuantity, err := resource.ParseQuantity(options.Memory)
		if err == nil {
			spec.MemoryRequest = &memoryQuantity
		} else {
			err = fmt.Errorf("Invalid value for memory: %s", err)
		}
	}
	if options.CpuLimit != "" {
		cpuQuantity, err := resource.ParseQuantity(options.CpuLimit)
		if err == nil {
			spec.CpuLimit = &cpuQuantity
		} else {
			err = fmt.Errorf("Invalid value for cpu: %s", err)
		}
	}
	if options.MemoryLimit != "" {
		memoryQuantity, err := resource.ParseQuantity(options.MemoryLimit)
		if err == nil {
			spec.MemoryLimit = &memoryQuantity
		} else {
			err = fmt.Errorf("Invalid value for memory: %s", err)
		}
	}
	return err
}

func expose(cli types.VanClientInterface, ctx context.Context, targetType string, targetName string, options ExposeOptions) (string, error) {
	serviceName := options.Address

	service, err := cli.ServiceInterfaceInspect(ctx, serviceName)
	if err != nil {
		return "", err
	}

	vanClient, realClient := cli.(*client.VanClient)
	var policy *client.PolicyAPIClient
	if realClient {
		policy = client.NewPolicyValidatorAPI(vanClient)
		res, err := policy.Expose(targetType, targetName)
		if err != nil {
			return "", err
		}
		if !res.Allowed {
			return "", res.Err()
		}
	}
	if service == nil {
		if options.Headless {
			if targetType != "statefulset" {
				return "", fmt.Errorf("The headless option is only supported for statefulsets")
			}
			service, err = cli.GetHeadlessServiceConfiguration(targetName, options.Protocol, options.Address, options.Ports, options.PublishNotReadyAddresses, options.Namespace)
			if err != nil {
				return "", err
			}
			err = configureHeadlessProxy(service.Headless, &options.ProxyTuning)
			return service.Address, cli.ServiceInterfaceUpdate(ctx, service)
		} else {
			if realClient {
				res, err := policy.Service(serviceName)
				if err != nil {
					return "", err
				}
				if !res.Allowed {
					return "", res.Err()
				}
			}
			service = &types.ServiceInterface{
				Address:                  serviceName,
				Ports:                    options.Ports,
				Protocol:                 options.Protocol,
				TlsCredentials:           options.TlsCredentials,
				TlsCertAuthority:         options.TlsCertAuthority,
				PublishNotReadyAddresses: options.PublishNotReadyAddresses,
				BridgeImage:              options.BridgeImage,
				Namespace:                options.Namespace,
			}
			err := service.SetIngressMode(options.IngressMode)
			if err != nil {
				return "", err

			}
		}
	} else if service.Headless != nil {
		return "", fmt.Errorf("Service already exposed as headless")
	} else if options.Headless {
		return "", fmt.Errorf("Service already exposed, cannot reconfigure as headless")
	} else if options.Protocol != "" && service.Protocol != options.Protocol {
		return "", fmt.Errorf("Invalid protocol %s for service with mapping %s", options.Protocol, service.Protocol)
	} else if (options.TlsCredentials != "" || options.TlsCertAuthority != "") && options.Protocol == "http" {
	} else if options.BridgeImage != "" && service.BridgeImage != options.BridgeImage {
		return "", fmt.Errorf("Service %s already exists with a different bridge image: %s", serviceName, service.BridgeImage)
	} else if options.TlsCredentials != "" && service.TlsCredentials == "" {
		return "", fmt.Errorf("Service already exposed without TLS support")
	} else if options.TlsCertAuthority != "" && service.TlsCertAuthority == "" {
		return "", fmt.Errorf("Service already exposed without TLS support")
	} else if options.TlsCredentials == "" && service.TlsCredentials != "" {
		return "", fmt.Errorf("Service already exposed with TLS support")
	} else if options.TlsCertAuthority == "" && service.TlsCertAuthority != "" {
		return "", fmt.Errorf("Service already exposed with TLS support")
	} else if options.IngressMode != "" && options.IngressMode != string(service.ExposeIngress) {
		return "", fmt.Errorf("Service already exposed with different ingress mode")
	}

	// service may exist from remote origin
	service.Origin = ""

	targetPorts, err := parsePortMapping(service, options.TargetPorts)
	if err != nil {
		return "", err
	}

	err = cli.ServiceInterfaceBind(ctx, service, targetType, targetName, targetPorts, options.Namespace)
	if errors.IsNotFound(err) {
		return "", SkupperNotInstalledError(cli.GetNamespace())
	} else if err != nil {
		return "", fmt.Errorf("Unable to create skupper service: %w", err)
	}

	return options.Address, nil
}

func createServiceArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || (len(args) == 1 && !strings.Contains(args[0], ":")) {
		return fmt.Errorf("Name and port(s) must be specified")
	}
	if len(args) > 1 {
		for _, v := range args[1:] {
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("%s is not a valid port", v)
			}
		}
	}
	return nil
}

func exposeGatewayArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || (!strings.Contains(args[1], ":") && len(args) < 3) {
		return fmt.Errorf("Gateway service address, target host and port must all be specified")
	}
	if len(args) > 2 && strings.Contains(args[1], ":") {
		return fmt.Errorf("extra argument: %s", args[2])
	}
	if len(args) > 2 {
		for _, v := range args[2:] {
			ports := strings.Split(v, ":")
			if len(ports) > 2 {
				return fmt.Errorf("%s is not a valid port", v)
			} else if len(ports) == 2 {
				if _, err := strconv.Atoi(ports[1]); err != nil {
					return fmt.Errorf("%s is not a valid port", v)
				}
			}
			if _, err := strconv.Atoi(ports[0]); err != nil {
				return fmt.Errorf("%s is not a valid port", v)
			}
		}
	}
	return nil
}

func bindGatewayArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || (!strings.Contains(args[1], ":") && len(args) < 3) {
		return fmt.Errorf("Service address, target host and port must all be specified")
	}
	if len(args) > 2 && strings.Contains(args[1], ":") {
		return fmt.Errorf("extra argument: %s", args[2])
	}
	if len(args) > 2 {
		for _, v := range args[2:] {
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("%s is not a valid port", v)
			}
		}
	}
	return nil
}

func silenceCobra(cmd *cobra.Command) {
	cmd.SilenceUsage = true
}

func NewClient(namespace string, context string, kubeConfigPath string) types.VanClientInterface {
	return NewClientHandleError(namespace, context, kubeConfigPath, true)
}

func NewClientHandleError(namespace string, context string, kubeConfigPath string, exitOnError bool) types.VanClientInterface {

	var cli types.VanClientInterface
	var err error

	cli, err = client.NewClient(namespace, context, kubeConfigPath)
	if err != nil {
		if exitOnError {
			if strings.Contains(err.Error(), "invalid configuration: no configuration has been provided") {
				fmt.Printf("%s. Please point to an existing, valid kubeconfig file.\n", err.Error())
			} else {
				fmt.Println(err.Error())
			}
			os.Exit(1)
		} else {
			return nil
		}
	}
	return cli
}

var routerCreateOpts types.SiteConfigSpec
var routerLogging string

func asMap(entries []string) map[string]string {
	result := map[string]string{}
	for _, entry := range entries {
		parts := strings.Split(entry, "=")
		if len(parts) > 1 {
			result[parts[0]] = parts[1]
		} else {
			result[parts[0]] = ""
		}
	}
	return result
}

var LoadBalancerTimeout time.Duration

type InitFlags struct {
	routerMode string
	labels     []string
}

var initFlags InitFlags

func NewCmdInit(skupperCli SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long: `Setup a router and other supporting objects to provide a functional skupper
installation that can then be connected to other skupper installations`,
		Args:   cobra.NoArgs,
		PreRun: skupperCli.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			routerModeFlag := cmd.Flag("router-mode")

			if routerModeFlag.Changed {
				options := []string{string(types.TransportModeInterior), string(types.TransportModeEdge)}
				if !utils.StringSliceContains(options, initFlags.routerMode) {
					return fmt.Errorf(`invalid "--router-mode=%v", it must be one of "%v"`, initFlags.routerMode, strings.Join(options, ", "))
				}
				routerCreateOpts.RouterMode = initFlags.routerMode
			} else {
				routerCreateOpts.RouterMode = string(types.TransportModeInterior)
			}

			if routerLogging != "" {
				logConfig, err := qdr.ParseRouterLogConfig(routerLogging)
				if err != nil {
					return fmt.Errorf("Bad value for --router-logging: %s", err)
				}
				routerCreateOpts.Router.Logging = logConfig
			}
			if routerCreateOpts.Router.DebugMode != "" {
				if routerCreateOpts.Router.DebugMode != "asan" && routerCreateOpts.Router.DebugMode != "gdb" {
					return fmt.Errorf("Bad value for --router-debug-mode: %s (use 'asan' or 'gdb')", routerCreateOpts.Router.DebugMode)
				}
			}

			return skupperCli.Create(cmd, args)
		},
	}
	platform := skupperCli.Platform()
	routerCreateOpts.EnableController = true
	cmd.Flags().StringVarP(&routerCreateOpts.SkupperName, "site-name", "", "", "Provide a specific name for this skupper installation")
	cmd.Flags().StringVarP(&routerCreateOpts.Ingress, "ingress", "", "", "Setup Skupper ingress to one of: ["+strings.Join(types.ValidIngressOptions(platform), "|")+"].")
	cmd.Flags().StringVarP(&initFlags.routerMode, "router-mode", "", string(types.TransportModeInterior), "Skupper router-mode")

	cmd.Flags().StringSliceVar(&initFlags.labels, "labels", []string{}, "Labels to add to skupper pods")
	cmd.Flags().StringVarP(&routerLogging, "router-logging", "", "", "Logging settings for router. 'trace', 'debug', 'info' (default), 'notice', 'warning', and 'error' are valid values.")
	cmd.Flags().StringVarP(&routerCreateOpts.Router.DebugMode, "router-debug-mode", "", "", "Enable debug mode for router ('asan' or 'gdb' are valid values)")

	cmd.Flags().SortFlags = false

	// platform specific flags
	skupperCli.CreateFlags(cmd)

	return cmd
}

func hideFlag(cmd *cobra.Command, name string) {
	f := cmd.Flag(name)
	f.Hidden = true
}

func NewCmdDelete(skupperCli SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete",
		Short:  "Delete skupper installation",
		Long:   `delete will delete any skupper related objects from the namespace`,
		Args:   cobra.NoArgs,
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Delete,
	}
	return cmd
}

var forceHup bool

func NewCmdUpdate(skupperCli SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update skupper installation version",
		Long:   "Update the skupper site to " + version.Version,
		Args:   cobra.NoArgs,
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Update,
	}
	cmd.Flags().BoolVarP(&forceHup, "force-restart", "", false, "Restart skupper daemons even if image tag is not updated")
	return cmd
}

var clientIdentity string

func NewCmdStatus(skupperCli SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Report the status of the current Skupper site",
		Args:   cobra.NoArgs,
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Status,
	}
	return cmd
}

var exposeOpts ExposeOptions

func NewCmdExpose(skupperCli SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "expose",
		Short:  "Expose a set of pods through a Skupper address",
		Args:   skupperCli.ExposeArgs,
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Expose,
	}

	cmd.Flags().StringVar(&(exposeOpts.Protocol), "protocol", "tcp", "The protocol to proxy (tcp, http, or http2)")
	cmd.Flags().StringVar(&(exposeOpts.Address), "address", "", "The Skupper address to expose")
	cmd.Flags().IntSliceVar(&(exposeOpts.Ports), "port", []int{}, "The ports to expose on")
	cmd.Flags().StringSliceVar(&(exposeOpts.TargetPorts), "target-port", []string{}, "The ports to target on pods")
	cmd.Flags().StringVar(&(exposeOpts.IngressMode), "enable-ingress-from-target-site", "", "Determines whether access to the Skupper service is enabled in the site the target was exposed through. Always (default) or Never are valid values.")

	cmd.Flags().StringVar(&exposeOpts.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&exposeOpts.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")

	cmd.Flags().BoolVar(&exposeOpts.GeneratedCerts, "enable-tls", false, "If specified, the service will be exposed over TLS (valid only for http2 and tcp protocols)")
	cmd.Flags().BoolVar(&exposeOpts.GeneratedCerts, "generate-tls-secrets", false, "If specified, the service will be exposed over TLS (valid only for http2 and tcp protocols)")

	f := cmd.Flag("enable-tls")
	f.Deprecated = "use 'generate-tls-secrets' instead"
	f.Hidden = true

	skupperCli.ExposeFlags(cmd)
	return cmd
}

var unexposeAddress string
var unexposeNamespace string

func NewCmdUnexpose(skupperCli SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unexpose",
		Short:  "Unexpose a set of pods previously exposed through a Skupper address",
		Args:   skupperCli.ExposeArgs,
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Unexpose,
	}
	cmd.Flags().StringVar(&unexposeAddress, "address", "", "Skupper address the target was exposed as")
	cmd.Flags().StringVar(&unexposeNamespace, "target-namespace", "", "Target namespace from previously exposed service")
	skupperCli.UnexposeFlags(cmd)
	return cmd
}

var showLabels bool

func NewCmdServiceStatus(skupperClient SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "List services exposed over the service network",
		Args:   cobra.NoArgs,
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			return skupperClient.Status(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&showLabels, "show-labels", false, "show service labels")
	return cmd
}

var addLabels, removeLabels []string

func NewCmdServiceLabel(skupperClient SkupperServiceClient) *cobra.Command {

	labels := &cobra.Command{
		Use:   "label <service> [labels...]",
		Short: "Manage service labels",
		Example: `
        # show labels for my-service
        skupper service label my-service

        # add label1=value1 and label2=value2 to my-service
        skupper service label my-service label1=value1 label2=value2

        # add label1=value1 and remove label2 to/from my-service 
        skupper service label my-service label1=value1 label2-`,
		PreRun: skupperClient.NewClient,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("service name is required")
			} else if len(args) == 1 {
				showLabels = true
			}
			if len(args) > 1 {
				for i := 1; i < len(args); i++ {
					label := args[i]
					labelFields := len(strings.Split(label, "="))
					if labelFields == 2 {
						addLabels = append(addLabels, label)
					} else if labelFields == 1 {
						if !strings.HasSuffix(label, "-") {
							return fmt.Errorf("no value provided to label %s", label)
						}
						removeLabels = append(removeLabels, strings.TrimSuffix(label, "-"))
					} else {
						return fmt.Errorf("invalid label [%s], use key=value or key- (to remove)", label)
					}
				}
			}
			return nil
		},
		RunE: skupperClient.Label,
	}

	return labels
}

func updateServiceLabels(si *types.ServiceInterface) {
	curLabels := si.Labels
	if curLabels == nil {
		curLabels = map[string]string{}
	}
	// removing labels
	for _, rmlabel := range removeLabels {
		delete(curLabels, rmlabel)
	}
	// adding labels
	for _, addLabels := range addLabels {
		for k, v := range utils.LabelToMap(addLabels) {
			curLabels[k] = v
		}
	}
	si.Labels = curLabels
}

func showServiceLabels(si *types.ServiceInterface, name string) {
	if si.Labels != nil && len(si.Labels) > 0 {
		l := formatter.NewList()
		l.Item(name)
		labels := l.NewChild("Labels:")
		for k, v := range si.Labels {
			labels.NewChild(fmt.Sprintf("%s=%s", k, v))
		}
		l.Print()
	} else {
		fmt.Printf("%s has no labels", name)
		fmt.Println()
	}
}

func NewCmdService() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service create <name> <port> or service delete port",
		Short: "Manage skupper service definitions",
	}
	return cmd
}

var serviceToCreate types.ServiceInterface
var serviceIngressMode string
var createSvcWithGeneratedTlsCerts bool

func NewCmdCreateService(skupperClient SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "create <name> <port...>",
		Short:  "Create a skupper service",
		Args:   createServiceArgs,
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			var sPorts []string

			if len(args) == 1 {
				parts := strings.Split(args[0], ":")
				serviceToCreate.Address = parts[0]
				sPorts = []string{parts[1]}
			} else {
				serviceToCreate.Address = args[0]
				sPorts = args[1:]
			}
			for _, p := range sPorts {
				servicePort, err := strconv.Atoi(p)
				if err != nil {
					return fmt.Errorf("%s is not a valid port", p)
				}
				serviceToCreate.Ports = append(serviceToCreate.Ports, servicePort)
			}

			createServiceOverTlsWithCustomAndGeneratedCerts := createSvcWithGeneratedTlsCerts && serviceToCreate.TlsCredentials != ""

			if createServiceOverTlsWithCustomAndGeneratedCerts {
				return fmt.Errorf("the option --generate-tls-secrets can not be used with custom certificates")
			}
			err := serviceToCreate.SetIngressMode(serviceIngressMode)
			if err != nil {
				return err
			}

			if createSvcWithGeneratedTlsCerts {
				serviceToCreate.TlsCredentials = types.SkupperServiceCertPrefix + serviceToCreate.Address
			}

			return skupperClient.Create(cmd, args)
		},
	}
	cmd.Flags().StringVar(&serviceToCreate.Protocol, "protocol", "tcp", "The mapping in use for this service address (tcp, http, http2)")
	cmd.Flags().StringVar(&serviceToCreate.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().StringVar(&serviceIngressMode, "enable-ingress", "", "Determines whether access to the Skupper service is enabled in this site. Valid values are Always (default) or Never.")
	cmd.Flags().BoolVar(&serviceToCreate.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().BoolVar(&createSvcWithGeneratedTlsCerts, "enable-tls", false, "If specified, the service communication will be encrypted using TLS")
	cmd.Flags().StringVar(&serviceToCreate.Protocol, "mapping", "tcp", "The mapping in use for this service address (currently one of tcp or http)")
	cmd.Flags().BoolVar(&createSvcWithGeneratedTlsCerts, "generate-tls-secrets", false, "If specified, the service communication will be encrypted using TLS")
	cmd.Flags().StringVar(&serviceToCreate.BridgeImage, "bridge-image", "", "The image to use for a bridge running external to the skupper router")
	cmd.Flags().StringVar(&serviceToCreate.TlsCredentials, "tls-cert", "", "K8s secret name with custom certificates to encrypt the communication using TLS (valid only for http2 and tcp protocols)")
	cmd.Flags().StringVar(&serviceToCreate.Namespace, "target-namespace", "", "Expose resources from a specific namespace")

	// platform specific flags
	skupperClient.CreateFlags(cmd)

	f := cmd.Flag("mapping")
	f.Deprecated = "protocol is now the flag to set the mapping"
	f.Hidden = true

	f = cmd.Flag("enable-tls")
	f.Deprecated = "use 'generate-tls-secrets' instead"
	f.Hidden = true

	return cmd
}

func NewCmdDeleteService(skupperClient SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Delete a skupper service",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			return skupperClient.Delete(cmd, args)
		},
	}
	return cmd
}

var bindOptions BindOptions

func NewCmdBind(skupperClient SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bind <service-name> <target-type> <target-name>",
		Short:  "Bind a target to a service",
		Args:   skupperClient.BindArgs,
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			return skupperClient.Bind(cmd, args)
		},
	}

	cmd.Flags().StringSliceVar(&bindOptions.TargetPorts, "target-port", []string{}, "The port the target is listening on (you can also use colon to map source-port to a target-port).")
	cmd.Flags().StringVar(&bindOptions.tlsCertAuthority, "tls-trust", "", "K8s secret name with the CA to expose the service over TLS (valid only for http2 and tcp protocols)")

	skupperClient.BindFlags(cmd)
	return cmd
}

func parsePortMapping(service *types.ServiceInterface, targetPorts []string) (map[int]int, error) {
	if len(targetPorts) > 0 && len(service.Ports) != len(targetPorts) {
		return nil, fmt.Errorf("service defines %d ports but only %d mapped (all ports must be mapped)",
			len(service.Ports), len(targetPorts))
	}
	ports := map[int]int{}
	for _, port := range service.Ports {
		ports[port] = port
	}
	for i, port := range targetPorts {
		portSplit := strings.SplitN(port, ":", 2)
		var sPort, tPort string
		sPort = portSplit[0]
		tPort = sPort
		mapping := false
		if len(portSplit) == 2 {
			tPort = portSplit[1]
			mapping = true
		}
		var isp, itp int
		var err error
		if isp, err = strconv.Atoi(sPort); err != nil {
			return nil, fmt.Errorf("invalid source port: %s", sPort)
		}
		if itp, err = strconv.Atoi(tPort); err != nil {
			return nil, fmt.Errorf("invalid target port: %s", tPort)
		}
		if _, ok := ports[isp]; mapping && !ok {
			return nil, fmt.Errorf("source port not defined in service: %d", isp)
		}
		// if target port not mapped, use positional index to determine it
		if !mapping {
			isp = service.Ports[i]
		}
		ports[isp] = itp
	}
	return ports, nil
}

var unbindNamespace string

func NewCmdUnbind(skupperClient SkupperServiceClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unbind <service-name> <target-type> <target-name>",
		Short:  "Unbind a target from a service",
		Args:   skupperClient.BindArgs,
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Unbind,
	}

	cmd.Flags().StringVar(&unbindNamespace, "target-namespace", "", "Target namespace from previously bound service")
	return cmd
}

func IsZero(v reflect.Value) bool {
	return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}

func NewCmdVersion(skupperClient SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Report the version of the Skupper CLI and services",
		Args:   cobra.NoArgs,
		PreRun: skupperClient.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			fmt.Printf("%-30s %s\n", "client version", version.Version)
			return skupperClient.Version(cmd, args)
		},
	}
	return cmd
}

func NewCmdDebug() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "debug dump <file>, debug events or debug service <service-name>",
		Short: "Debug skupper installation",
	}
	return cmd
}

func NewCmdDebugDump(skupperCli SkupperDebugClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dump <filename>.tar.gz",
		Short:  "Collect and store skupper logs, config, etc. to compressed archive file",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperCli.NewClient,
		RunE:   skupperCli.Dump,
	}
	return cmd
}

var verbose bool

func NewCmdDebugEvents(skupperClient SkupperDebugClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "events",
		Short:  "Show events",
		Args:   cobra.NoArgs,
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Events,
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "More detailed output (in json)")
	return cmd
}

func NewCmdDebugService(skupperClient SkupperDebugClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "service <service-name>",
		Short:  "Check the internal state of a skupper exposed service",
		Args:   cobra.ExactArgs(1),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.Service,
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "More detailed output (in json)")
	return cmd
}

func NewCmdRevokeaccess(skupperClient SkupperSiteClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke-access",
		Short: "Revoke all previously granted access to the site.",
		Long: `This will invalidate all previously issued tokens and require that all
links to this site be re-established with new tokens.`,
		Args:   cobra.ExactArgs(0),
		PreRun: skupperClient.NewClient,
		RunE:   skupperClient.RevokeAccess,
	}
	return cmd
}

func NewCmdCompletion() *cobra.Command {
	completionLong := `
Output shell completion code for bash.
The shell code must be evaluated to provide interactive
completion of skupper commands.  This can be done by sourcing it from
the .bash_profile. i.e.: $ source <(skupper completion)
`

	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Output shell completion code for bash",
		Long:  completionLong,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenBashCompletion(os.Stdout)

		},
	}
	return cmd
}

func NewCmdMan() *cobra.Command {
	var outDir string
	cmd := &cobra.Command{
		Use:   "man",
		Short: "generate man pages",
		RunE: func(cmd *cobra.Command, args []string) error {
			info, err := os.Stat(outDir)
			if err != nil && os.IsNotExist(err) {
				err = os.MkdirAll(outDir, 0755)
				if err != nil {
					return fmt.Errorf("error creating output directory - %w", err)
				}
			} else if err == nil {
				if !info.IsDir() {
					return fmt.Errorf("%s is not a directory", outDir)
				}
			}
			header := &doc.GenManHeader{
				Title:   "SKUPPER",
				Section: "1",
			}
			err = doc.GenManTree(rootCmd, header, outDir)
			if err != nil {
				return err
			}
			err = doc.GenMarkdownTree(rootCmd, outDir)
			if err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outDir, "output-directory", "", "docs.out", "output directory")
	cmd.Hidden = true
	return cmd
}

type cobraFunc func(cmd *cobra.Command, args []string)

var rootCmd *cobra.Command

func isSupported(skupperCli SkupperClient, cmd string) bool {
	return utils.StringSliceContains(skupperCli.SupportedCommands(), cmd)
}

func addCommands(skupperCli SkupperClient, rootCmd *cobra.Command, cmds ...*cobra.Command) {
	for _, cmd := range cmds {
		if isSupported(skupperCli, cmd.Name()) {
			rootCmd.AddCommand(cmd)
		}
	}
}

func init() {
	rootCmd = &cobra.Command{Use: "skupper"}
	routev1.AddToScheme(scheme.Scheme)

	rootCmd.PersistentFlags().StringVarP(&config.Platform, "platform", "", "", "The platform type to use [kubernetes, podman]")
	rootCmd.ParseFlags(os.Args)

	var skupperCli SkupperClient
	switch config.GetPlatform() {
	case types.PlatformKubernetes:
		skupperCli = &SkupperKube{}
	case types.PlatformPodman:
		skupperCli = &SkupperPodman{}
	default:
		fmt.Printf("invalid platform: %s", config.GetPlatform())
		fmt.Println()
		os.Exit(1)
	}

	cmdInit := NewCmdInit(skupperCli.Site())
	cmdDelete := NewCmdDelete(skupperCli.Site())
	cmdUpdate := NewCmdUpdate(skupperCli.Site())
	cmdStatus := NewCmdStatus(skupperCli.Site())
	cmdExpose := NewCmdExpose(skupperCli.Service())
	cmdUnexpose := NewCmdUnexpose(skupperCli.Service())
	cmdCreateService := NewCmdCreateService(skupperCli.Service())
	cmdDeleteService := NewCmdDeleteService(skupperCli.Service())
	cmdStatusService := NewCmdServiceStatus(skupperCli.Service())
	cmdLabelsService := NewCmdServiceLabel(skupperCli.Service())

	cmdVersion := NewCmdVersion(skupperCli.Site())
	cmdDebugDump := NewCmdDebugDump(skupperCli.Debug())
	cmdDebugEvents := NewCmdDebugEvents(skupperCli.Debug())
	cmdDebugService := NewCmdDebugService(skupperCli.Debug())

	// Gateway command is only valid on Kubernetes sites
	cmdGateway := NewCmdGateway()
	if skupperKube, ok := skupperCli.(*SkupperKube); ok {
		cmdInitGateway := NewCmdInitGateway(skupperKube)
		cmdExportConfigGateway := NewCmdExportConfigGateway(skupperKube)
		cmdGenerateBundleGateway := NewCmdGenerateBundleGateway(skupperKube)
		cmdDeleteGateway := NewCmdDeleteGateway(skupperKube)
		cmdExposeGateway := NewCmdExposeGateway(skupperKube)
		cmdUnexposeGateway := NewCmdUnexposeGateway(skupperKube)
		cmdStatusGateway := NewCmdStatusGateway(skupperKube)
		cmdBindGateway := NewCmdBindGateway(skupperKube)
		cmdUnbindGateway := NewCmdUnbindGateway(skupperKube)
		cmdForwardGateway := NewCmdForwardGateway(skupperKube)
		cmdUnforwardGateway := NewCmdUnforwardGateway(skupperKube)

		cmdGateway.AddCommand(cmdInitGateway)
		cmdGateway.AddCommand(cmdExportConfigGateway)
		cmdGateway.AddCommand(cmdGenerateBundleGateway)
		cmdGateway.AddCommand(cmdDeleteGateway)
		cmdGateway.AddCommand(cmdExposeGateway)
		cmdGateway.AddCommand(cmdUnexposeGateway)
		cmdGateway.AddCommand(cmdStatusGateway)
		cmdGateway.AddCommand(cmdBindGateway)
		cmdGateway.AddCommand(cmdUnbindGateway)
		cmdGateway.AddCommand(cmdForwardGateway)
		cmdGateway.AddCommand(cmdUnforwardGateway)
	}

	// setup subcommands
	cmdService := NewCmdService()
	cmdService.AddCommand(cmdCreateService)
	cmdService.AddCommand(cmdDeleteService)
	cmdService.AddCommand(NewCmdBind(skupperCli.Service()))
	cmdService.AddCommand(NewCmdUnbind(skupperCli.Service()))
	cmdService.AddCommand(cmdStatusService)
	// Labels cannot be modified on podman sites
	if config.GetPlatform() == types.PlatformKubernetes {
		cmdService.AddCommand(cmdLabelsService)
	}

	cmdDebug := NewCmdDebug()
	cmdDebug.AddCommand(cmdDebugDump)
	cmdDebug.AddCommand(cmdDebugEvents)
	cmdDebug.AddCommand(cmdDebugService)

	cmdLink := NewCmdLink()
	cmdLink.AddCommand(NewCmdLinkCreate(skupperCli.Link(), ""))
	cmdLink.AddCommand(NewCmdLinkDelete(skupperCli.Link()))
	cmdLink.AddCommand(NewCmdLinkStatus(skupperCli.Link()))

	cmdToken := NewCmdToken()
	cmdToken.AddCommand(NewCmdTokenCreate(skupperCli.Token(), ""))

	cmdCompletion := NewCmdCompletion()

	cmdRevokeAll := NewCmdRevokeaccess(skupperCli.Site())

	cmdNetwork := NewCmdNetwork()
	cmdNetwork.AddCommand(NewCmdNetworkStatus(skupperCli.Network()))

	cmdSwitch := NewCmdSwitch()
	cmdSwitch.Hidden = true

	addCommands(skupperCli, rootCmd,
		cmdInit,
		cmdDelete,
		cmdUpdate,
		cmdToken,
		cmdLink,
		cmdStatus,
		cmdExpose,
		cmdUnexpose,
		cmdService,
		cmdVersion,
		cmdDebug,
		cmdCompletion,
		cmdGateway,
		cmdRevokeAll,
		cmdNetwork)

	rootCmd.AddCommand(cmdSwitch)
	rootCmd.AddCommand(NewCmdMan())
	skupperCli.Options(rootCmd)

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
