package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	routev1 "github.com/openshift/api/route/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
)

type ExposeOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
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

func silenceCobra(cmd *cobra.Command) {
	cmd.SilenceUsage = true
}

func NewClient(namespace string, context string, kubeConfigPath string) *client.VanClient {
	cli, err := client.NewClient(namespace, context, kubeConfigPath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
	return cli
}

var routerCreateOpts types.SiteConfigSpec
var routerLogging string

// TODO unit-test me
func inStringSlice(options []string, value string) bool {
	l := options[:] //do a copy to avoid modifying the original list
	sort.Sort(sort.StringSlice(l))

	// from library doc:
	// SearchStrings searches for x in a sorted slice of strings and returns the index
	// as specified by Search. The return value is the index to insert x if x is not
	// present (it could be len(a)).
	// The slice must be sorted in ascending order.
	//
	position := sort.SearchStrings(l, value)
	if position == len(l) || (l[position] != value) {
		return false
	}
	return true
}

var ClusterLocal bool

func NewCmdInit(newClient cobraFunc) *cobra.Command {
	var routerMode string
	annotations := []string{}
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
				if !inStringSlice(options, routerMode) {
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
			for _, a := range annotations {
				parts := strings.Split(a, "=")
				if routerCreateOpts.Annotations == nil {
					routerCreateOpts.Annotations = map[string]string{}
				}
				if len(parts) > 1 {
					routerCreateOpts.Annotations[parts[0]] = parts[1]
				} else {
					routerCreateOpts.Annotations[parts[0]] = ""
				}
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
				routerCreateOpts.RouterLogging = logConfig
			}
			if routerCreateOpts.RouterDebugMode != "" {
				if routerCreateOpts.RouterDebugMode != "valgrind" && routerCreateOpts.RouterDebugMode != "gdb" {
					return fmt.Errorf("Bad value for --router-debug-mode: %s (use 'valgrind' or 'gdb')", routerCreateOpts.RouterDebugMode)
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
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Participate in cross-site service synchronization")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmd.Flags().StringVarP(&routerLogging, "router-logging", "", "", "Logging settings for router (e.g. trace,debug,info,notice,warning,error)")
	cmd.Flags().StringVarP(&routerCreateOpts.RouterDebugMode, "router-debug-mode", "", "", "Enable debug mode for router ('valgrind' or 'gdb' are valid values)")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", true, "Enable skupper console")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringSliceVar(&annotations, "annotations", []string{}, "Annotations to add to skupper deployments")

	cmd.Flags().BoolVarP(&ClusterLocal, "cluster-local", "", false, "Set up Skupper to only accept connections from within the local cluster.")
	f := cmd.Flag("cluster-local")
	f.Deprecated = "This flag is deprecated, use --ingress [loadbalancer|route|none]"
	f.Hidden = true
	cmd.Flags().StringVarP(&routerCreateOpts.Ingress, "ingress", "", "loadbalancer", "Setup Skupper ingress to one of: [loadbalancer|route|none].")

	cmd.Flags().BoolVarP(&isEdge, "edge", "", false, "Configure as an edge")
	f = cmd.Flag("edge")
	f.Deprecated = "This flag is deprecated, use --router-mode [interior|edge]"
	f.Hidden = true
	cmd.Flags().StringVarP(&routerMode, "router-mode", "", string(types.TransportModeInterior), "Skupper router-mode")

	return cmd
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
						fmt.Printf("    %s:%s (name=%s)", c.Host, c.Port, c.Name)
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

func NewCmdVersion(newClient cobraFunc) *cobra.Command {
	// TODO: change to inspect
	cmd := &cobra.Command{
		Use:    "version",
		Short:  "Report the version of the Skupper CLI and services",
		Args:   cobra.NoArgs,
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			fmt.Printf("%-30s %s\n", "client version", client.Version)
			fmt.Printf("%-30s %s\n", "transport version", cli.GetVersion(types.TransportComponentName, types.TransportContainerName))
			fmt.Printf("%-30s %s\n", "controller version", cli.GetVersion(types.ControllerComponentName, types.ControllerContainerName))

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
		Use:    "dump <filename>",
		Short:  "Collect and save skupper logs, config, etc.",
		Args:   cobra.ExactArgs(1),
		PreRun: newClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)
			err := cli.SkupperDump(context.Background(), args[0], client.Version, kubeConfigPath, kubeContext)
			if err != nil {
				return fmt.Errorf("Unable to save skupper details: %w", err)
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
	cmdVersion := NewCmdVersion(newClient)
	cmdDebugDump := NewCmdDebugDump(newClient)

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

	cmdDebug := NewCmdDebug()
	cmdDebug.AddCommand(cmdDebugDump)

	cmdLink := NewCmdLink()
	cmdLink.AddCommand(NewCmdLinkCreate(newClient, ""))
	cmdLink.AddCommand(NewCmdLinkDelete(newClient))
	cmdLink.AddCommand(NewCmdLinkStatus(newClient))

	cmdToken := NewCmdToken()
	cmdToken.AddCommand(NewCmdTokenCreate(newClient, ""))

	cmdCompletion := NewCmdCompletion()

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
		cmdCompletion)

	rootCmd.PersistentFlags().StringVarP(&kubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")

}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
