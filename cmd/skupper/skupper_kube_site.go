package main

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/pkg/network"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/spf13/cobra"
)

type SkupperKubeSite struct {
	kube     *SkupperKube
	kubeInit kubeInit
}

func (s *SkupperKubeSite) NewClient(cmd *cobra.Command, args []string) {
	s.kube.NewClient(cmd, args)
}

func (s *SkupperKubeSite) Platform() types.Platform {
	return s.kube.Platform()
}

func (s *SkupperKubeSite) Create(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli

	silenceCobra(cmd)
	ns := cli.GetNamespace()

	routerIngressFlag := cmd.Flag("ingress")
	routerCreateOpts.Platform = s.kube.Platform()

	if !routerIngressFlag.Changed {
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
	routerCreateOpts.Router.PodAnnotations = asMap(s.kubeInit.routerPodAnnotations)
	routerCreateOpts.Router.MaxFrameSize = types.RouterMaxFrameSizeDefault
	routerCreateOpts.Router.MaxSessionFrames = types.RouterMaxSessionFramesDefault
	routerCreateOpts.Controller.ServiceAnnotations = asMap(s.kubeInit.controllerServiceAnnotations)
	routerCreateOpts.Controller.PodAnnotations = asMap(s.kubeInit.controllerPodAnnotations)
	routerCreateOpts.PrometheusServer.PodAnnotations = asMap(s.kubeInit.prometheusServerPodAnnotations)
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

	if LoadBalancerTimeout.Seconds() <= 0 {
		return fmt.Errorf(`invalid timeout value`)
	}
	if routerCreateOpts.SiteTtl != 0 && routerCreateOpts.SiteTtl < time.Minute {
		return fmt.Errorf("The minimum value for service-sync-site-ttl is 1 minute")
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
		err2 := cli.SiteConfigRemove(context.Background())
		if err2 != nil {
			fmt.Println("Failed to cleanup site: ", err2)
		}
		return err
	}
	fmt.Println("Skupper is now installed in namespace '" + ns + "'.  Use 'skupper status' to get more information.")

	return nil
}

func (s *SkupperKubeSite) CreateFlags(cmd *cobra.Command) {
	s.kubeInit = kubeInit{}
	s.kubeInit.ingressAnnotations = []string{}
	s.kubeInit.annotations = []string{}
	s.kubeInit.routerServiceAnnotations = []string{}
	s.kubeInit.routerPodAnnotations = []string{}
	s.kubeInit.controllerServiceAnnotations = []string{}
	s.kubeInit.controllerPodAnnotations = []string{}
	s.kubeInit.prometheusServerPodAnnotations = []string{}
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console must be used in conjunction with '--enable-flow-collector' flag")
	cmd.Flag("ingress").Usage += " If not specified route is used when available, otherwise loadbalancer is used."
	cmd.Flags().StringVarP(&routerCreateOpts.IngressHost, "ingress-host", "", "", "Hostname or alias by which the ingress route or proxy can be reached")
	cmd.Flags().BoolVarP(&routerCreateOpts.CreateNetworkPolicy, "create-network-policy", "", false, "Create network policy to restrict access to skupper services exposed through this site to current pods in namespace")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "internal", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.ConsoleIngress, "console-ingress", "", "", "Determines if/how console is exposed outside cluster. If not specified uses value of --ingress. One of: ["+strings.Join(types.ValidIngressOptions(s.kube.Platform()), "|")+"].")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRestAPI, "enable-rest-api", "", false, "Enable REST API")
	cmd.Flags().StringSliceVar(&s.kubeInit.ingressAnnotations, "ingress-annotations", []string{}, "Annotations to add to skupper ingress")
	cmd.Flags().StringSliceVar(&s.kubeInit.annotations, "annotations", []string{}, "Annotations to add to skupper pods")
	cmd.Flags().StringSliceVar(&s.kubeInit.routerServiceAnnotations, "router-service-annotations", []string{}, "Annotations to add to skupper router service")
	cmd.Flags().StringSliceVar(&s.kubeInit.routerPodAnnotations, "router-pod-annotations", []string{}, "Annotations to add to skupper router pod")
	cmd.Flags().StringSliceVar(&s.kubeInit.controllerServiceAnnotations, "controller-service-annotation", []string{}, "Annotations to add to skupper controller service")
	cmd.Flags().StringSliceVar(&s.kubeInit.controllerPodAnnotations, "controller-pod-annotation", []string{}, "Annotations to add to skupper controller pod")
	cmd.Flags().StringSliceVar(&s.kubeInit.prometheusServerPodAnnotations, "prometheus-server-pod-annotation", []string{}, "Annotations to add to skupper prometheus pod")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableServiceSync, "enable-service-sync", "", true, "Participate in cross-site service synchronization")
	cmd.Flags().DurationVar(&routerCreateOpts.SiteTtl, "service-sync-site-ttl", 0, "Time after which stale services, i.e. those whose site has not been heard from, created through service-sync are removed.")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableFlowCollector, "enable-flow-collector", "", false, "Enable cross-site flow collection for the application network")
	cmd.Flags().Int64Var(&routerCreateOpts.RunAsUser, "run-as-user", 0, "The UID to run the entrypoint of the container processes")
	cmd.Flags().Int64Var(&routerCreateOpts.RunAsGroup, "run-as-group", 0, "The GID to run the entrypoint of the container processes")

	cmd.Flags().IntVar(&routerCreateOpts.Routers, "routers", 0, "Number of router replicas to start")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Cpu, "router-cpu", "", "CPU request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Memory, "router-memory", "", "Memory request for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.CpuLimit, "router-cpu-limit", "", "CPU limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.MemoryLimit, "router-memory-limit", "", "Memory limit for router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.NodeSelector, "router-node-selector", "", "Node selector to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.Affinity, "router-pod-affinity", "", "Pod affinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.AntiAffinity, "router-pod-antiaffinity", "", "Pod antiaffinity label matches to control placement of router pods")
	cmd.Flags().StringVar(&routerCreateOpts.Router.IngressHost, "router-ingress-host", "", "Host through which node is accessible when using nodeport as ingress.")
	cmd.Flags().StringVar(&routerCreateOpts.Router.LoadBalancerIp, "router-load-balancer-ip", "", "Load balancer ip that will be used for router service, if supported by cloud provider")
	cmd.Flags().BoolVarP(&routerCreateOpts.Router.DisableMutualTLS, "router-disable-mutual-tls", "", false, "Disables client authentication through TLS of sites linking to this site")
	cmd.Flags().StringVarP(&routerCreateOpts.Router.DataConnectionCount, "router-data-connection-count", "", "", "Configures the number of data connections the router will use when linking to other routers")

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
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableClusterPermissions, "enable-cluster-permissions", "", false, "Enable cluster wide permissions in order to expose deployments/statefulsets in other namespaces")

	cmd.Flags().DurationVar(&routerCreateOpts.FlowCollector.FlowRecordTtl, "flow-collector-record-ttl", 0, "Time after which terminated flow records are deleted, i.e. those flow records that have an end time set. Default is 15 minutes.")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.Cpu, "flow-collector-cpu", "", "CPU request for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.Memory, "flow-collector-memory", "", "Memory request for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.CpuLimit, "flow-collector-cpu-limit", "", "CPU limit for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.MemoryLimit, "flow-collector-memory-limit", "", "Memory limit for flow collector pods")

	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.Cpu, "prometheus-cpu", "", "CPU request for prometheus pods")
	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.Memory, "prometheus-memory", "", "Memory request for prometheus pods")
	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.CpuLimit, "prometheus-cpu-limit", "", "CPU limit for prometheus pods")
	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.MemoryLimit, "prometheus-memory-limit", "", "Memory limit for prometheus pods")

	cmd.Flags().DurationVar(&LoadBalancerTimeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for the ingress loadbalancer option.")
	cmd.Flags().BoolVar(&routerCreateOpts.EnableSkupperEvents, "enable-skupper-events", true, "Enable sending Skupper events to Kubernetes")

	// hide run-as flags
	f := cmd.Flag("run-as-user")
	f.Hidden = true
	f = cmd.Flag("run-as-group")
	f.Hidden = true
	f = cmd.Flag("router-disable-mutual-tls")
	f.Hidden = true
}

func (s *SkupperKubeSite) Delete(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	cli := s.kube.Cli
	gateways, err := cli.GatewayList(context.Background())
	for _, gateway := range gateways {
		cli.GatewayRemove(context.Background(), gateway.Name)
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
}
func (s *SkupperKubeSite) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperKubeSite) List(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *SkupperKubeSite) ListFlags(cmd *cobra.Command) {}

func (s *SkupperKubeSite) Status(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	cli := s.kube.Cli
	vir, err := cli.RouterInspect(context.Background())
	if err != nil {
		return err
	}

	currentStatus, errStatus := cli.NetworkStatus(context.Background())
	if errStatus != nil {
		return errStatus
	}

	statusManager := network.SkupperStatus{
		NetworkStatus: currentStatus,
	}

	siteConfig, err := s.kube.Cli.SiteConfigInspect(context.Background(), nil)
	if err != nil || siteConfig == nil {
		fmt.Printf("The site configuration is not available: %s", err)
		fmt.Println()
		return nil
	}
	var currentSite = statusManager.GetSiteById(siteConfig.Reference.UID)

	if currentSite != nil {

		statusDataOutput := StatusData{
			enabledIn: PlatformSupport{"namespace", currentSite.Site.Namespace},
			mode:      currentSite.RouterStatus[0].Router.Mode,
			siteName:  currentSite.Site.Name,
			policies:  currentSite.Site.Policy,
		}

		if len(vir.Status.ConnectedSites.Warnings) > 0 {
			var warnings []string
			for _, w := range vir.Status.ConnectedSites.Warnings {
				warnings = append(warnings, w)
			}

			statusDataOutput.warnings = warnings
		}

		mapSiteLink := statusManager.GetSiteLinkMapPerRouter(&currentSite.RouterStatus[0], &currentSite.Site)

		totalSites := len(currentStatus.SiteStatus)
		// the current site does not count as a connection
		connections := totalSites - 1
		directConnections := len(mapSiteLink)
		statusDataOutput.totalConnections = connections
		statusDataOutput.directConnections = directConnections
		statusDataOutput.indirectConnections = connections - directConnections

		statusDataOutput.exposedServices = len(currentStatus.Addresses)

		siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
		if err != nil {
			return err
		} else {
			if siteConfig.Spec.EnableFlowCollector && vir.ConsoleUrl != "" {
				statusDataOutput.consoleUrl = vir.ConsoleUrl
				if siteConfig.Spec.AuthMode == "internal" {
					statusDataOutput.credentials = PlatformSupport{"secret", "'skupper-console-users'"}
				}
			}
		}

		if err == nil && verboseStatus {
			err := PrintVerboseStatus(statusDataOutput)
			if err != nil {
				return err
			}

		} else if err == nil {
			err := PrintStatus(statusDataOutput)
			if err != nil {
				return err
			}
		}
	} else {
		return fmt.Errorf("unable to retrieve skupper status")
	}
	return nil
}

func (s *SkupperKubeSite) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperKubeSite) Update(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	cli := s.kube.Cli

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
}

func (s *SkupperKubeSite) UpdateFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&forceHup, "force-restart", "", false, "Restart skupper daemons even if image tag is not updated")
}

func (s *SkupperKubeSite) Version(cmd *cobra.Command, args []string) error {
	cli := s.kube.Cli
	if !IsZero(reflect.ValueOf(cli)) {
		fmt.Printf("%-30s %s\n", "transport version", cli.GetVersion(types.TransportComponentName, types.TransportContainerName))
		fmt.Printf("%-30s %s\n", "controller version", cli.GetVersion(types.ControllerComponentName, types.ControllerContainerName))
		fmt.Printf("%-30s %s\n", "config-sync version", cli.GetVersion(types.TransportComponentName, types.ConfigSyncContainerName))
		fmt.Printf("%-30s %s\n", "flow-collector version", cli.GetVersion(types.ControllerComponentName, types.FlowCollectorContainerName))
	} else {
		fmt.Printf("%-30s %s\n", "transport version", "not-found (no configuration has been provided)")
		fmt.Printf("%-30s %s\n", "controller version", "not-found (no configuration has been provided)")
	}
	return nil
}

func (s *SkupperKubeSite) RevokeAccess(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	err := s.kube.Cli.RevokeAccess(context.Background())
	if err != nil {
		return fmt.Errorf("Unable to revoke access: %w", err)
	}
	return nil
}
