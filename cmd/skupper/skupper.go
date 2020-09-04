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

func expose(cli *client.VanClient, ctx context.Context, targetType string, targetName string, options ExposeOptions) error {
	serviceName := options.Address
	if serviceName == "" {
		if targetType == "service" {
			return fmt.Errorf("The --address option is required for target type 'service'")
		} else {
			serviceName = targetName
		}
	}
	service, err := cli.ServiceInterfaceInspect(ctx, serviceName)
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

var rootCmd *cobra.Command

func init() {
	routev1.AddToScheme(scheme.Scheme)

	var kubeContext string
	var namespace string
	var kubeconfig string

	var routerCreateOpts types.SiteConfigSpec
	var cmdInit = &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long:  `init will setup a router and other supporting objects to provide a functional skupper installation that can then be connected to other skupper installations`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			//TODO: should cli allow init to diff ns?
			routerCreateOpts.SkupperNamespace = cli.Namespace
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if check(err) {
				if siteConfig == nil {
					siteConfig, err = cli.SiteConfigCreate(context.Background(), routerCreateOpts)
				}

				if check(err) {
					err = cli.RouterCreate(context.Background(), *siteConfig)
					if check(err) {
						fmt.Println("Skupper is now installed in namespace '" + cli.Namespace + "'.  Use 'skupper status' to get more information.")
					}
				}
			}
		},
	}
	cmdInit.Flags().StringVarP(&routerCreateOpts.SkupperName, "site-name", "", "", "Provide a specific name for this skupper installation")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.IsEdge, "edge", "", false, "Configure as an edge")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.EnableController, "enable-proxy-controller", "", true, "Setup the proxy controller as well as the router")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Configure proxy controller to particiapte in service sync (not relevant if --enable-proxy-controller is false)")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console")
	cmdInit.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmdInit.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmdInit.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmdInit.Flags().BoolVarP(&routerCreateOpts.ClusterLocal, "cluster-local", "", false, "Set up skupper to only accept connections from within the local cluster.")

	var cmdDelete = &cobra.Command{
		Use:   "delete",
		Short: "Delete skupper installation",
		Long:  `delete will delete any skupper related objects from the namespace`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			err := cli.SiteConfigRemove(context.Background())
			if err != nil {
				err = cli.RouterRemove(context.Background())
			}
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			} else {
				fmt.Println("Skupper is now removed from '" + cli.Namespace + "'.")
			}
		},
	}

	var clientIdentity string
	var cmdConnectionToken = &cobra.Command{
		Use:   "connection-token <output-file>",
		Short: "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
		Args:  requiredArg("output-file"),
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
			if err != nil {
				fmt.Println("Failed to create connection token: ", err.Error())
				os.Exit(1)
			}
		},
	}
	cmdConnectionToken.Flags().StringVarP(&clientIdentity, "client-identity", "i", types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")

	var connectorCreateOpts types.ConnectorCreateOptions
	var cmdConnect = &cobra.Command{
		Use:   "connect <connection-token-file>",
		Short: "Connect this skupper installation to that which issued the specified connectionToken",
		Args:  requiredArg("connection-token"),
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if err != nil {
				fmt.Println("Error, unable to retrieve site config: ", err.Error())
				os.Exit(1)
			} else if siteConfig == nil || !siteConfig.Spec.SiteControlled {
				connectorCreateOpts.SkupperNamespace = cli.Namespace
				secret, err := cli.ConnectorCreateFromFile(context.Background(), args[0], connectorCreateOpts)
				if err != nil {
					fmt.Println("Failed to create connection: ", err.Error())
					os.Exit(1)
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
					fmt.Println("Failed to create connection: ", err.Error())
					os.Exit(1)
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
		},
	}
	cmdConnect.Flags().StringVarP(&connectorCreateOpts.Name, "connection-name", "", "", "Provide a specific name for the connection (used when removing it with disconnect)")
	cmdConnect.Flags().Int32VarP(&connectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this connection.")

	var connectorRemoveOpts types.ConnectorRemoveOptions
	var cmdDisconnect = &cobra.Command{
		Use:   "disconnect <name>",
		Short: "Remove specified connection",
		Args:  requiredArg("connection name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			connectorRemoveOpts.Name = args[0]
			connectorRemoveOpts.SkupperNamespace = cli.Namespace
			connectorRemoveOpts.ForceCurrent = false
			err := cli.ConnectorRemove(context.Background(), connectorRemoveOpts)
			if err == nil {
				fmt.Println("Connection '" + args[0] + "' has been removed")
			} else {
				fmt.Println("Failed to remove connection: ", err.Error())
				os.Exit(1)
			}
		},
	}

	var cmdListConnectors = &cobra.Command{
		Use:   "list-connectors",
		Short: "List configured outgoing connections",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
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
				fmt.Println("Skupper is not installed in '" + cli.Namespace + "`")
				os.Exit(1)
			} else {
				fmt.Println("Error, unable to retrieve connections: ", err.Error())
				os.Exit(1)
			}
		},
	}

	var waitFor int
	var cmdCheckConnection = &cobra.Command{
		Use:   "check-connection all|<connection-name>",
		Short: "Check whether a connection to another Skupper site is active",
		Args:  requiredArg("connection name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
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
		},
	}
	cmdCheckConnection.Flags().IntVar(&waitFor, "wait", 1, "The number of seconds to wait for connections to become active")

	var cmdStatus = &cobra.Command{
		Use:   "status",
		Short: "Report the status of the current Skupper site",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			vir, err := cli.RouterInspect(context.Background())
			if err == nil {
				var modedesc string = " in interior mode"
				if vir.Status.Mode == types.TransportModeEdge {
					modedesc = " in edge mode"
				}
				sitename := ""
				if vir.Status.SiteName != "" && vir.Status.SiteName != cli.Namespace {
					sitename = fmt.Sprintf(" with site name %q", vir.Status.SiteName)
				}
				fmt.Printf("Skupper is enabled for namespace %q%s%s.", cli.Namespace, sitename, modedesc)
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
				fmt.Println("Unable to retrieve skupper status: ", err.Error())
				os.Exit(1)
			}
		},
	}

	exposeOpts := ExposeOptions{}
	var cmdExpose = &cobra.Command{
		Use:   "expose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short: "Expose a set of pods through a Skupper address",
		Args:  exposeTargetArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)

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
						fmt.Printf("--address option is required for target type 'service'")
						os.Exit(1)
					} else {
						address = targetType
					}
				}
				fmt.Printf("%s %s exposed as %s\n", targetType, targetName, address)
			} else if errors.IsNotFound(err) {
				fmt.Println("Skupper is not installed in '" + cli.Namespace + "`")
				os.Exit(1)
			} else {
				fmt.Println("Error, unable to create skupper service: ", err.Error())
				os.Exit(1)
			}
		},
	}
	cmdExpose.Flags().StringVar(&(exposeOpts.Protocol), "protocol", "tcp", "The protocol to proxy (tcp, http, or http2)")
	cmdExpose.Flags().StringVar(&(exposeOpts.Address), "address", "", "The Skupper address to expose")
	cmdExpose.Flags().IntVar(&(exposeOpts.Port), "port", 0, "The port to expose on")
	cmdExpose.Flags().IntVar(&(exposeOpts.TargetPort), "target-port", 0, "The port to target on pods")
	cmdExpose.Flags().BoolVar(&(exposeOpts.Headless), "headless", false, "Expose through a headless service (valid only for a statefulset target)")

	var unexposeAddress string
	var cmdUnexpose = &cobra.Command{
		Use:   "unexpose [deployment <name>|pods <selector>|statefulset <statefulsetname>|service <name>]",
		Short: "Unexpose a set of pods previously exposed through a Skupper address",
		Args:  exposeTargetArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
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
				os.Exit(1)
			} else {
				fmt.Println("Error, unable to skupper service: ", err.Error())
				os.Exit(1)
			}
		},
	}
	cmdUnexpose.Flags().StringVar(&unexposeAddress, "address", "", "Skupper address the target was exposed as")

	var cmdListExposed = &cobra.Command{
		Use:   "list-exposed",
		Short: "List services exposed over the Skupper network",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
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
				fmt.Println("Could not retrieve services:", err.Error())
				os.Exit(1)
			}
		},
	}

	var cmdService = &cobra.Command{
		Use:   "service create <name> <port> or service delete port",
		Short: "Manage skupper service definitions",
	}

	var serviceToCreate types.ServiceInterface
	var cmdCreateService = &cobra.Command{
		Use:   "create <name> <port>",
		Short: "Create a skupper service",
		Args:  createServiceArgs,
		Run: func(cmd *cobra.Command, args []string) {
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
				fmt.Printf("%s is not a valid port.", sPort)
				fmt.Println()
				os.Exit(1)
			} else {
				serviceToCreate.Port = servicePort
				cli := NewClient(namespace, kubeContext, kubeconfig)
				err = cli.ServiceInterfaceCreate(context.Background(), &serviceToCreate)
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				}
			}
		},
	}
	cmdCreateService.Flags().StringVar(&serviceToCreate.Protocol, "mapping", "tcp", "The mapping in use for this service address (currently one of tcp or http)")
	cmdCreateService.Flags().StringVar(&serviceToCreate.Aggregate, "aggregate", "", "The aggregation strategy to use. One of 'json' or 'multipart'. If specified requests to this service will be sent to all registered implementations and the responses aggregated.")
	cmdCreateService.Flags().BoolVar(&serviceToCreate.EventChannel, "event-channel", false, "If specified, this service will be a channel for multicast events.")
	cmdService.AddCommand(cmdCreateService)

	var cmdDeleteService = &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a skupper service",
		Args:  requiredArg("service-name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			err := cli.ServiceInterfaceRemove(context.Background(), args[0])
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		},
	}
	cmdService.AddCommand(cmdDeleteService)

	var targetPort int
	var protocol string
	var cmdBind = &cobra.Command{
		Use:   "bind <service-name> <target-type> <target-name>",
		Short: "Bind a target to a service",
		Args:  bindArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if protocol != "" && protocol != "tcp" && protocol != "http" && protocol != "http2" {
				fmt.Printf("%s is not a valid protocol. Choose 'tcp', 'http' or 'http2'.", protocol)
				fmt.Println()
				os.Exit(1)
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
				cli := NewClient(namespace, kubeContext, kubeconfig)
				service, err := cli.ServiceInterfaceInspect(context.Background(), args[0])
				if err != nil {
					fmt.Println(err.Error())
					os.Exit(1)
				} else if service == nil {
					fmt.Printf("Service %s not found", args[0])
					fmt.Println()
					os.Exit(1)
				} else {
					err = cli.ServiceInterfaceBind(context.Background(), service, targetType, targetName, protocol, targetPort)
					if err != nil {
						fmt.Println(err.Error())
					}
				}
			}
		},
	}
	cmdBind.Flags().StringVar(&protocol, "protocol", "", "The protocol to proxy (tcp, http or http2).")
	cmdBind.Flags().IntVar(&targetPort, "target-port", 0, "The port the target is listening on.")

	var cmdUnbind = &cobra.Command{
		Use:   "unbind <service-name> <target-type> <target-name>",
		Short: "Unbind a target from a service",
		Args:  bindArgs,
		Run: func(cmd *cobra.Command, args []string) {
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
			cli := NewClient(namespace, kubeContext, kubeconfig)
			err := cli.ServiceInterfaceUnbind(context.Background(), targetType, targetName, args[0], false)
			if err != nil {
				fmt.Println(err.Error())
				os.Exit(1)
			}
		},
	}

	// TODO: change to inspect
	var cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Report the version of the Skupper CLI and services",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli := NewClient(namespace, kubeContext, kubeconfig)
			vir, err := cli.RouterInspect(context.Background())
			fmt.Printf("%-30s %s\n", "client version", version)
			if err == nil {
				fmt.Printf("%-30s %s\n", "transport version", vir.TransportVersion)
				fmt.Printf("%-30s %s\n", "controller version", vir.ControllerVersion)
			} else {
				fmt.Println("Unable to retrieve skupper component versions: ", err.Error())
				os.Exit(1)
			}
		},
	}

	rootCmd = &cobra.Command{Use: "skupper"}
	rootCmd.Version = version
	rootCmd.AddCommand(cmdInit, cmdDelete, cmdConnectionToken, cmdConnect, cmdDisconnect, cmdCheckConnection, cmdStatus, cmdListConnectors, cmdExpose, cmdUnexpose, cmdListExposed,
		cmdService, cmdBind, cmdUnbind, cmdVersion)
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")

}

func main() {
	if rootCmd.Execute() != nil {
		os.Exit(1)
	}
}
