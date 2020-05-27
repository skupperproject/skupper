package main

import (
	"context"
	"fmt"
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

func exposeTarget() func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("expose target must be specified (e.g. 'skupper expose deployment <name>'")
		}
		if len(args) > 2 {
			return fmt.Errorf("illegal argument: %s", args[2])
		}
		if args[0] != "deployment" && args[0] != "statefulset" && args[0] != "pods" {
			return fmt.Errorf("expose target type must be one of 'deployment', 'statefulset' or 'pods'")
		}
		return nil
	}
}

func main() {
	routev1.AddToScheme(scheme.Scheme)

	var kubeContext string
	var namespace string
	var kubeconfig string

	var vanRouterCreateOpts types.VanRouterCreateOptions
	var cmdInit = &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long:  `init will setup a router and other supporting objects to provide a functional skupper installation that can then be connected to other skupper installations`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			err := cli.VanRouterCreate(context.Background(), vanRouterCreateOpts)
			if err != nil {
				fmt.Println("Error, unable to init Skupper VAN router: ", err.Error())
			} else {
				fmt.Println("Skupper is now installed in namespace '" + cli.Namespace + "'.  Use 'skupper status' to get more information.")
			}
		},
	}
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.SkupperName, "site-name", "", "", "Provide a specific name for this skupper installation")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.IsEdge, "edge", "", false, "Configure as an edge")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableController, "enable-proxy-controller", "", true, "Setup the proxy controller as well as the router")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Configure proxy controller to particiapte in service sync (not relevant if --enable-proxy-controller is false)")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableRouterConsole, "enable-router-console", "", false, "Enable router console")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.ClusterLocal, "cluster-local", "", false, "Set up skupper to only accept connections from within the local cluster.")

	var cmdDelete = &cobra.Command{
		Use:   "delete",
		Short: "Delete skupper installation",
		Long:  `delete will delete any skupper related objects from the namespace`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			err := cli.VanRouterRemove(context.Background())
			if err == nil {
				fmt.Println("Skupper is now removed from '" + cli.Namespace + "'.")
			} else {
				fmt.Println(err.Error())
			}
		},
	}

	var clientIdentity string
	var cmdConnectionToken = &cobra.Command{
		Use:   "connection-token <output-file>",
		Short: "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
		Args:  requiredArg("output-file"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			err := cli.VanConnectorTokenCreate(context.Background(), clientIdentity, args[0])
			if err != nil {
				fmt.Println("Failed to create connection token: ", err.Error())
			}
		},
	}
	cmdConnectionToken.Flags().StringVarP(&clientIdentity, "client-identity", "i", types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")

	var vanConnectorCreateOpts types.VanConnectorCreateOptions
	var cmdConnect = &cobra.Command{
		Use:   "connect <connection-token-file>",
		Short: "Connect this skupper installation to that which issued the specified connectionToken",
		Args:  requiredArg("connection-token"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			// TODO: check error, return results for connection
			cli.VanConnectorCreate(context.Background(), args[0], vanConnectorCreateOpts)
		},
	}
	cmdConnect.Flags().StringVarP(&vanConnectorCreateOpts.Name, "connection-name", "", "", "Provide a specific name for the connection (used when removing it with disconnect)")
	cmdConnect.Flags().Int32VarP(&vanConnectorCreateOpts.Cost, "cost", "", 1, "Specify a cost for this connection.")

	var cmdDisconnect = &cobra.Command{
		Use:   "disconnect <name>",
		Short: "Remove specified connection",
		Args:  requiredArg("connection name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			err := cli.VanConnectorRemove(context.Background(), args[0])
			if err == nil {
				fmt.Println("Connection '" + args[0] + "' has been removed")
			} else {
				fmt.Println("Failed to remove connection: ", err.Error())
			}
		},
	}

	var waitFor int
	var cmdCheckConnection = &cobra.Command{
		Use:   "check-connection all|<connection-name>",
		Short: "Check whether a connection to another Skupper site is active",
		Args:  requiredArg("connection name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			var connectors []*types.VanConnectorInspectResponse
			connected := 0

			if args[0] == "all" {
				vcis, err := cli.VanConnectorList(context.Background())
				if err == nil {
					for _, vci := range vcis {
						connectors = append(connectors, &types.VanConnectorInspectResponse{
							Connector: vci,
							Connected: false,
						})
					}
				}
			} else {
				vci, err := cli.VanConnectorInspect(context.Background(), args[0])
				if err == nil {
					connectors = append(connectors, vci)
					if vci.Connected {
						connected++
					}
				}
			}

			for i := 0; connected < len(connectors) && i < waitFor; i++ {
				for _, c := range connectors {
					vci, err := cli.VanConnectorInspect(context.Background(), c.Connector.Name)
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
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			vir, err := cli.VanRouterInspect(context.Background())
			if err == nil {
				var modedesc string = " in interior mode"
				if vir.Status.Mode == types.TransportModeEdge {
					modedesc = " in edge mode"
				}
				if vir.Status.TransportReadyReplicas == 0 {
					fmt.Printf("VanRouter is installed in namespace '%q%s'. Status pending...", cli.Namespace, modedesc)
				} else {
					fmt.Printf("VanRouter is enabled for namespace '%q%s'.", cli.Namespace, modedesc)
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
			} else {
				fmt.Println("Unable to retrieve skupper status: ", err.Error())
			}
		},
	}

	var cmdListConnectors = &cobra.Command{
		Use:   "list-connectors",
		Short: "List configured outgoing connections",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			connectors, err := cli.VanConnectorList(context.Background())
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
				fmt.Println("The VanRouter is not install in '" + cli.Namespace + "`")
			} else {
				fmt.Println("Error, unable to retrieve VAN connections: ", err.Error())
			}
		},
	}

	vanServiceInterfaceCreateOpts := types.VanServiceInterfaceCreateOptions{}
	var cmdExpose = &cobra.Command{
		Use:   "expose [deployment <name>|pods <selector>|statefulset <statefulsetname>]",
		Short: "Expose a set of pods through a Skupper address",
		Args:  exposeTarget(),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			targetType := args[0]
			var targetName string
			if len(args) == 2 {
				targetName = args[1]
			} else {
				parts := strings.Split(args[0], "/")
				targetType = parts[0]
				targetName = parts[1]
			}
			err := cli.VanServiceInterfaceCreate(context.Background(), targetType, targetName, vanServiceInterfaceCreateOpts)
			if err == nil {
				fmt.Printf("VAN Service Interface Target %s exposed\n", args[1])
			} else if errors.IsNotFound(err) {
				fmt.Println("The VAN Router is not installed in '" + cli.Namespace + "`")
			} else {
				fmt.Println("Error, unable to create VAN service interface: ", err.Error())
			}
		},
	}
	cmdExpose.Flags().StringVar(&(vanServiceInterfaceCreateOpts.Protocol), "protocol", "tcp", "The protocol to proxy (tcp, http, or http2)")
	cmdExpose.Flags().StringVar(&(vanServiceInterfaceCreateOpts.Address), "address", "", "The Skupper address to expose")
	cmdExpose.Flags().IntVar(&(vanServiceInterfaceCreateOpts.Port), "port", 0, "The port to expose on")
	cmdExpose.Flags().IntVar(&(vanServiceInterfaceCreateOpts.TargetPort), "target-port", 0, "The port to target on pods")
	cmdExpose.Flags().BoolVar(&(vanServiceInterfaceCreateOpts.Headless), "headless", false, "Expose through a headless service (valid only for a statefulset target)")

	var unexposeAddress string
	var cmdUnexpose = &cobra.Command{
		Use:   "unexpose [deployment <name>|pods <selector>|statefulset <statefulsetname>]",
		Short: "Unexpose a set of pods previously exposed through a Skupper address",
		Args:  exposeTarget(),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			targetType := args[0]
			var targetName string
			if len(args) == 2 {
				targetName = args[1]
			} else {
				parts := strings.Split(args[0], "/")
				targetType = parts[0]
				targetName = parts[1]
			}
			err := cli.VanServiceInterfaceRemove(context.Background(), targetType, targetName, unexposeAddress)
			if err == nil {
				fmt.Printf("VAN Service Interface Target %s unexposed\n", targetName)
			} else {
				fmt.Println("Error, unable to remove VAN service interface: ", err.Error())
			}
		},
	}
	cmdUnexpose.Flags().StringVar(&unexposeAddress, "address", "", "Skupper address the target was exposed as")

	var cmdListExposed = &cobra.Command{
		Use:   "list-exposed",
		Short: "List services exposed over the Skupper network",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			vsis, err := cli.VanServiceInterfaceList(context.Background())
			if err == nil {
				if len(vsis) == 0 {
					fmt.Println("No service interfaces defined")
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
								fmt.Printf("      => %s %s", t.Selector, name)
								fmt.Println()
							}
						}
					}
				}
			} else {
				fmt.Println("Could not retrieve service interfaces:", err.Error())
			}
		},
	}

	var cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Report the version of the Skupper CLI and services",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext, kubeconfig)
			vir, err := cli.VanRouterInspect(context.Background())
			if err == nil {
				fmt.Printf("client version               %s\n", version)
				fmt.Printf("transport version            %s\n", vir.TransportVersion)
				fmt.Printf("controller version           %s\n", vir.ControllerVersion)
			} else {
				fmt.Println("Unable to retrieve skupper component versions: ", err.Error())
			}
		},
	}

	var rootCmd = &cobra.Command{Use: "skupper"}
	rootCmd.Version = version
	rootCmd.AddCommand(cmdInit, cmdDelete, cmdConnectionToken, cmdConnect, cmdDisconnect, cmdCheckConnection, cmdStatus, cmdListConnectors, cmdExpose, cmdUnexpose, cmdListExposed, cmdVersion)
	rootCmd.PersistentFlags().StringVarP(&kubeconfig, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "The Kubernetes namespace to use")
	rootCmd.Execute()
}
