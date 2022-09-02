package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
)

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

func NewCmdInitGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "init",
		Short:  "Initialize a gateway to the service network",
		Args:   cobra.NoArgs,
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			if gatewayType != "" && gatewayType != "service" && gatewayType != "docker" && gatewayType != "podman" {
				return fmt.Errorf("%s is not a valid gateway type. Choose 'service', 'docker' or 'podman'.", gatewayType)
			}

			actual, err := kube.Cli.GatewayInit(context.Background(), gatewayName, gatewayType, gatewayConfigFile)
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

func NewCmdDeleteGateway(kube *SkupperKube) *cobra.Command {
	verbose := false
	cmd := &cobra.Command{
		Use:    "delete",
		Short:  "Stop the gateway instance and remove the definition",
		Args:   cobra.NoArgs,
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			err := kube.Cli.GatewayRemove(context.Background(), gatewayName)
			if err != nil && verbose {
				l := formatter.NewList()
				l.Item("Exception while removing gateway definition:")
				parts := strings.Split(err.Error(), ",")
				for _, part := range parts {
					l.NewChild(fmt.Sprintf("%s", part))
				}
				l.Print()
			}
			return nil
		},
	}
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "More details on any exceptions during gateway removal")
	cmd.Flags().StringVar(&deprecatedName, "name", "", "The name of gateway definition to remove")

	f := cmd.Flag("name")
	f.Deprecated = "default name will be used"
	f.Hidden = true

	return cmd
}

func NewCmdDownloadGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "download <output-path>",
		Short:  "Download a gateway definition to a directory",
		Args:   cobra.ExactArgs(1),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			fileName, err := kube.Cli.GatewayDownload(context.Background(), gatewayName, args[0])
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

func NewCmdExportConfigGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "export-config <export-gateway-name> <output-path>",
		Short:  "Export the configuration for a gateway definition",
		Args:   cobra.ExactArgs(2),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			// TODO: validate args must be non nil, etc.
			fileName, err := kube.Cli.GatewayExportConfig(context.Background(), gatewayName, args[0], args[1])
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

func NewCmdGenerateBundleGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "generate-bundle <config-file> <output-path>",
		Short:  "Generate an installation bundle using a gateway config file",
		Args:   cobra.ExactArgs(2),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			fileName, err := kube.Cli.GatewayGenerateBundle(context.Background(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			fmt.Println("Skupper gateway bundle written to '" + fileName + "'")
			return nil
		},
	}

	return cmd
}

func NewCmdExposeGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "expose <address> <host> <port...>",
		Short:  "Expose a process to the service network (ensure gateway and cluster service)",
		Args:   exposeGatewayArgs,
		PreRun: kube.NewClient,
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

			_, err := kube.Cli.GatewayExpose(context.Background(), gatewayName, gatewayType, gatewayEndpoint)
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

func NewCmdUnexposeGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unexpose <address>",
		Short:  "Unexpose a process previously exposed to the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := kube.Cli.GatewayUnexpose(context.Background(), gatewayName, gatewayEndpoint, deleteLast)
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

func NewCmdBindGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "bind <address> <host> <port...>",
		Short:  "Bind a process to the service network",
		Args:   bindGatewayArgs,
		PreRun: kube.NewClient,
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

			err := kube.Cli.GatewayBind(context.Background(), gatewayName, gatewayEndpoint)
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

func NewCmdUnbindGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unbind <address>",
		Short:  "Unbind a process from the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := kube.Cli.GatewayUnbind(context.Background(), gatewayName, gatewayEndpoint)
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

func NewCmdStatusGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "status <gateway-name>",
		Short:  "Report the status of the gateway(s) for the current skupper site",
		Args:   cobra.MaximumNArgs(1),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gateways, err := kube.Cli.GatewayList(context.Background())
			if err != nil {
				return fmt.Errorf("%w", err)
			}

			if len(gateways) == 0 {
				l := formatter.NewList()
				l.Item("No gateway definition found on cluster")
				gatewayType, err := client.GatewayDetectTypeIfPresent()
				if err == nil {
					if gatewayType != "" {
						l.NewChild(fmt.Sprintf(" A gateway of type %s is detected on local host.", gatewayType))
					}
				}
				l.Print()
				return nil
			}

			l := formatter.NewList()
			l.Item("Gateway Definition:")
			for _, gateway := range gateways {
				gw := l.NewChild(fmt.Sprintf("%s type:%s version:%s", gateway.Name, gateway.Type, gateway.Version))
				if len(gateway.Connectors) > 0 {
					listeners := gw.NewChild("Bindings:")
					for _, connector := range gateway.Connectors {
						listeners.NewChild(fmt.Sprintf("%s %s %s %s %d", strings.TrimPrefix(connector.Name, gateway.Name+"-egress-"), connector.Service.Protocol, connector.Service.Address, connector.Host, connector.Service.Ports[0]))
					}
				}
				if len(gateway.Listeners) > 0 {
					listeners := gw.NewChild("Forwards:")
					for _, listener := range gateway.Listeners {
						listeners.NewChild(fmt.Sprintf("%s %s %s %s %d:%s", strings.TrimPrefix(listener.Name, gateway.Name+"-ingress-"), listener.Service.Protocol, listener.Service.Address, listener.Host, listener.Service.Ports[0], listener.LocalPort))
					}
				}
			}
			l.Print()

			return nil
		},
	}

	return cmd
}

func NewCmdForwardGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "forward <address> <port...>",
		Short:  "Forward an address to the service network",
		Args:   cobra.MinimumNArgs(2),
		PreRun: kube.NewClient,
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

			err := kube.Cli.GatewayForward(context.Background(), gatewayName, gatewayEndpoint)
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

func NewCmdUnforwardGateway(kube *SkupperKube) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "unforward <address>",
		Short:  "Stop forwarding an address to the service network",
		Args:   cobra.ExactArgs(1),
		PreRun: kube.NewClient,
		RunE: func(cmd *cobra.Command, args []string) error {
			silenceCobra(cmd)

			gatewayEndpoint.Service.Address = args[0]
			err := kube.Cli.GatewayUnforward(context.Background(), gatewayName, gatewayEndpoint)
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
