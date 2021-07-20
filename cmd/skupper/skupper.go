package main

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"

	routev1 "github.com/openshift/api/route/v1"

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
	Protocol    string
	Address     string
	Port        int
	TargetPort  int
	Headless    bool
	ProxyTuning types.Tuning
}

func SkupperNotInstalledError(namespace string) error {
	return fmt.Errorf("Skupper is not installed in Namespace: '" + namespace + "`")

}

func parseTargetTypeAndName(args []string) (string, string) {
	//this functions assumes it is called with the right arguments, wrong
	//argument verification is done on the "Args:" functions
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
	return err
}

func expose(cli types.VanClientInterface, ctx context.Context, targetType string, targetName string, options ExposeOptions) (string, error) {
	serviceName := options.Address

	service, err := cli.ServiceInterfaceInspect(ctx, serviceName)
	if err != nil {
		return "", err
	}

	if service == nil {
		if options.Headless {
			if targetType != "statefulset" {
				return "", fmt.Errorf("The headless option is only supported for statefulsets")
			}
			service, err = cli.GetHeadlessServiceConfiguration(targetName, options.Protocol, options.Address, options.Port)
			if err != nil {
				return "", err
			}
			err = configureHeadlessProxy(service.Headless, &options.ProxyTuning)
			return service.Address, cli.ServiceInterfaceUpdate(ctx, service)
		} else {
			service = &types.ServiceInterface{
				Address:  serviceName,
				Port:     options.Port,
				Protocol: options.Protocol,
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
	err = cli.ServiceInterfaceBind(ctx, service, targetType, targetName, options.Protocol, options.TargetPort)
	if errors.IsNotFound(err) {
		return "", SkupperNotInstalledError(cli.GetNamespace())
	} else if err != nil {
		return "", fmt.Errorf("Unable to create skupper service: %w", err)
	}

	return options.Address, nil
}

func stringSliceContains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

var validExposeTargets = []string{"deployment", "statefulset", "pods", "service"}

func verifyTargetTypeFromArgs(args []string) error {
	targetType, _ := parseTargetTypeAndName(args)
	if !stringSliceContains(validExposeTargets, targetType) {
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
	if len(args) < 1 || (!strings.Contains(args[0], ":") && len(args) < 2) {
		return fmt.Errorf("Name and port must be specified")
	}
	if len(args) > 2 {
		return fmt.Errorf("illegal argument: %s", args[2])
	}
	if len(args) > 1 && strings.Contains(args[0], ":") {
		return fmt.Errorf("extra argument: %s", args[1])
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

func exposeProxyArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 2 || (!strings.Contains(args[1], ":") && len(args) < 3) {
		return fmt.Errorf("Proxy service address, target host and port must all be specified")
	}
	if len(args) > 3 {
		return fmt.Errorf("illegal argument: %s", args[3])
	}
	if len(args) > 2 && strings.Contains(args[1], ":") {
		return fmt.Errorf("extra argument: %s", args[2])
	}
	return nil
}

func bindProxyArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 3 || (!strings.Contains(args[2], ":") && len(args) < 4) {
		return fmt.Errorf("Proxy name, service address, target host and port must all be specified")
	}
	if len(args) > 4 {
		return fmt.Errorf("illegal argument: %s", args[4])
	}
	if len(args) > 3 && strings.Contains(args[2], ":") {
		return fmt.Errorf("extra argument: %s", args[3])
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
				fmt.Printf("%s. Please point to an existing, complete config file.\n", err.Error())
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
			//TODO: should cli allow init to diff ns?
			silenceCobra(cmd)
			ns := cli.GetNamespace()

			routerModeFlag := cmd.Flag("router-mode")
			edgeFlag := cmd.Flag("edge")
			if routerModeFlag.Changed && edgeFlag.Changed {
				return fmt.Errorf("You can not use the deprecated --edge, and --router-mode together, use --router-mode")
			}

			if routerModeFlag.Changed {
				options := []string{string(types.TransportModeInterior), string(types.TransportModeEdge)}
				if !stringSliceContains(options, routerMode) {
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
				if ClusterLocal { //this is redundant, because "if changed" it must be true, but it is also correct
					routerCreateOpts.Ingress = types.IngressNoneString
				}
			} else if !routerIngressFlag.Changed {
				routerCreateOpts.Ingress = cli.GetIngressDefault()
			}
			if routerCreateOpts.Ingress == types.IngressNodePortString && routerCreateOpts.IngressHost == "" && routerCreateOpts.Router.IngressHost == "" {
				return fmt.Errorf(`One of --ingress-host or --router-ingress-host option is required when using "--ingress nodeport"`)
			}
			routerCreateOpts.Annotations = asMap(annotations)
			routerCreateOpts.Labels = asMap(labels)
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
				if routerCreateOpts.Router.DebugMode != "valgrind" && routerCreateOpts.Router.DebugMode != "gdb" {
					return fmt.Errorf("Bad value for --router-debug-mode: %s (use 'valgrind' or 'gdb')", routerCreateOpts.Router.DebugMode)
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
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Ingress, "ingress", "", "", "Setup Skupper ingress to one of: [loadbalancer|route|nodeport|nginx-ingress-v1|none]. If not specified route is used when available, otherwise loadbalancer is used.")
	cmd.Flags().StringVarP(&routerCreateOpts.ConsoleIngress, "console-ingress", "", "", "Determines if/how console is exposed outside cluster. If not specified uses value of --ingress. One of: [loadbalancer|route|nodeport|nginx-ingress-v1|none].")
	cmd.Flags().StringVarP(&routerCreateOpts.IngressHost, "ingress-host", "", "", "Hostname by which the ingress proxy can be reached")
	cmd.Flags().StringVarP(&routerMode, "router-mode", "", string(types.TransportModeInterior), "Skupper router-mode")

	cmd.Flags().StringSliceVar(&annotations, "annotations", []string{}, "Annotations to add to skupper pods")
	cmd.Flags().StringSliceVar(&labels, "labels", []string{}, "Labels to add to skupper pods")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Participate in cross-site service synchronization")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmd.Flags().StringVarP(&routerLogging, "router-logging", "", "", "Logging settings for router (e.g. trace,debug,info,notice,warning,error)")
	cmd.Flags().StringVarP(&routerCreateOpts.Router.DebugMode, "router-debug-mode", "", "", "Enable debug mode for router ('valgrind' or 'gdb' are valid values)")

	cmd.Flags().StringVar(&routerCreateOpts.Router.Cpu, "router-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Memory, "router-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.NodeSelector, "router-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Affinity, "router-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.AntiAffinity, "router-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.IngressHost, "router-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")

	cmd.Flags().StringVar(&routerCreateOpts.Controller.Cpu, "controller-cpu", "", "CPU request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Memory, "controller-memory", "", "Memory request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.NodeSelector, "controller-node-selector", "", "Node selector to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Affinity, "controller-pod-affinity", "", "Pod affinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.AntiAffinity, "controller-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.IngressHost, "controller-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")

	cmd.Flags().BoolVarP(&ClusterLocal, "cluster-local", "", false, "Set up Skupper to only accept connections from within the local cluster.")
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
			err := cli.SiteConfigRemove(context.Background())
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
				fmt.Printf("Skupper is enabled for namespace %q%s%s.", ns, sitename, modedesc)
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

			//silence cobra may be moved below the "if" we want to print
			//the usage message along with this error
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

			addr, err := expose(cli, context.Background(), targetType, targetName, exposeOpts)
			if err == nil {
				fmt.Printf("%s %s exposed as %s\n", targetType, targetName, addr)
			}
			return err
		},
	}
	cmd.Flags().StringVar(&(exposeOpts.Protocol), "protocol", "tcp", "The protocol to proxy (tcp, http, or http2)")
	cmd.Flags().StringVar(&(exposeOpts.Address), "address", "", "The Skupper address to expose")
	cmd.Flags().IntVar(&(exposeOpts.Port), "port", 0, "The port to expose on")
	cmd.Flags().IntVar(&(exposeOpts.TargetPort), "target-port", 0, "The port to target on pods")
	cmd.Flags().BoolVar(&(exposeOpts.Headless), "headless", false, "Expose through a headless service (valid only for a statefulset target)")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Cpu, "proxy-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Memory, "proxy-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.NodeSelector, "proxy-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.Affinity, "proxy-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&exposeOpts.ProxyTuning.AntiAffinity, "proxy-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")

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
	cmd := &cobra.Command{
		Use:    "status",
		Short:  "List services exposed over the Skupper network",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			vsis, err := cli.ServiceInterfaceList(context.Background())
			if err == nil {
				if len(vsis) == 0 {
					fmt.Println("No services defined")
				} else {
					fmt.Println("Services exposed through Skupper:")
					for _, si := range vsis {
						if len(si.Targets) == 0 {
							fmt.Printf("    %s (%s port %d)", si.Address, si.Protocol, si.Port)
							fmt.Println()
						} else {
							fmt.Printf("    %s (%s port %d) with targets", si.Address, si.Protocol, si.Port)
							fmt.Println()
							for _, t := range si.Targets {
								var name string
								if t.Name != "" {
									name = fmt.Sprintf("name=%s", t.Name)
								}
								if t.Selector != "" {
									fmt.Printf("      => %s %s", t.Selector, name)
								} else if t.Service != "" {
									fmt.Printf("      => %s %s", t.Service, name)
								} else {
									fmt.Printf("      => %s (no selector)", name)
								}
								fmt.Println()
							}
						}
					}
				}
			} else {
				return fmt.Errorf("Could not retrieve services: %w", err)
			}
			return nil
		},
	}

	return cmd
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
		Use:    "create <name> <port>",
		Short:  "Create a skupper service",
		Args:   createServiceArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			var sPort string
			if len(args) == 1 {
				parts := strings.Split(args[0], ":")
				serviceToCreate.Address = parts[0]
				sPort = parts[1]
			} else {
				serviceToCreate.Address = args[0]
				sPort = args[1]
			}
			servicePort, err := strconv.Atoi(sPort)
			if err != nil {
				return fmt.Errorf("%s is not a valid port", sPort)
			} else {
				serviceToCreate.Port = servicePort
				err = cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&serviceToCreate.Protocol, "mapping", "tcp", "The mapping in use for this service address (currently one of tcp or http)")
	cmd.Flags().StringVar(&serviceToCreate.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&serviceToCreate.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")

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

var targetPort int
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
				} else {
					err = cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, protocol, targetPort)
					if err != nil {
						return fmt.Errorf("%w", err)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "", "The protocol to proxy (tcp, http or http2).")
	cmd.Flags().IntVar(&targetPort, "target-port", 0, "The port the target is listening on.")

	return cmd
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

func NewCmdProxy() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy init or proxy delete <proxy-name>",
		Short: "Manage skupper proxy definitions",
	}
	return cmd
}

var proxyInitOptions types.ProxyInitOptions

func NewCmdInitProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Short:  "Initialize a proxy to link to the skupper network",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			name, err := cli.ProxyInit(context.Background(), proxyInitOptions)
			if err != nil {
				return fmt.Errorf("%w", err)
			} else {
				fmt.Printf("Skupper proxy %s created\n", name)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&proxyInitOptions.Name, "name", "", "The name of proxy definition")
	cmd.Flags().BoolVarP(&proxyInitOptions.StartProxy, "start-proxy", "", true, "Start local proxy instance")
	return cmd
}

func NewCmdDeleteProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "delete <name>",
		Short:  "Remove the proxy definition and stop local instance if running",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.ProxyRemove(context.Background(), args[0])
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}

	return cmd
}

func NewCmdDownloadProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "download <name> <output-path>",
		Short:  "Download a proxy definition",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.ProxyDownload(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}

	return cmd
}

var proxyExposeOptions types.ProxyExposeOptions

func NewCmdExposeProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "expose <address> <host> <port>",
		Short:  "Expose a service process via proxy through a skupper address",
		Args:   exposeProxyArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			if len(args) == 2 {
				parts := strings.Split(args[1], ":")
				proxyExposeOptions.Egress.Host = parts[0]
				proxyExposeOptions.Egress.Port = parts[1]
			} else {
				proxyExposeOptions.Egress.Host = args[1]
				proxyExposeOptions.Egress.Port = args[2]
			}
			proxyExposeOptions.Egress.Address = args[0]
			proxyExposeOptions.Egress.ErrIfNoSvc = false

			name, err := cli.ProxyExpose(context.Background(), proxyExposeOptions)
			if err != nil {
				return fmt.Errorf("%w", err)
			} else {
				fmt.Printf("Skupper proxy %s created\n", name)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&proxyExposeOptions.Egress.Protocol, "protocol", "tcp", "The protocol to proxy (tcp, http or http2).")
	cmd.Flags().StringVar(&proxyExposeOptions.ProxyName, "name", "", "The name of external service to create. Defaults to service address value")
	return cmd
}

func NewCmdUnexposeProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unexpose <name> <address>",
		Short:  "Unexpose a service process previously exposed via proxy through a skupper address",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.ProxyUnexpose(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}

	return cmd
}

var proxyBindOptions types.ProxyBindOptions

func NewCmdBindProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bind <proxy-name> <address> <host> <port>",
		Short:  "Bind a service process via proxy to a skupper service",
		Args:   bindProxyArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			if len(args) == 3 {
				parts := strings.Split(args[2], ":")
				proxyBindOptions.Host = parts[0]
				proxyBindOptions.Port = parts[1]
			} else {
				proxyBindOptions.Host = args[2]
				proxyBindOptions.Port = args[3]
			}
			proxyBindOptions.Address = args[1]
			proxyBindOptions.ErrIfNoSvc = true

			err := cli.ProxyBind(context.Background(), args[0], proxyBindOptions)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&proxyBindOptions.Protocol, "protocol", "tcp", "The protocol to proxy (tcp, http or http2).")
	return cmd
}

func NewCmdUnbindProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unbind <proxy-name> <address>",
		Short:  "Unbind the service process from the skupper network",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.ProxyUnbind(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "tcp", "The protocol to proxy (tcp, http or http2).")
	return cmd
}

func NewCmdStatusProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status <proxy-name>",
		Short:  "Report the status of a proxy for the current skupper site",
		Args:   cobra.MaximumNArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			if len(args) == 1 && args[0] != "all" {
				proxyName := args[0]
				inspect, err := cli.ProxyInspect(context.Background(), proxyName)
				if err != nil {
					return fmt.Errorf("%w", err)
				}

				fmt.Printf("%-30s %s\n", "Name", inspect.ProxyName)
				fmt.Printf("%-30s %s\n", "Version", strings.TrimSuffix(inspect.ProxyVersion, "\n"))
				fmt.Printf("%-30s %s\n", "URL", inspect.ProxyUrl)

				fmt.Println("")

				if len(inspect.TcpConnectors) == 0 && len(inspect.TcpListeners) == 0 {
					fmt.Println("No Services Defined")
				} else {
					fmt.Println("Service Definitions:")
					tw := new(tabwriter.Writer)
					tw.Init(os.Stdout, 0, 4, 1, ' ', 0)
					fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t", "TYPE", "SERVICE", "ADDRESS", "HOST", "PORT", "FORWARD_PORT"))
					for _, connector := range inspect.TcpConnectors {
						fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t", "bind", strings.TrimPrefix(connector.Name, proxyName+"-egress-"), connector.Address, connector.Host, connector.Port, ""))
					}
					for _, listener := range inspect.TcpListeners {
						fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t", "forward", strings.TrimPrefix(listener.Name, proxyName+"-ingress-"), listener.Address, listener.Host, listener.Port, listener.LocalPort))
					}
					tw.Flush()
				}
			} else {
				proxies, err := cli.ProxyList(context.Background())
				if err != nil {
					return fmt.Errorf("%w", err)
				}

				if len(proxies) == 0 {
					fmt.Println("No proxy definitions found")
					return nil
				}

				fmt.Println("Proxy Definitions Summary")
				fmt.Println("")
				tw := new(tabwriter.Writer)
				tw.Init(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t", "NAME", "BINDS", "FORWARDS", "URL"))
				for _, proxy := range proxies {
					fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t", proxy.ProxyName, strconv.Itoa(len(proxy.TcpConnectors)), strconv.Itoa(len(proxy.TcpListeners)), proxy.ProxyUrl))
				}
				tw.Flush()
			}

			return nil
		},
	}

	return cmd
}

var proxyForwardService types.ServiceInterface
var loopback bool

func NewCmdForwardProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "forward <proxy-name> <address> <port>",
		Short:  "Forward a service address via proxy to the skupper network",
		Args:   cobra.ExactArgs(3),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			forwardPort, err := strconv.Atoi(args[2])
			if err != nil {
				return fmt.Errorf("%s is not a valid forward port", args[2])
			}

			proxyForwardService.Address = args[1]
			proxyForwardService.Port = forwardPort

			err = cli.ProxyForward(context.Background(), args[0], loopback, &proxyForwardService)
			//			err = cli.ProxyForward(context.Background(), args[0], lPort, loopback, &proxyForwardService)
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&proxyForwardService.Protocol, "mapping", "tcp", "The mapping in use for this service address (currently one of tcp or http)")
	cmd.Flags().StringVar(&proxyForwardService.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmd.Flags().BoolVar(&proxyForwardService.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmd.Flags().BoolVarP(&loopback, "loopback", "", false, "Forward from loopback only")
	return cmd
}

func NewCmdUnforwardProxy(newClient cobraFunc) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unforward <proxy-name> <address>",
		Short:  "Stop forwarding a service address via proxy to the skupper network",
		Args:   cobra.ExactArgs(2),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := cli.ProxyUnforward(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&protocol, "protocol", "tcp", "The protocol to proxy (tcp, http or http2).")
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
		Use:   "debug dump <file> or debug action <tbd>",
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
	cmdBind := NewCmdBind(newClient)
	cmdUnbind := NewCmdUnbind(newClient)
	cmdVersion := NewCmdVersion(newClientSansExit)
	cmdDebugDump := NewCmdDebugDump(newClient)

	cmdInitProxy := NewCmdInitProxy(newClient)
	cmdDownloadProxy := NewCmdDownloadProxy(newClient)
	cmdDeleteProxy := NewCmdDeleteProxy(newClient)
	cmdExposeProxy := NewCmdExposeProxy(newClient)
	cmdUnexposeProxy := NewCmdUnexposeProxy(newClient)
	cmdStatusProxy := NewCmdStatusProxy(newClient)
	cmdBindProxy := NewCmdBindProxy(newClient)
	cmdUnbindProxy := NewCmdUnbindProxy(newClient)
	cmdForwardProxy := NewCmdForwardProxy(newClient)
	cmdUnforwardProxy := NewCmdUnforwardProxy(newClient)

	//backwards compatibility commands hidden
	deprecatedMessage := "please use 'skupper service [bind|unbind]' instead"
	cmdBind.Hidden = true
	cmdBind.Deprecated = deprecatedMessage
	cmdUnbind.Deprecated = deprecatedMessage
	cmdUnbind.Hidden = true

	cmdListConnectors := NewCmdListConnectors(newClient) //listconnectors just keeped
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

	// setup subcommands
	cmdService := NewCmdService()
	cmdService.AddCommand(cmdCreateService)
	cmdService.AddCommand(cmdDeleteService)
	cmdService.AddCommand(NewCmdBind(newClient))
	cmdService.AddCommand(NewCmdUnbind(newClient))
	cmdService.AddCommand(cmdStatusService)

	cmdProxy := NewCmdProxy()
	cmdProxy.AddCommand(cmdInitProxy)
	cmdProxy.AddCommand(cmdDownloadProxy)
	cmdProxy.AddCommand(cmdDeleteProxy)
	cmdProxy.AddCommand(cmdExposeProxy)
	cmdProxy.AddCommand(cmdUnexposeProxy)
	cmdProxy.AddCommand(cmdStatusProxy)
	cmdProxy.AddCommand(cmdBindProxy)
	cmdProxy.AddCommand(cmdUnbindProxy)
	cmdProxy.AddCommand(cmdForwardProxy)
	cmdProxy.AddCommand(cmdUnforwardProxy)

	cmdDebug := NewCmdDebug()
	cmdDebug.AddCommand(cmdDebugDump)

	cmdLink := NewCmdLink()
	cmdLink.AddCommand(NewCmdLinkCreate(newClient, ""))
	cmdLink.AddCommand(NewCmdLinkDelete(newClient))
	cmdLink.AddCommand(NewCmdLinkStatus(newClient))

	cmdToken := NewCmdToken()
	cmdToken.AddCommand(NewCmdTokenCreate(newClient, ""))

	cmdCompletion := NewCmdCompletion()

	cmdRevokeAll := NewCmdRevokeaccess(newClient)

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
		cmdProxy,
		cmdRevokeAll)

	rootCmd.PersistentFlags().StringVarP(&kubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
