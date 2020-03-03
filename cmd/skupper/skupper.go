package main

import (
	"context"
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"github.com/spf13/cobra"

	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/client"
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

func main() {
	routev1.AddToScheme(scheme.Scheme)

	var kubeContext string
	var namespace string

	var vanRouterCreateOpts types.VanRouterCreateOptions
	var cmdInit = &cobra.Command{
		Use:   "init",
		Short: "Initialise skupper installation",
		Long:  `init will setup a router and other supporting objects to provide a functional skupper installation that can then be connected to other skupper installations`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
			cli.VanRouterCreate(context.Background(), vanRouterCreateOpts)
			fmt.Println("Skupper is now installed in namespace '" + cli.Namespace + "'.  Use 'skupper status' to get more information.")
		},
	}
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.SkupperName, "id", "", "", "Provide a specific identity for the skupper installation")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.IsEdge, "edge", "", false, "Configure as an edge")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableController, "enable-proxy-controller", "", true, "Setup the proxy controller as well as the router")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Configure proxy controller to particiapte in service sync (not relevant if --enable-proxy-controller is false)")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.EnableConsole, "enable-router-console", "", false, "Enable router console")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.AuthMode, "router-console-auth", "", "", "Authentication mode for router console. One of: 'openshift', 'internal', 'unsecured'")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.User, "router-console-user", "", "", "Router console user. Valid only when --router-console-auth=internal")
	cmdInit.Flags().StringVarP(&vanRouterCreateOpts.Password, "router-console-password", "", "", "Router console user. Valid only when --router-console-auth=internal")
	cmdInit.Flags().BoolVarP(&vanRouterCreateOpts.ClusterLocal, "cluster-local", "", false, "Set up skupper to only accept connections from within the local cluster.")

	var cmdDelete = &cobra.Command{
		Use:   "delete",
		Short: "Delete skupper installation",
		Long:  `delete will delete any skupper related objects from the namespace`,
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
			cli.VanRouterRemove(context.Background())
		},
	}

	var clientIdentity string
	var cmdConnectionToken = &cobra.Command{
		Use:   "connection-token <output-file>",
		Short: "Create a connection token.  The 'connect' command uses the token to establish a connection from a remote Skupper site.",
		Args:  requiredArg("output-file"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
			cli.VanConnectorTokenCreate(context.Background(), clientIdentity, args[0])
			//generateConnectSecret(clientIdentity, args[0], initKubeConfig(namespace, context))
		},
	}
	cmdConnectionToken.Flags().StringVarP(&clientIdentity, "client-identity", "i", types.DefaultVanName, "Provide a specific identity as which connecting skupper installation will be authenticated")

	var vanConnectorCreateOpts types.VanConnectorCreateOptions
	var cmdConnect = &cobra.Command{
		Use:   "connect <connection-token-file>",
		Short: "Connect this skupper installation to that which issued the specified connectionToken",
		Args:  requiredArg("connection-token"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
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
			cli, _ := client.NewClient(namespace, kubeContext)
			cli.VanConnectorRemove(context.Background(), args[0])
		},
	}

	var cmdListConnectors = &cobra.Command{
		Use:   "list-connectors",
		Short: "List configured outgoing VAN connections",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
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
				fmt.Println("Error, unable to retrive VAN connections: ", err.Error())
			}
		},
	}

	var waitFor int
	var cmdCheckConnection = &cobra.Command{
		Use:   "check-connection all|<connection-name>",
		Short: "Check whether a connection to another Skupper site is active",
		Args:  requiredArg("connection name"),
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
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
			cli, _ := client.NewClient(namespace, kubeContext)
			vir, err := cli.VanRouterInspect(context.Background())
			if err == nil {
				var modedesc string = " in interior mode"
				if vir.Status.Mode == types.QdrModeEdge {
					modedesc = " in edge mode"
				}
				if vir.Status.QdrReadyReplicas == 0 {
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
				fmt.Println()
			} else {
				fmt.Println("Unable to retrieve skupper status: ", err.Error())
			}
		},
	}

	// TODO: change to inspect
	var cmdVersion = &cobra.Command{
		Use:   "version",
		Short: "Report the version of the Skupper CLI and services",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cli, _ := client.NewClient(namespace, kubeContext)
			vir, err := cli.VanRouterInspect(context.Background())
			if err == nil {
				fmt.Printf("skupctl version              %s\n", version)
				fmt.Printf("qdr version                  %s\n", vir.QdrVersion)
				fmt.Printf("controller version           %s\n", vir.ControllerVersion)
			} else {
				fmt.Println("Unable to retrieve skupper component versions: ", err.Error())
			}
		},
	}

	var rootCmd = &cobra.Command{Use: "skupper"}
	rootCmd.Version = version
	//	rootCmd.AddCommand(cmdInit, cmdDelete, cmdConnectionToken, cmdConnect, cmdListConnectors, cmdDisconnect, cmdCheckConnection, cmdStatus, cmdVersion)
	rootCmd.AddCommand(cmdInit, cmdDelete, cmdConnectionToken, cmdConnect, cmdCheckConnection, cmdListConnectors, cmdDisconnect, cmdStatus, cmdVersion)
	rootCmd.PersistentFlags().StringVarP(&kubeContext, "context", "c", "", "kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use")
	rootCmd.Execute()
}
