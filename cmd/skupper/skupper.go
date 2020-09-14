package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
)

var version = "undefined"

type ExposeOptions struct {
	Protocol   string
	Address    string
	Port       int
	TargetPort int
	Headless   bool
}

func expose(cli types.VanClientInterface, ctx context.Context, targetType string, targetName string, options ExposeOptions) error {
	serviceName := options.Address
	if serviceName == "" {
		if targetType == "service" {
			return fmt.Errorf("The --address option is required for target type 'service'")
		} else {
			serviceName = targetName
		}
	}
	service, err := cli.ServiceInterfaceInspect(ctx, serviceName)
	if err != nil {
		return err
	}

	if service == nil {
		if options.Headless {
			if targetType != "statefulset" {
				return fmt.Errorf("The headless option is only supported for statefulsets")
			}
			service, err = cli.GetHeadlessServiceConfiguration(targetName, options.Protocol, options.Address, options.Port)
			if err != nil {
				return err
			}
			return cli.ServiceInterfaceUpdate(ctx, service)
		} else {
			service = &types.ServiceInterface{
				Address:  serviceName,
				Port:     options.Port,
				Protocol: options.Protocol,
			}
		}
	} else if service.Headless != nil {
		return fmt.Errorf("Service already exposed as headless")
	} else if options.Headless {
		return fmt.Errorf("Service already exposed, cannot reconfigure as headless")
	} else if options.Protocol != "" && service.Protocol != options.Protocol {
		return fmt.Errorf("Invalid protocol %s for service with mapping %s", options.Protocol, service.Protocol)
	}

	// service may exist from remote origin
	service.Origin = ""
	return cli.ServiceInterfaceBind(ctx, service, targetType, targetName, options.Protocol, options.TargetPort)
}

func requiredArg(name string) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("%s must be specified", name)
		}
		if len(args) > 1 {
			return fmt.Errorf("illegal argument: %s", args[1])
		}
		return nil
	}
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
	targetType := args[0]
	if strings.Contains(args[0], "/") {
		parts := strings.Split(args[0], "/")
		targetType = parts[0]
	}
	if !stringSliceContains(validExposeTargets, targetType) {
		return fmt.Errorf("expose target type must be one of: [%s]", strings.Join(validExposeTargets, ", "))
	}
	return nil
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
	return nil
}

func check(err error) bool {
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
		return false
	} else {
		return true
	}
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

func NewCmdInit(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long: `Setup a router and other supporting objects to provide a functional skupper
installation that can then be connected to other skupper installations`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			//TODO: should cli allow init to diff ns?
			ns := cli.GetNamespace()
			routerCreateOpts.SkupperNamespace = ns
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if check(err) {
				if siteConfig == nil {
					siteConfig, err = cli.SiteConfigCreate(context.Background(), routerCreateOpts)
				}

				if check(err) {
					err = cli.RouterCreate(context.Background(), *siteConfig)
					if check(err) {
						fmt.Println("Skupper is now installed in namespace '" + ns + "'.  Use 'skupper status' to get more information.")
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&routerCreateOpts.SkupperName, "site-name", "", "", "Provide a specific name for this skupper installation")
	cmd.Flags().BoolVarP(&routerCreateOpts.IsEdge, "edge", "", false, "Configure as an edge")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableController, "enable-proxy-controller", "", true, "Setup the proxy controller as well as the router")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Configure proxy controller to particiapte in service sync (not relevant if --enable-proxy-controller is false)")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().BoolVarP(&routerCreateOpts.ClusterLocal, "cluster-local", "", false, "Set up skupper to only accept connections from within the local cluster.")

	return cmd
}

func NewCmdDelete(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete skupper installation",
		Long:  `delete will delete any skupper related objects from the namespace`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

var clientIdentity string

func NewCmdConnectionToken(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connection-token <output-file>",
		Short: "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
		Args:  requiredArg("output-file"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
			if err != nil {
				return fmt.Errorf("Failed to create connection token: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&clientIdentity, "client-identity", "i", types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")

	return cmd
}

var connectorCreateOpts types.ConnectorCreateOptions

func NewCmdConnect(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect <connection-token-file>",
		Short: "Connect this skupper installation to that which issued the specified connectionToken",
		Args:  requiredArg("connection-token"),
		RunE: func(cmd *cobra.Command, args []string) error {
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if err != nil {
				fmt.Println("Unable to retrieve site config: ", err.Error())
				os.Exit(1)
			} else if siteConfig == nil || !siteConfig.Spec.SiteControlled {
				connectorCreateOpts.SkupperNamespace = cli.GetNamespace()
				secret, err := cli.ConnectorCreateFromFile(context.Background(), args[0], connectorCreateOpts)
				if err != nil {
					return fmt.Errorf("Failed to create connection: %w", err)
				} else {
					if siteConfig.Spec.IsEdge {
						fmt.Printf("Skupper configured to connect to %s:%s (name=%s)\n",
							secret.ObjectMeta.Annotations["edge-host"],
							secret.ObjectMeta.Annotations["edge-port"],
							secret.ObjectMeta.Name)
					} else {
						fmt.Printf("Skupper configured to connect to %s:%s (name=%s)\n",
							secret.ObjectMeta.Annotations["inter-router-host"],
							secret.ObjectMeta.Annotations["inter-router-port"],
							secret.ObjectMeta.Name)
					}
				}
			} else {
				// create the secret, site-controller will do the rest
				secret, err := cli.ConnectorCreateSecretFromFile(context.Background(), args[0], connectorCreateOpts)
				if err != nil {
					return fmt.Errorf("Failed to create connection: %w", err)
				} else {
					if siteConfig.Spec.IsEdge {
						fmt.Printf("Skupper site-controller configured to connect to %s:%s (name=%s)\n",
							secret.ObjectMeta.Annotations["edge-host"],
							secret.ObjectMeta.Annotations["edge-port"],
							secret.ObjectMeta.Name)
					} else {
						fmt.Printf("Skupper site-controller configured to connect to %s:%s (name=%s)\n",
							secret.ObjectMeta.Annotations["inter-router-host"],
							secret.ObjectMeta.Annotations["inter-router-port"],
							secret.ObjectMeta.Name)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&connectorCreateOpts.Name, "connection-name", "", "", "Provide a specific name for the connection (used when removing it with disconnect)")
	cmd.Flags().Int32VarP(&connectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this connection.")

	return cmd
}

var connectorRemoveOpts types.ConnectorRemoveOptions

func NewCmdDisconnect(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disconnect <name>",
		Short: "Remove specified connection",
		Args:  requiredArg("connection name"),
		RunE: func(cmd *cobra.Command, args []string) error {
			connectorRemoveOpts.Name = args[0]
			connectorRemoveOpts.SkupperNamespace = cli.GetNamespace()
			connectorRemoveOpts.ForceCurrent = false
			err := cli.ConnectorRemove(context.Background(), connectorRemoveOpts)
			if err == nil {
				fmt.Println("Connection '" + args[0] + "' has been removed")
			} else {
				return fmt.Errorf("Failed to remove connection: %w", err)
			}
			return nil
		},
	}

	return cmd
}

func NewCmdListConnectors(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-connectors",
		Short: "List configured outgoing connections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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
				return fmt.Errorf("Skupper is not installed in '" + cli.GetNamespace() + "`")
			} else {
				return fmt.Errorf("Unable to retrieve connections: %w", err)
			}
			return nil
		},
	}
	return cmd
}

var waitFor int

func NewCmdCheckConnection(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-connection all|<connection-name>",
		Short: "Check whether a connection to another Skupper site is active",
		Args:  requiredArg("connection name"),
		RunE: func(cmd *cobra.Command, args []string) error {
			var connectors []*types.ConnectorInspectResponse
			connected := 0

			if args[0] == "all" {
				vcis, err := cli.ConnectorList(context.Background())
				if err == nil {
					for _, vci := range vcis {
						connectors = append(connectors, &types.ConnectorInspectResponse{
							Connector: vci,
							Connected: false,
						})
					}
				}
			} else {
				vci, err := cli.ConnectorInspect(context.Background(), args[0])
				if err == nil {
					connectors = append(connectors, vci)
					if vci.Connected {
						connected++
					}
				}
			}

			for i := 0; connected < len(connectors) && i < waitFor; i++ {
				for _, c := range connectors {
					vci, err := cli.ConnectorInspect(context.Background(), c.Connector.Name)
					if err == nil && vci.Connected && c.Connected == false {
						c.Connected = true
						connected++
					}
				}
				time.Sleep(time.Second)
			}

			if len(connectors) == 0 {
				fmt.Println("There are no connectors configured or active")
			} else {
				for _, c := range connectors {
					if c.Connected {
						fmt.Printf("Connection for %s is active", c.Connector.Name)
						fmt.Println()
					} else {
						fmt.Printf("Connection for %s not active", c.Connector.Name)
						fmt.Println()
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&waitFor, "wait", 1, "The number of seconds to wait for connections to become active")

	return cmd

}

func NewCmdStatus(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Report the status of the current Skupper site",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vir, err := cli.RouterInspect(context.Background())
			if err == nil {
				ns := cli.GetNamespace()
				var modedesc string = " in interior mode"
				if vir.Status.Mode == types.TransportModeEdge {
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
					if check(err) {
						if siteConfig.Spec.AuthMode == "internal" {
							fmt.Println("The credentials for internal console-auth mode are held in secret: 'skupper-users'")
						}
					}
				}
			} else {
				return fmt.Errorf("Unable to retrieve skupper status: %w", err)
			}
			return nil
		},
	}
	return cmd
}

var exposeOpts ExposeOptions

func NewCmdExpose(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short: "Expose a set of pods through a Skupper address",
		Args:  exposeTargetArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetType := args[0]
			var targetName string
			if len(args) == 2 {
				targetName = args[1]
			} else {
				parts := strings.Split(args[0], "/")
				targetType = parts[0]
				targetName = parts[1]
			}

			err := expose(cli, context.Background(), targetType, targetName, exposeOpts)

			if err == nil {
				address := exposeOpts.Address
				if address == "" {
					if args[0] == "service" {
						return fmt.Errorf("--address option is required for target type 'service'")
					} else {
						address = targetType
					}
				}
				fmt.Printf("%s %s exposed as %s\n", targetType, targetName, address)
			} else if errors.IsNotFound(err) {
				return fmt.Errorf("Skupper is not installed in '" + cli.GetNamespace() + "`")
			} else {
				return fmt.Errorf("Unable to create skupper service: %w", err)
			}
			return nil
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

func NewCmdUnexpose(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unexpose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short: "Unexpose a set of pods previously exposed through a Skupper address",
		Args:  exposeTargetArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetType := args[0]
			var targetName string
			if len(args) == 2 {
				targetName = args[1]
			} else {
				parts := strings.Split(args[0], "/")
				targetType = parts[0]
				targetName = parts[1]
			}
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

func NewCmdListExposed(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-exposed",
		Short: "List services exposed over the Skupper network",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

func NewCmdCreateService(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create <name> <port>",
		Short: "Create a skupper service",
		Args:  createServiceArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

func NewCmdDeleteService(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a skupper service",
		Args:  requiredArg("service-name"),
		RunE: func(cmd *cobra.Command, args []string) error {
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

func NewCmdBind(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bind <service-name> <target-type> <target-name>",
		Short: "Bind a target to a service",
		Args:  bindArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if protocol != "" && protocol != "tcp" && protocol != "http" && protocol != "http2" {
				return fmt.Errorf("%s is not a valid protocol. Choose 'tcp', 'http' or 'http2'.", protocol)
			} else {
				var targetType string
				var targetName string
				if len(args) == 2 {
					parts := strings.Split(args[1], "/")
					targetType = parts[0]
					targetName = parts[1]
				} else if len(args) == 3 {
					targetType = args[1]
					targetName = args[2]
				}
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

func NewCmdUnbind(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unbind <service-name> <target-type> <target-name>",
		Short: "Unbind a target from a service",
		Args:  bindArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var targetType string
			var targetName string
			if len(args) == 2 {
				parts := strings.Split(args[1], "/")
				targetType = parts[0]
				targetName = parts[1]
			} else if len(args) == 3 {
				targetType = args[1]
				targetName = args[2]
			}
			err := cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			return nil
		},
	}
	return cmd
}

func NewCmdVersion(cli types.VanClientInterface) *cobra.Command {
	// TODO: change to inspect
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Report the version of the Skupper CLI and services",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			vir, err := cli.RouterInspect(context.Background())
			fmt.Printf("%-30s %s\n", "client version", version)
			if err == nil {
				fmt.Printf("%-30s %s\n", "transport version", vir.TransportVersion)
				fmt.Printf("%-30s %s\n", "controller version", vir.ControllerVersion)
			} else {
				return fmt.Errorf("Unable to retrieve skupper component versions: %w", err)
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

func NewCmdDebugDump(cli types.VanClientInterface) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dump <filename>",
		Short: "Collect and save skupper logs, config, etc.",
		Args:  requiredArg("save file"),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cli.SkupperDump(context.Background(), args[0], version, kubeconfig, kubeContext)
			if err != nil {
				return fmt.Errorf("Unable to save skupper details: %w", err)
			}
			return nil
		},
	}
	return cmd
}

var kubeContext string
var namespace string
var kubeconfig string
var rootCmd *cobra.Command

func init() {
	routev1.AddToScheme(scheme.Scheme)

	cli := NewClient(namespace, kubeContext, kubeconfig)

	cmdInit := NewCmdInit(cli)
	cmdDelete := NewCmdDelete(cli)
	cmdConnectionToken := NewCmdConnectionToken(cli)
	cmdConnect := NewCmdConnect(cli)
	cmdDisconnect := NewCmdDisconnect(cli)
	cmdListConnectors := NewCmdListConnectors(cli)
	cmdCheckConnection := NewCmdCheckConnection(cli)
	cmdStatus := NewCmdStatus(cli)
	cmdExpose := NewCmdExpose(cli)
	cmdUnexpose := NewCmdUnexpose(cli)
	cmdListExposed := NewCmdListExposed(cli)
	cmdCreateService := NewCmdCreateService(cli)
	cmdDeleteService := NewCmdDeleteService(cli)
	cmdBind := NewCmdBind(cli)
	cmdUnbind := NewCmdUnbind(cli)
	cmdVersion := NewCmdVersion(cli)
	cmdDebugDump := NewCmdDebugDump(cli)

	// setup subcommands
	cmdService := NewCmdService()
	cmdService.AddCommand(cmdCreateService)
	cmdService.AddCommand(cmdDeleteService)

	cmdDebug := NewCmdDebug()
	cmdDebug.AddCommand(cmdDebugDump)

	completionLong := `
Output shell completion code for bash.
The shell code must be evaluated to provide interactive
completion of skupper commands.  This can be done by sourcing it from
the .bash_profile. i.e.: $ source <(skupper completion)
`
	var cmdCompletion = &cobra.Command{
		Use:   "completion",
		Short: "Output shell completion code for bash",
		Long:  completionLong,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			rootCmd.GenBashCompletion(os.Stdout)

		},
	}

	rootCmd = &cobra.Command{Use: "skupper"}
	rootCmd.Version = version
	rootCmd.AddCommand(cmdInit, cmdDelete, cmdConnectionToken, cmdConnect, cmdDisconnect, cmdCheckConnection, cmdStatus, cmdListConnectors, cmdExpose, cmdUnexpose, cmdListExposed,
		cmdService, cmdBind, cmdUnbind, cmdVersion, cmdDebug, cmdCompletion)
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")

}

func main() {
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
