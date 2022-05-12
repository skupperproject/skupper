package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
)

type ExposeOptions struct {
	Protocol       string
	Address        string
	Ports          []int
	TargetPorts    []string
	Headless       bool
	ProxyTuning    types.Tuning
	EnableTls      bool
	TlsCredentials string
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
			service, err = cli.GetHeadlessServiceConfiguration(targetName, options.Protocol, options.Address, options.Ports)
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
				Address:        serviceName,
				Ports:          options.Ports,
				Protocol:       options.Protocol,
				EnableTls:      options.EnableTls,
				TlsCredentials: options.TlsCredentials,
			}
		}
	} else if service.Headless != nil {
		return "", fmt.Errorf("Service already exposed as headless")
	} else if options.Headless {
		return "", fmt.Errorf("Service already exposed, cannot reconfigure as headless")
	} else if options.Protocol != "" && service.Protocol != options.Protocol {
		return "", fmt.Errorf("Invalid protocol %s for service with mapping %s", options.Protocol, service.Protocol)
	}

	// service may exist from remote origin
	service.Origin = ""

	targetPorts, err := parsePortMapping(service, options.TargetPorts)
	if err != nil {
		return "", err
	}

	err = cli.ServiceInterfaceBind(ctx, service, targetType, targetName, options.Protocol, targetPorts)
	if errors.IsNotFound(err) {
		return "", SkupperNotInstalledError(cli.GetNamespace())
	} else if err != nil {
		return "", fmt.Errorf("Unable to create skupper service: %w", err)
	}

	return options.Address, nil
}

var validExposeTargets = []string{"deployment", "statefulset", "pods", "service"}

func verifyTargetTypeFromArgs(args []string) error {
	targetType, _ := parseTargetTypeAndName(args)
	if !utils.StringSliceContains(validExposeTargets, targetType) {
		return fmt.Errorf("target type must be one of: [%s]", strings.Join(validExposeTargets, ", "))
	}
	return nil
}

func exposeTargetArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 1 || (!strings.Contains(args[0], "/") && len(args) < 2) {
		return fmt.Errorf("expose target and name must be specified (e.g. 'skupper expose deployment <name>'")
	}
	if len(args) > 2 {
		return fmt.Errorf("illegal argument: %s", args[2])
	}
	if len(args) > 1 && strings.Contains(args[0], "/") {
		return fmt.Errorf("extra argument: %s", args[1])
	}
	return verifyTargetTypeFromArgs(args)
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

func bindArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || (!strings.Contains(args[1], "/") && len(args) < 3) {
		return fmt.Errorf("Service name, target type and target name must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-name>')")
	}
	if len(args) > 3 {
		return fmt.Errorf("illegal argument: %s", args[3])
	}
	if len(args) > 2 && strings.Contains(args[1], "/") {
		return fmt.Errorf("extra argument: %s", args[2])
	}
	return verifyTargetTypeFromArgs(args[1:])
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

func NewClient(namespace string, context string, kubeConfigPath string) *client.VanClient {
	return NewClientHandleError(namespace, context, kubeConfigPath, true)
}

func NewClientHandleError(namespace string, context string, kubeConfigPath string, exitOnError bool) *client.VanClient {
	cli, err := client.NewClient(namespace, context, kubeConfigPath)
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

var ClusterLocal bool

func NewCmdInit(newClient cobraFunc) *cobra.Command {
	var routerMode string
	annotations := []string{}
	ingressAnnotations := []string{}
	routerServiceAnnotations := []string{}
	controllerServiceAnnotations := []string{}
	labels := []string{}
	var isEdge bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long: `Setup a router and other supporting objects to provide a functional skupper
installation that can then be connected to other skupper installations`,
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: should cli allow init to diff ns?
			silenceCobra(cmd)
			ns := cli.GetNamespace()

			routerModeFlag := cmd.Flag("router-mode")
			edgeFlag := cmd.Flag("edge")
			if routerModeFlag.Changed && edgeFlag.Changed {
				return fmt.Errorf("You can not use the deprecated --edge, and --router-mode together, use --router-mode")
			}

			if routerModeFlag.Changed {
				options := []string{string(types.TransportModeInterior), string(types.TransportModeEdge)}
				if !utils.StringSliceContains(options, routerMode) {
					return fmt.Errorf(`invalid "--router-mode=%v", it must be one of "%v"`, routerMode, strings.Join(options, ", "))
				}
				routerCreateOpts.RouterMode = routerMode
			} else {
				if isEdge {
					routerCreateOpts.RouterMode = string(types.TransportModeEdge)
				} else {
					routerCreateOpts.RouterMode = string(types.TransportModeInterior)
				}
			}

			routerIngressFlag := cmd.Flag("ingress")
			routerClusterLocalFlag := cmd.Flag("cluster-local")

			if routerIngressFlag.Changed && routerClusterLocalFlag.Changed {
				return fmt.Errorf(`You can not use the deprecated --cluster-local, and --ingress together, use "--ingress none" as equivalent of --cluster-local`)
			} else if routerClusterLocalFlag.Changed {
				if ClusterLocal { // this is redundant, because "if changed" it must be true, but it is also correct
					routerCreateOpts.Ingress = types.IngressNoneString
				}
			} else if !routerIngressFlag.Changed {
				routerCreateOpts.Ingress = cli.GetIngressDefault()
			}
			if routerCreateOpts.Ingress == types.IngressNodePortString && routerCreateOpts.IngressHost == "" && routerCreateOpts.Router.IngressHost == "" {
				return fmt.Errorf(`One of --ingress-host or --router-ingress-host option is required when using "--ingress nodeport"`)
			}
			if routerCreateOpts.Ingress == types.IngressContourHttpProxyString && routerCreateOpts.IngressHost == "" {
				return fmt.Errorf(`--ingress-host option is required when using "--ingress contour-http-proxy"`)
			}
			routerCreateOpts.Annotations = asMap(annotations)
			routerCreateOpts.Labels = asMap(labels)
			routerCreateOpts.IngressAnnotations = asMap(ingressAnnotations)
			routerCreateOpts.Router.ServiceAnnotations = asMap(routerServiceAnnotations)
			routerCreateOpts.Controller.ServiceAnnotations = asMap(controllerServiceAnnotations)
			if err := routerCreateOpts.CheckIngress(); err != nil {
				return err
			}
			if err := routerCreateOpts.CheckConsoleIngress(); err != nil {
				return err
			}

			routerCreateOpts.SkupperNamespace = ns
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if err != nil {
				return err
			}
			if routerLogging != "" {
				logConfig, err := client.ParseRouterLogConfig(routerLogging)
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

			if siteConfig == nil {
				siteConfig, err = cli.SiteConfigCreate(context.Background(), routerCreateOpts)
				if err != nil {
					return err
				}
			} else {
				updated, err := cli.SiteConfigUpdate(context.Background(), routerCreateOpts)
				if err != nil {
					return fmt.Errorf("Error while trying to update router configuration: %s", err)
				}
				if len(updated) > 0 {
					for _, i := range updated {
						fmt.Println("Updated", i)
					}
				}
			}

			err = cli.RouterCreate(context.Background(), *siteConfig)
			if err != nil {
				return err
			}
			fmt.Println("Skupper is now installed in namespace '" + ns + "'.  Use 'skupper status' to get more information.")

			return nil
		},
	}
	routerCreateOpts.EnableController = true
	cmd.Flags().StringVarP(&routerCreateOpts.SkupperName, "site-name", "", "", "Provide a specific name for this skupper installation")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", true, "Enable skupper console")
	cmd.Flags().BoolVarP(&routerCreateOpts.CreateNetworkPolicy, "create-network-policy", "", false, "Create network policy to restrict access to skupper services exposed through this site to current pods in namespace")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Ingress, "ingress", "", "", "Setup Skupper ingress to one of: ["+strings.Join(types.ValidIngressOptions(), "|")+"]. If not specified route is used when available, otherwise loadbalancer is used.")
	cmd.Flags().StringSliceVar(&ingressAnnotations, "ingress-annotations", []string{}, "Annotations to add to skupper ingress")
	cmd.Flags().StringVarP(&routerCreateOpts.ConsoleIngress, "console-ingress", "", "", "Determines if/how console is exposed outside cluster. If not specified uses value of --ingress. One of: ["+strings.Join(types.ValidIngressOptions(), "|")+"].")
	cmd.Flags().StringVarP(&routerCreateOpts.IngressHost, "ingress-host", "", "", "Hostname or alias by which the ingress route or proxy can be reached")
	cmd.Flags().StringVarP(&routerMode, "router-mode", "", string(types.TransportModeInterior), "Skupper router-mode")

	cmd.Flags().StringSliceVar(&annotations, "annotations", []string{}, "Annotations to add to skupper pods")
	cmd.Flags().StringSliceVar(&labels, "labels", []string{}, "Labels to add to skupper pods")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Participate in cross-site service synchronization")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmd.Flags().StringVarP(&routerLogging, "router-logging", "", "", "Logging settings for router. 'trace', 'debug', 'info' (default), 'notice', 'warning', and 'error' are valid values.")
	cmd.Flags().StringVarP(&routerCreateOpts.Router.DebugMode, "router-debug-mode", "", "", "Enable debug mode for router ('asan' or 'gdb' are valid values)")

	cmd.Flags().IntVar(&routerCreateOpts.Routers, "routers", 0, "Number of router replicas to start")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Cpu, "router-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Memory, "router-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.CpuLimit, "router-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.MemoryLimit, "router-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.NodeSelector, "router-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Affinity, "router-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.AntiAffinity, "router-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.IngressHost, "router-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")
	cmd.Flags().StringSliceVar(&routerServiceAnnotations, "router-service-annotations", []string{}, "Annotations to add to skupper router service")
	cmd.Flags().StringVar(&routerCreateOpts.Router.LoadBalancerIp, "router-load-balancer-ip", "", "Load balancer ip that will be used for router service, if supported by cloud provider")

	cmd.Flags().StringVar(&routerCreateOpts.Controller.Cpu, "controller-cpu", "", "CPU request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Memory, "controller-memory", "", "Memory request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.CpuLimit, "controller-cpu-limit", "", "CPU limit for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.MemoryLimit, "controller-memory-limit", "", "Memory limit for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.NodeSelector, "controller-node-selector", "", "Node selector to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Affinity, "controller-pod-affinity", "", "Pod affinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.AntiAffinity, "controller-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.IngressHost, "controller-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")
	cmd.Flags().StringSliceVar(&controllerServiceAnnotations, "controller-service-annotation", []string{}, "Annotations to add to skupper controller service")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.LoadBalancerIp, "controller-load-balancer-ip", "", "Load balancer ip that will be used for controller service, if supported by cloud provider")

	cmd.Flags().BoolVarP(&ClusterLocal, "cluster-local", "", false, "Set up Skupper to only accept links from within the local cluster.")
	f := cmd.Flag("cluster-local")
	f.Deprecated = "This flag is deprecated, use --ingress [loadbalancer|route|none]"
	f.Hidden = true

	cmd.Flags().BoolVarP(&isEdge, "edge", "", false, "Configure as an edge")
	f = cmd.Flag("edge")
	f.Deprecated = "This flag is deprecated, use --router-mode [interior|edge]"
	f.Hidden = true

	cmd.Flags().IntVar(&routerCreateOpts.Router.MaxFrameSize, "xp-router-max-frame-size", types.RouterMaxFrameSizeDefault, "Set  max frame size on inter-router listeners/connectors")
	cmd.Flags().IntVar(&routerCreateOpts.Router.MaxSessionFrames, "xp-router-max-session-frames", types.RouterMaxSessionFramesDefault, "Set  max session frames on inter-router listeners/connectors")
	hideFlag(cmd, "xp-router-max-frame-size")
	hideFlag(cmd, "xp-router-max-session-frames")
	cmd.Flags().SortFlags = false

	return cmd
}

func hideFlag(cmd *cobra.Command, name string) {
	f := cmd.Flag(name)
	f.Hidden = true
}

func NewCmdDelete(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete",
		Short:  "Delete skupper installation",
		Long:   `delete will delete any skupper related objects from the namespace`,
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			gateways, err := cli.GatewayList(context.Background())
			for _, gateway := range gateways {
				cli.GatewayRemove(context.Background(), gateway.GatewayName)
			}
			err = cli.SiteConfigRemove(context.Background())
			if err != nil {
				err = cli.RouterRemove(context.Background())
			}
			if err != nil {
				return err
			} else {
				fmt.Println("Skupper is now removed from '" + cli.GetNamespace() + "'.")
			}
			return nil
		},
	}
	return cmd
}

var forceHup bool

func NewCmdUpdate(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "update",
		Short:  "Update skupper installation version",
		Long:   "Update the skupper site to " + client.Version,
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			updated, err := cli.RouterUpdateVersion(context.Background(), forceHup)
			if err != nil {
				return err
			}
			if updated {
				fmt.Println("Skupper is now updated in '" + cli.GetNamespace() + "'.")
			} else {
				fmt.Println("No update required in '" + cli.GetNamespace() + "'.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&forceHup, "force-restart", "", false, "Restart skupper daemons even if image tag is not updated")
	return cmd
}

var clientIdentity string

func NewCmdConnectionToken(newClient cobraFunc) *cobra.Command {
	cmd := NewCmdTokenCreate(newClient, "client-identity")
	cmd.Use = "connection-token <output-file>"
	cmd.Short = "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site."
	return cmd
}

func NewCmdConnect(newClient cobraFunc) *cobra.Command {
	cmd := NewCmdLinkCreate(newClient, "connection-name")
	cmd.Use = "connect <connection-token-file>"
	cmd.Short = "Connect this skupper installation to that which issued the specified connectionToken"
	return cmd

}
func NewCmdDisconnect(newClient cobraFunc) *cobra.Command {
	cmd := NewCmdLinkDelete(newClient)
	cmd.Use = "disconnect <name>"
	cmd.Short = "Remove specified connection"
	return cmd

}
func NewCmdCheckConnection(newClient cobraFunc) *cobra.Command {
	cmd := NewCmdLinkStatus(newClient)
	cmd.Use = "check-connection all|<connection-name>"
	cmd.Short = "Check whether a connection to another Skupper site is active"
	return cmd
}

func NewCmdListConnectors(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "list-connectors",
		Short:  "List configured outgoing connections",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			connectors, err := cli.ConnectorList(context.Background())
			if err == nil {
				if len(connectors) == 0 {
					fmt.Println("There are no connectors defined.")
				} else {
					fmt.Println("Connectors:")
					for _, c := range connectors {
						fmt.Printf("    %s (name=%s)", c.Url, c.Name)
						fmt.Println()
					}
				}
			} else if errors.IsNotFound(err) {
				return SkupperNotInstalledError(cli.GetNamespace())
			} else {
				return fmt.Errorf("Unable to retrieve connections: %w", err)
			}
			return nil
		},
	}
	return cmd
}

func NewCmdStatus(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "Report the status of the current Skupper site",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			vir, err := cli.RouterInspect(context.Background())
			if err == nil {
				ns := cli.GetNamespace()
				var modedesc string = " in interior mode"
				if vir.Status.Mode == string(types.TransportModeEdge) {
					modedesc = " in edge mode"
				}
				sitename := ""
				if vir.Status.SiteName != "" && vir.Status.SiteName != ns {
					sitename = fmt.Sprintf(" with site name %q", vir.Status.SiteName)
				}
				policyStr := ""
				if vanClient, ok := cli.(*client.VanClient); ok {
					p := client.NewPolicyValidatorAPI(vanClient)
					r, err := p.IncomingLink()
					if err == nil && r.Enabled {
						policyStr = " (with policies)"
					}
				}
				fmt.Printf("Skupper is enabled for namespace %q%s%s%s.", ns, sitename, modedesc, policyStr)
				if vir.Status.TransportReadyReplicas == 0 {
					fmt.Printf(" Status pending...")
				} else {
					if len(vir.Status.ConnectedSites.Warnings) > 0 {
						for _, w := range vir.Status.ConnectedSites.Warnings {
							fmt.Printf("Warning: %s", w)
							fmt.Println()
						}
					}
					if vir.Status.ConnectedSites.Total == 0 {
						fmt.Printf(" It is not connected to any other sites.")
					} else if vir.Status.ConnectedSites.Total == 1 {
						fmt.Printf(" It is connected to 1 other site.")
					} else if vir.Status.ConnectedSites.Total == vir.Status.ConnectedSites.Direct {
						fmt.Printf(" It is connected to %d other sites.", vir.Status.ConnectedSites.Total)
					} else {
						fmt.Printf(" It is connected to %d other sites (%d indirectly).", vir.Status.ConnectedSites.Total, vir.Status.ConnectedSites.Indirect)
					}
				}
				if vir.ExposedServices == 0 {
					fmt.Printf(" It has no exposed services.")
				} else if vir.ExposedServices == 1 {
					fmt.Printf(" It has 1 exposed service.")
				} else {
					fmt.Printf(" It has %d exposed services.", vir.ExposedServices)
				}
				fmt.Println()
				if vir.ConsoleUrl != "" {
					fmt.Println("The site console url is: ", vir.ConsoleUrl)
					siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
					if err != nil {
						return err
					}
					if siteConfig.Spec.AuthMode == "internal" {
						fmt.Println("The credentials for internal console-auth mode are held in secret: 'skupper-console-users'")
					}
				}
			} else {
				if vir == nil {
					fmt.Printf("Skupper is not enabled in namespace '%s'\n", cli.GetNamespace())
				} else {
					return fmt.Errorf("Unable to retrieve skupper status: %w", err)
				}
			}
			return nil
		},
	}
	return cmd
}

var exposeOpts ExposeOptions

func NewCmdExpose(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "expose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short:  "Expose a set of pods through a Skupper address",
		Args:   exposeTargetArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			targetType, targetName := parseTargetTypeAndName(args)

			// silence cobra may be moved below the "if" we want to print
			// the usage message along with this error
			if exposeOpts.Address == "" {
				if targetType == "service" {
					return fmt.Errorf("--address option is required for target type 'service'")
				}
				if !exposeOpts.Headless {
					exposeOpts.Address = targetName
				}
			}
			if !exposeOpts.Headless {
				if exposeOpts.ProxyTuning.Cpu != "" {
					return fmt.Errorf("--proxy-cpu option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.Memory != "" {
					return fmt.Errorf("--proxy-memory option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.CpuLimit != "" {
					return fmt.Errorf("--proxy-cpu-limit option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.MemoryLimit != "" {
					return fmt.Errorf("--proxy-memory-limit option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.Affinity != "" {
					return fmt.Errorf("--proxy-pod-affinity option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.AntiAffinity != "" {
					return fmt.Errorf("--proxy-pod-antiaffinity option is only valid for headless services")
				}
				if exposeOpts.ProxyTuning.NodeSelector != "" {
					return fmt.Errorf("--proxy-node-selector option is only valid for headless services")
				}
			}

			if exposeOpts.EnableTls {
				exposeOpts.TlsCredentials = types.SkupperServiceCertPrefix + exposeOpts.Address
			}

			addr, err := expose(cli, context.Background(), targetType, targetName, exposeOpts)
			if err == nil {
				fmt.Printf("%s %s exposed as %s\n", targetType, targetName, addr)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&(exposeOpts.Protocol), "protocol", "tcp", "The protocol to proxy (tcp, http, or http2)")
	cmd.Flags().StringVar(&(exposeOpts.Address), "address", "", "The Skupper address to expose")
	cmd.Flags().IntSliceVar(&(exposeOpts.Ports), "port", []int{}, "The ports to expose on")
	cmd.Flags().StringSliceVar(&(exposeOpts.TargetPorts), "target-port", []string{}, "The ports to target on pods")
	cmd.Flags().BoolVar(&(exposeOpts.Headless), "headless", false, "Expose through a headless service (valid only for a statefulset target)")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Cpu, "proxy-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Memory, "proxy-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.CpuLimit, "proxy-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.MemoryLimit, "proxy-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.NodeSelector, "proxy-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Affinity, "proxy-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.AntiAffinity, "proxy-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().BoolVar(&exposeOpts.EnableTls, "enable-tls", false, "If specified, the service will be exposed over TLS (valid only for http2 protocol)")

	return cmd
}

var unexposeAddress string

func NewCmdUnexpose(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unexpose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short:  "Unexpose a set of pods previously exposed through a Skupper address",
		Args:   exposeTargetArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			targetType, targetName := parseTargetTypeAndName(args)

			err := cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, unexposeAddress, true)
			if err == nil {
				fmt.Printf("%s %s unexposed\n", targetType, targetName)
			} else {
				return fmt.Errorf("Unable to unbind skupper service: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&unexposeAddress, "address", "", "Skupper address the target was exposed as")

	return cmd
}

func NewCmdListExposed(newClient cobraFunc) *cobra.Command {
	cmd := NewCmdServiceStatus(newClient)
	cmd.Use = "list-exposed"
	return cmd
}

func NewCmdServiceStatus(newClient cobraFunc) *cobra.Command {
	showLabels := false
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "List services exposed over the service network",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			vsis, err := cli.ServiceInterfaceList(context.Background())
			if err == nil {
				if len(vsis) == 0 {
					fmt.Println("No services defined")
				} else {
					l := formatter.NewList()
					l.Item("Services exposed through Skupper:")
					addresses := []string{}
					for _, si := range vsis {
						addresses = append(addresses, si.Address)
					}
					svcAuth := map[string]bool{}
					for _, addr := range addresses {
						svcAuth[addr] = true
					}
					if vc, ok := cli.(*client.VanClient); ok {
						policy := client.NewPolicyValidatorAPI(vc)
						res, _ := policy.Services(addresses...)
						for addr, auth := range res {
							svcAuth[addr] = auth.Allowed
						}
					}

					for _, si := range vsis {
						portStr := "port"
						if len(si.Ports) > 1 {
							portStr = "ports"
						}
						for _, port := range si.Ports {
							portStr += fmt.Sprintf(" %d", port)
						}
						authSuffix := ""
						if !svcAuth[si.Address] {
							authSuffix = " - not authorized"
						}
						svc := l.NewChild(fmt.Sprintf("%s (%s %s)%s", si.Address, si.Protocol, portStr, authSuffix))
						if len(si.Targets) > 0 {
							targets := svc.NewChild("Targets:")
							for _, t := range si.Targets {
								var name string
								if t.Name != "" {
									name = fmt.Sprintf("name=%s", t.Name)
								}
								targetInfo := ""
								if t.Selector != "" {
									targetInfo = fmt.Sprintf("%s %s", t.Selector, name)
								} else if t.Service != "" {
									targetInfo = fmt.Sprintf("%s %s", t.Service, name)
								} else {
									targetInfo = fmt.Sprintf("%s (no selector)", name)
								}
								targets.NewChild(targetInfo)
							}
						}
						if showLabels && len(si.Labels) > 0 {
							labels := svc.NewChild("Labels:")
							for k, v := range si.Labels {
								labels.NewChild(fmt.Sprintf("%s=%s", k, v))
							}
						}
					}
					l.Print()
				}
			} else {
				return fmt.Errorf("Could not retrieve services: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showLabels, "show-labels", false, "show service labels")
	return cmd
}

func NewCmdServiceLabel(newClient cobraFunc) *cobra.Command {

	var addLabels, removeLabels []string
	var showLabels bool
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
		PreRun: newClient,
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
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			si, err := cli.ServiceInterfaceInspect(context.Background(), name)
			if si == nil {
				return fmt.Errorf("invalid service name")
			}
			if err != nil {
				return fmt.Errorf("error retrieving service: %v", err)
			}
			if showLabels {
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
				return nil
			}
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
			err = cli.ServiceInterfaceUpdate(context.Background(), si)
			if err != nil {
				return fmt.Errorf("error updating service labels: %v", err)
			}
			return nil
		},
	}

	return labels
}

func NewCmdService() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service create <name> <port> or service delete port",
		Short: "Manage skupper service definitions",
	}
	return cmd
}

var serviceToCreate types.ServiceInterface

func NewCmdCreateService(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "create <name> <port...>",
		Short:  "Create a skupper service",
		Args:   createServiceArgs,
		PreRun: newClient,
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

			if serviceToCreate.EnableTls {
				serviceToCreate.TlsCredentials = types.SkupperServiceCertPrefix + serviceToCreate.Address
			}

			err := cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceToCreate.Protocol, "mapping", "tcp", "The mapping in use for this service address (currently one of tcp or http)")
	cmd.Flags().StringVar(&serviceToCreate.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&serviceToCreate.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().BoolVar(&serviceToCreate.EnableTls, "enable-tls", false, "If specified, the service communication will be encrypted using TLS")

	return cmd
}

func NewCmdDeleteService(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Delete a skupper service",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			err := cli.ServiceInterfaceRemove(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		},
	}
	return cmd
}

var targetPorts []string
var protocol string

func NewCmdBind(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bind <service-name> <target-type> <target-name>",
		Short:  "Bind a target to a service",
		Args:   bindArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			if protocol != "" && protocol != "tcp" && protocol != "http" && protocol != "http2" {
				return fmt.Errorf("%s is not a valid protocol. Choose 'tcp', 'http' or 'http2'.", protocol)
			} else {
				targetType, targetName := parseTargetTypeAndName(args[1:])

				service, err := cli.ServiceInterfaceInspect(context.Background(), args[0])

				if err != nil {
					return fmt.Errorf("%w", err)
				} else if service == nil {
					return fmt.Errorf("Service %s not found", args[0])
				}

				// validating ports
				portMapping, err := parsePortMapping(service, targetPorts)
				if err != nil {
					return err
				}

				err = cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, protocol, portMapping)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "", "The protocol to proxy (tcp, http or http2).")
	cmd.Flags().StringSliceVar(&targetPorts, "target-port", []string{}, "The port the target is listening on (you can also use colon to map source-port to a target-port).")

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

func NewCmdUnbind(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unbind <service-name> <target-type> <target-name>",
		Short:  "Unbind a target from a service",
		Args:   bindArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			targetType, targetName := parseTargetTypeAndName(args[1:])

			err := cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		},
	}
	return cmd
}

func IsZero(v reflect.Value) bool {
	return !v.IsValid() || reflect.DeepEqual(v.Interface(), reflect.Zero(v.Type()).Interface())
}

func NewCmdGateway() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gateway init or gateway delete",
		Short: "Manage skupper gateway definitions",
	}
	return cmd
}

const gatewayName string = ""

var gatewayConfigFile string
var gatewayEndpoint types.GatewayEndpoint
var gatewayType string
var deprecatedName string
var deprecatedExportOnly bool

func NewCmdInitGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Short:  "Initialize a gateway to the service network",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			if gatewayType != "" && gatewayType != "service" && gatewayType != "docker" && gatewayType != "podman" {
				return fmt.Errorf("%s is not a valid gateway type. Choose 'service', 'docker' or 'podman'.", gatewayType)
			}

			actual, err := cli.GatewayInit(context.Background(), gatewayName, gatewayType, gatewayConfigFile)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			fmt.Println("Skupper gateway: '" + actual + "'. Use 'skupper gateway status' to get more information.")

			return nil
		},
	}
	cmd.Flags().StringVarP(&gatewayType, "type", "", "service", "The gateway type one of: 'service', 'docker', 'podman'")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of the gateway definition")
	cmd.Flags().StringVar(&gatewayConfigFile, "config", "", "The gateway config file to use for initialization")
	cmd.Flags().BoolVarP(&deprecatedExportOnly, "exportonly", "", false, "Gateway definition for export-config only (e.g. will not be started)")

	f := cmd.Flag("exportonly")
	f.Deprecated = "gateway will be started"
	f.Hidden = true

	f = cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdDeleteGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete",
		Short:  "Stop the gateway instance and remove the definition",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.GatewayRemove(context.Background(), gatewayName)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdDownloadGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "download <output-path>",
		Short:  "Download a gateway definition to a directory",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			fileName, err := cli.GatewayDownload(context.Background(), gatewayName, args[0])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			fmt.Println("Skupper gateway definition written to '" + fileName + "'")
			return nil
		},
	}
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdExportConfigGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "export-config <export-gateway-name> <output-path>",
		Short:  "Export the configuration for a gateway definition",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			// TODO: validate args must be non nil, etc.
			fileName, err := cli.GatewayExportConfig(context.Background(), gatewayName, args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			fmt.Println("Skupper gateway definition configuration written to '" + fileName + "'")
			return nil
		},
	}
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdGenerateBundleGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-bundle <config-file> <output-path>",
		Short:  "Generate an installation bundle using a gateway config file",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			fileName, err := cli.GatewayGenerateBundle(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			fmt.Println("Skupper gateway bundle written to '" + fileName + "'")
			return nil
		},
	}

	return cmd
}

func NewCmdExposeGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "expose <address> <host> <port...>",
		Short:  "Expose a process to the service network (ensure gateway and cluster service)",
		Args:   exposeGatewayArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			if gatewayType != "" && gatewayType != "service" && gatewayType != "docker" && gatewayType != "podman" {
				return fmt.Errorf("%s is not a valid gateway type. Choose 'service', 'docker' or 'podman'.", gatewayType)
			}

			if len(args) == 2 {
				parts := strings.Split(args[1], ":")
				gatewayEndpoint.Host = parts[0]
				port, _ := strconv.Atoi(parts[1])
				gatewayEndpoint.Service.Ports = []int{port}
			} else {
				tPorts := []int{}
				sPorts := []int{}
				for _, v := range args[2:] {
					ports := strings.Split(v, ":")
					sPort, _ := strconv.Atoi(ports[0])
					tPort := sPort
					if len(ports) == 2 {
						tPort, _ = strconv.Atoi(ports[1])
					}
					sPorts = append(sPorts, sPort)
					tPorts = append(tPorts, tPort)
				}
				gatewayEndpoint.Host = args[1]
				gatewayEndpoint.Service.Ports = sPorts
				gatewayEndpoint.TargetPorts = tPorts
			}
			gatewayEndpoint.Service.Address = args[0]

			_, err := cli.GatewayExpose(context.Background(), gatewayName, gatewayType, gatewayEndpoint)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Protocol, "protocol", "tcp", "The protocol to gateway (tcp, http or http2).")
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&gatewayEndpoint.Service.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().StringVarP(&gatewayType, "type", "", "service", "The gateway type one of: 'service', 'docker', 'podman'")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

var deleteLast bool

func NewCmdUnexposeGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unexpose <address>",
		Short:  "Unexpose a process previously exposed to the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := cli.GatewayUnexpose(context.Background(), gatewayName, gatewayEndpoint, deleteLast)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().BoolVarP(&deleteLast, "delete-last", "", true, "Delete the gateway if no services remain")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdBindGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bind <address> <host> <port...>",
		Short:  "Bind a process to the service network",
		Args:   bindGatewayArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			if len(args) == 2 {
				parts := strings.Split(args[1], ":")
				port, _ := strconv.Atoi(parts[1])
				gatewayEndpoint.Host = parts[0]
				gatewayEndpoint.Service.Ports = []int{port}
			} else {
				ports := []int{}
				for _, p := range args[2:] {
					port, _ := strconv.Atoi(p)
					ports = append(ports, port)
				}
				gatewayEndpoint.Host = args[1]
				gatewayEndpoint.Service.Ports = ports
			}
			gatewayEndpoint.Service.Address = args[0]
			gatewayEndpoint.Name = args[0]

			err := cli.GatewayBind(context.Background(), gatewayName, gatewayEndpoint)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Protocol, "protocol", "tcp", "The mapping in use for this service address (currently on of tcp, http or http2).")
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&gatewayEndpoint.Service.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	f = cmd.Flag("protocol")
	f.Deprecated = "protocol is derived from service definition"
	f.Hidden = true

	return cmd
}

func NewCmdUnbindGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unbind <address>",
		Short:  "Unbind a process from the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := cli.GatewayUnbind(context.Background(), gatewayName, gatewayEndpoint)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Protocol, "protocol", "tcp", "The protocol to gateway (tcp, http or http2).")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	f = cmd.Flag("protocol")
	f.Deprecated = "protocol is derived from service definition"
	f.Hidden = true

	return cmd
}

func NewCmdStatusGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status <gateway-name>",
		Short:  "Report the status of the gateway(s) for the current skupper site",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gateways, err := cli.GatewayList(context.Background())
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			if len(gateways) == 0 {
				fmt.Println("No gateway definitions found")
				return nil
			}

			l := formatter.NewList()
			l.Item("Gateway Definition:")
			for _, gateway := range gateways {
				gw := l.NewChild(fmt.Sprintf("%s type:%s version:%s", gateway.GatewayName, gateway.GatewayType, gateway.GatewayVersion))
				if len(gateway.GatewayConnectors) > 0 {
					listeners := gw.NewChild("Bindings:")
					for _, connector := range gateway.GatewayConnectors {
						listeners.NewChild(fmt.Sprintf("%s %s %s %s %d", strings.TrimPrefix(connector.Name, gateway.GatewayName+"-egress-"), connector.Service.Protocol, connector.Service.Address, connector.Host, connector.Service.Ports[0]))
					}
				}
				if len(gateway.GatewayListeners) > 0 {
					listeners := gw.NewChild("Forwards:")
					for _, listener := range gateway.GatewayListeners {
						listeners.NewChild(fmt.Sprintf("%s %s %s %s %d:%s", strings.TrimPrefix(listener.Name, gateway.GatewayName+"-ingress-"), listener.Service.Protocol, listener.Service.Address, listener.Host, listener.Service.Ports[0], listener.LocalPort))
					}
				}
			}
			l.Print()

			return nil
		},
	}

	return cmd
}

func NewCmdForwardGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "forward <address> <port...>",
		Short:  "Forward an address to the service network",
		Args:   cobra.MinimumNArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			ports := []int{}
			for _, p := range args[1:] {
				port, err := strconv.Atoi(p)
				if err != nil {
					return fmt.Errorf("%s is not a valid forward port", p)
				}
				ports = append(ports, port)
			}

			gatewayEndpoint.Service.Address = args[0]
			gatewayEndpoint.Service.Ports = ports

			err := cli.GatewayForward(context.Background(), gatewayName, gatewayEndpoint)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Protocol, "protocol", "tcp", "The mapping in use for this service address (currently one of tcp, http or http2)")
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&gatewayEndpoint.Service.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().BoolVarP(&gatewayEndpoint.Loopback, "loopback", "", false, "Forward from loopback only")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	f = cmd.Flag("protocol")
	f.Deprecated = "protocol is derived from service definition"
	f.Hidden = true

	return cmd
}

func NewCmdUnforwardGateway(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unforward <address>",
		Short:  "Stop forwarding an address to the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := cli.GatewayUnforward(context.Background(), gatewayName, gatewayEndpoint)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayEndpoint.Service.Protocol, "protocol", "tcp", "The protocol to gateway (tcp, http or http2).")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	f = cmd.Flag("protocol")
	f.Deprecated = "protocol is derived from service definition"
	f.Hidden = true

	return cmd
}

func NewCmdVersion(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Report the version of the Skupper CLI and services",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			fmt.Printf("%-30s %s\n", "client version", client.Version)
			if !IsZero(reflect.ValueOf(cli)) {
				fmt.Printf("%-30s %s\n", "transport version", cli.GetVersion(types.TransportComponentName, types.TransportContainerName))
				fmt.Printf("%-30s %s\n", "controller version", cli.GetVersion(types.ControllerComponentName, types.ControllerContainerName))
				fmt.Printf("%-30s %s\n", "config-sync version", cli.GetVersion(types.TransportComponentName, types.ConfigSyncContainerName))
			} else {
				fmt.Printf("%-30s %s\n", "transport version", "not-found (no configuration has been provided)")
				fmt.Printf("%-30s %s\n", "controller version", "not-found (no configuration has been provided)")
			}
			return nil
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

func NewCmdDebugDump(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "dump <filename>.tar.gz",
		Short:  "Collect and store skupper logs, config, etc. to compressed archive file",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			file, err := cli.SkupperDump(context.Background(), args[0], client.Version, kubeConfigPath, kubeContext)
			if err != nil {
				return fmt.Errorf("Unable to save skupper dump details: %w", err)
			} else {
				fmt.Println("Skupper dump details written to compressed archive: ", file)
			}
			return nil
		},
	}
	return cmd
}

func NewCmdDebugEvents(newClient cobraFunc) *cobra.Command {
	verbose := false
	cmd := &cobra.Command{
		Use:    "events",
		Short:  "Show events",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			output, err := cli.SkupperEvents(verbose)
			if err != nil {
				return err
			}
			os.Stdout.Write(output.Bytes())
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "More detailed output (in json)")
	return cmd
}

func NewCmdDebugService(newClient cobraFunc) *cobra.Command {
	verbose := false
	cmd := &cobra.Command{
		Use:    "service <service-name>",
		Short:  "Check the internal state of a skupper exposed service",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			output, err := cli.SkupperCheckService(args[0], verbose)
			if err != nil {
				return err
			}
			os.Stdout.Write(output.Bytes())
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "More detailed output (in json)")
	return cmd
}

func NewCmdRevokeaccess(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revoke-access",
		Short: "Revoke all previously granted access to the site.",
		Long: `This will invalidate all previously issued tokens and require that all
links to this site be re-established with new tokens.`,
		Args:   cobra.ExactArgs(0),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			err := cli.RevokeAccess(context.Background())
			if err != nil {
				return fmt.Errorf("Unable to revoke access: %w", err)
			}
			return nil
		},
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

type cobraFunc func(cmd *cobra.Command, args []string)

func newClient(cmd *cobra.Command, args []string) {
	cli = NewClient(namespace, kubeContext, kubeConfigPath)
}

func newClientSansExit(cmd *cobra.Command, args []string) {
	cli = NewClientHandleError(namespace, kubeContext, kubeConfigPath, false)
}

var kubeContext string
var namespace string
var kubeConfigPath string
var rootCmd *cobra.Command
var cli types.VanClientInterface

func init() {
	routev1.AddToScheme(scheme.Scheme)

	cmdInit := NewCmdInit(newClient)
	cmdDelete := NewCmdDelete(newClient)
	cmdUpdate := NewCmdUpdate(newClient)
	cmdStatus := NewCmdStatus(newClient)
	cmdExpose := NewCmdExpose(newClient)
	cmdUnexpose := NewCmdUnexpose(newClient)
	cmdListExposed := NewCmdListExposed(newClient)
	cmdCreateService := NewCmdCreateService(newClient)
	cmdDeleteService := NewCmdDeleteService(newClient)
	cmdStatusService := NewCmdServiceStatus(newClient)
	cmdLabelsService := NewCmdServiceLabel(newClient)
	cmdBind := NewCmdBind(newClient)
	cmdUnbind := NewCmdUnbind(newClient)
	cmdVersion := NewCmdVersion(newClientSansExit)
	cmdDebugDump := NewCmdDebugDump(newClient)
	cmdDebugEvents := NewCmdDebugEvents(newClient)
	cmdDebugService := NewCmdDebugService(newClient)

	cmdInitGateway := NewCmdInitGateway(newClient)
	cmdDownloadGateway := NewCmdDownloadGateway(newClient)
	cmdExportConfigGateway := NewCmdExportConfigGateway(newClient)
	cmdGenerateBundleGateway := NewCmdGenerateBundleGateway(newClient)
	cmdDeleteGateway := NewCmdDeleteGateway(newClient)
	cmdExposeGateway := NewCmdExposeGateway(newClient)
	cmdUnexposeGateway := NewCmdUnexposeGateway(newClient)
	cmdStatusGateway := NewCmdStatusGateway(newClient)
	cmdBindGateway := NewCmdBindGateway(newClient)
	cmdUnbindGateway := NewCmdUnbindGateway(newClient)
	cmdForwardGateway := NewCmdForwardGateway(newClient)
	cmdUnforwardGateway := NewCmdUnforwardGateway(newClient)

	// backwards compatibility commands hidden
	deprecatedMessage := "please use 'skupper service [bind|unbind]' instead"
	cmdBind.Hidden = true
	cmdBind.Deprecated = deprecatedMessage
	cmdUnbind.Deprecated = deprecatedMessage
	cmdUnbind.Hidden = true

	cmdListConnectors := NewCmdListConnectors(newClient) // listconnectors just keeped
	cmdListConnectors.Hidden = true
	cmdListConnectors.Deprecated = "please use 'skupper link status'"

	linkDeprecationMessage := "please use 'skupper link [create|delete|status]' instead."

	cmdConnect := NewCmdConnect(newClient)
	cmdConnect.Hidden = true
	cmdConnect.Deprecated = linkDeprecationMessage

	cmdDisconnect := NewCmdDisconnect(newClient)
	cmdDisconnect.Hidden = true
	cmdDisconnect.Deprecated = linkDeprecationMessage

	cmdCheckConnection := NewCmdCheckConnection(newClient)
	cmdCheckConnection.Hidden = true
	cmdCheckConnection.Deprecated = linkDeprecationMessage

	cmdConnectionToken := NewCmdConnectionToken(newClient)
	cmdConnectionToken.Hidden = true
	cmdConnectionToken.Deprecated = "please use 'skupper token create' instead."

	cmdListExposed.Hidden = true
	cmdListExposed.Deprecated = "please use 'skupper service status' instead."

	cmdDownloadGateway.Hidden = true
	cmdDownloadGateway.Deprecated = "please use 'skupper gateway export-config' instead."

	// setup subcommands
	cmdService := NewCmdService()
	cmdService.AddCommand(cmdCreateService)
	cmdService.AddCommand(cmdDeleteService)
	cmdService.AddCommand(NewCmdBind(newClient))
	cmdService.AddCommand(NewCmdUnbind(newClient))
	cmdService.AddCommand(cmdStatusService)
	cmdService.AddCommand(cmdLabelsService)

	cmdGateway := NewCmdGateway()
	cmdGateway.AddCommand(cmdInitGateway)
	cmdGateway.AddCommand(cmdDownloadGateway)
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

	cmdDebug := NewCmdDebug()
	cmdDebug.AddCommand(cmdDebugDump)
	cmdDebug.AddCommand(cmdDebugEvents)
	cmdDebug.AddCommand(cmdDebugService)

	cmdLink := NewCmdLink()
	cmdLink.AddCommand(NewCmdLinkCreate(newClient, ""))
	cmdLink.AddCommand(NewCmdLinkDelete(newClient))
	cmdLink.AddCommand(NewCmdLinkStatus(newClient))

	cmdToken := NewCmdToken()
	cmdToken.AddCommand(NewCmdTokenCreate(newClient, ""))

	cmdCompletion := NewCmdCompletion()

	cmdRevokeAll := NewCmdRevokeaccess(newClient)

	cmdNetwork := NewCmdNetwork()
	cmdNetwork.AddCommand(NewCmdNetworkStatus(newClient))

	rootCmd = &cobra.Command{Use: "skupper"}
	rootCmd.AddCommand(cmdInit,
		cmdDelete,
		cmdUpdate,
		cmdConnectionToken,
		cmdToken,
		cmdLink,
		cmdConnect,
		cmdDisconnect,
		cmdCheckConnection,
		cmdStatus,
		cmdListConnectors,
		cmdExpose,
		cmdUnexpose,
		cmdListExposed,
		cmdService,
		cmdBind,
		cmdUnbind,
		cmdVersion,
		cmdDebug,
		cmdCompletion,
		cmdGateway,
		cmdRevokeAll,
		cmdNetwork)

	rootCmd.PersistentFlags().StringVarP(&kubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
