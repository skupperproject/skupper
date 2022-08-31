package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

var SkupperKubeCommands = []string{
	"init", "delete", "update", "connection-token", "token", "link", "connect", "disconnect",
	"check-connection", "status", "list-connectors", "expose", "unexpose", "list-exposed",
	"service", "bind", "unbind", "version", "debug", "completion", "gateway", "revoke-access",
	"network", "switch",
}

type SkupperKube struct {
	cli            types.VanClientInterface
	kubeContext    string
	namespace      string
	kubeConfigPath string
	kubeInit       kubeInit
}

func (s *SkupperKube) SupportedCommands() []string {
	return SkupperKubeCommands
}

func (s *SkupperKube) Platform() types.Platform {
	return types.PlatformKubernetes
}

func (s *SkupperKube) Options(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&s.kubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&s.kubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&s.namespace, "namespace", "n", "", "The Kubernetes namespace to use")
}

type kubeInit struct {
	annotations                  []string
	ingressAnnotations           []string
	routerServiceAnnotations     []string
	controllerServiceAnnotations []string
	clusterLocal                 bool
	isEdge                       bool
}

func (s *SkupperKube) NewClient(cmd *cobra.Command, args []string) {
	exitOnError := true
	if cmd.Name() == "version" {
		exitOnError = false
	}
	s.cli = NewClientHandleError(s.namespace, s.kubeContext, s.kubeConfigPath, exitOnError)
	// TODO remove once all methods converted
	cli = s.cli
}

func (s *SkupperKube) Init(cmd *cobra.Command, args []string) error {
	cli := s.cli

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
		if !utils.StringSliceContains(options, initFlags.routerMode) {
			return fmt.Errorf(`invalid "--router-mode=%v", it must be one of "%v"`, initFlags.routerMode, strings.Join(options, ", "))
		}
		routerCreateOpts.RouterMode = initFlags.routerMode
	} else {
		if s.kubeInit.isEdge {
			routerCreateOpts.RouterMode = string(types.TransportModeEdge)
		} else {
			routerCreateOpts.RouterMode = string(types.TransportModeInterior)
		}
	}

	routerIngressFlag := cmd.Flag("ingress")
	routerClusterLocalFlag := cmd.Flag("cluster-local")
	routerCreateOpts.Platform = s.Platform()

	if routerIngressFlag.Changed && routerClusterLocalFlag.Changed {
		return fmt.Errorf(`You can not use the deprecated --cluster-local, and --ingress together, use "--ingress none" as equivalent of --cluster-local`)
	} else if routerClusterLocalFlag.Changed {
		if s.kubeInit.clusterLocal { // this is redundant, because "if changed" it must be true, but it is also correct
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
	routerCreateOpts.Annotations = asMap(s.kubeInit.annotations)
	routerCreateOpts.Labels = asMap(initFlags.labels)
	routerCreateOpts.IngressAnnotations = asMap(s.kubeInit.ingressAnnotations)
	routerCreateOpts.Router.ServiceAnnotations = asMap(s.kubeInit.routerServiceAnnotations)
	routerCreateOpts.Controller.ServiceAnnotations = asMap(s.kubeInit.controllerServiceAnnotations)
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

	if LoadBalancerTimeout.Seconds() <= 0 {
		return fmt.Errorf(`invalid timeout value`)
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

	ctx, cancel := context.WithTimeout(context.Background(), LoadBalancerTimeout)
	defer cancel()

	err = cli.RouterCreate(ctx, *siteConfig)
	if err != nil {
		return err
	}
	fmt.Println("Skupper is now installed in namespace '" + ns + "'.  Use 'skupper status' to get more information.")

	return nil
}

func (s *SkupperKube) InitFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", true, "Enable skupper console")
	cmd.Flags().BoolVarP(&routerCreateOpts.CreateNetworkPolicy, "create-network-policy", "", false, "Create network policy to restrict access to skupper services exposed through this site to current pods in namespace")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.ConsoleIngress, "console-ingress", "", "", "Determines if/how console is exposed outside cluster. If not specified uses value of --ingress. One of: ["+strings.Join(types.ValidIngressOptions(s.Platform()), "|")+"].")
	cmd.Flags().StringSliceVar(&s.kubeInit.ingressAnnotations, "ingress-annotations", []string{}, "Annotations to add to skupper ingress")
	cmd.Flags().StringSliceVar(&s.kubeInit.annotations, "annotations", []string{}, "Annotations to add to skupper pods")
	cmd.Flags().StringSliceVar(&s.kubeInit.routerServiceAnnotations, "router-service-annotations", []string{}, "Annotations to add to skupper router service")
	cmd.Flags().StringSliceVar(&s.kubeInit.controllerServiceAnnotations, "controller-service-annotation", []string{}, "Annotations to add to skupper controller service")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Participate in cross-site service synchronization")

	cmd.Flags().StringVar(&routerCreateOpts.Router.Cpu, "router-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Memory, "router-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.CpuLimit, "router-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.MemoryLimit, "router-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.NodeSelector, "router-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Affinity, "router-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.AntiAffinity, "router-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.IngressHost, "router-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")
	cmd.Flags().StringVar(&routerCreateOpts.Router.LoadBalancerIp, "router-load-balancer-ip", "", "Load balancer ip that will be used for router service, if supported by cloud provider")

	cmd.Flags().StringVar(&routerCreateOpts.Controller.Cpu, "controller-cpu", "", "CPU request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Memory, "controller-memory", "", "Memory request for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.CpuLimit, "controller-cpu-limit", "", "CPU limit for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.MemoryLimit, "controller-memory-limit", "", "Memory limit for controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.NodeSelector, "controller-node-selector", "", "Node selector to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.Affinity, "controller-pod-affinity", "", "Pod affinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.AntiAffinity, "controller-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of controller pods")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.IngressHost, "controller-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.LoadBalancerIp, "controller-load-balancer-ip", "", "Load balancer ip that will be used for controller service, if supported by cloud provider")

	cmd.Flags().StringVar(&routerCreateOpts.ConfigSync.Cpu, "config-sync-cpu", "", "CPU request for config-sync pods")
	cmd.Flags().StringVar(&routerCreateOpts.ConfigSync.Memory, "config-sync-memory", "", "Memory request for config-sync pods")
	cmd.Flags().StringVar(&routerCreateOpts.ConfigSync.CpuLimit, "config-sync-cpu-limit", "", "CPU limit for config-sync pods")
	cmd.Flags().StringVar(&routerCreateOpts.ConfigSync.MemoryLimit, "config-sync-memory-limit", "", "Memory limit for config-sync pods")

	cmd.Flags().DurationVar(&LoadBalancerTimeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for the ingress loadbalancer option.")

	cmd.Flags().BoolVarP(&s.kubeInit.clusterLocal, "cluster-local", "", false, "Set up Skupper to only accept links from within the local cluster.")
	f := cmd.Flag("cluster-local")
	f.Deprecated = "This flag is deprecated, use --ingress [" + strings.Join(types.ValidIngressOptions(s.Platform()), "|") + "]"
	f.Hidden = true

	cmd.Flags().BoolVarP(&s.kubeInit.isEdge, "edge", "", false, "Configure as an edge")
	f = cmd.Flag("edge")
	f.Deprecated = "This flag is deprecated, use --router-mode [interior|edge]"
	f.Hidden = true
}

func (s *SkupperKube) DebugDump(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	file, err := cli.SkupperDump(context.Background(), args[0], client.Version, s.kubeConfigPath, s.kubeContext)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	} else {
		fmt.Println("Skupper dump details written to compressed archive: ", file)
	}
	return nil
}
