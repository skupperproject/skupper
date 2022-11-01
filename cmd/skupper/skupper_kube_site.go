package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
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
	routerCreateOpts.Router.MaxFrameSize = types.RouterMaxFrameSizeDefault
	routerCreateOpts.Router.MaxSessionFrames = types.RouterMaxSessionFramesDefault
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
	s.kubeInit.controllerServiceAnnotations = []string{}
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console must be used in conjunction with '--enable-flow-collector' flag")
	cmd.Flag("ingress").Usage += " If not specified route is used when available, otherwise loadbalancer is used."
	cmd.Flags().StringVarP(&routerCreateOpts.IngressHost, "ingress-host", "", "", "Hostname or alias by which the ingress route or proxy can be reached")
	cmd.Flags().BoolVarP(&routerCreateOpts.CreateNetworkPolicy, "create-network-policy", "", false, "Create network policy to restrict access to skupper services exposed through this site to current pods in namespace")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'openshift', 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.ConsoleIngress, "console-ingress", "", "", "Determines if/how console is exposed outside cluster. If not specified uses value of --ingress. One of: ["+strings.Join(types.ValidIngressOptions(s.kube.Platform()), "|")+"].")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableRestAPI, "enable-rest-api", "", false, "Enable REST API")
	cmd.Flags().StringSliceVar(&s.kubeInit.ingressAnnotations, "ingress-annotations", []string{}, "Annotations to add to skupper ingress")
	cmd.Flags().StringSliceVar(&s.kubeInit.annotations, "annotations", []string{}, "Annotations to add to skupper pods")
	cmd.Flags().StringSliceVar(&s.kubeInit.routerServiceAnnotations, "router-service-annotations", []string{}, "Annotations to add to skupper router service")
	cmd.Flags().StringSliceVar(&s.kubeInit.controllerServiceAnnotations, "controller-service-annotation", []string{}, "Annotations to add to skupper controller service")
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

	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.Cpu, "flow-collector-cpu", "", "CPU request for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.Memory, "flow-collector-memory", "", "Memory request for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.CpuLimit, "flow-collector-cpu-limit", "", "CPU limit for flow collector pods")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.MemoryLimit, "flow-collector-memory-limit", "", "Memory limit for flow collector pods")

	cmd.Flags().DurationVar(&LoadBalancerTimeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for the ingress loadbalancer option.")

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
		siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
		if err != nil {
			return err
		} else {
			if siteConfig.Spec.EnableFlowCollector && vir.ConsoleUrl != "" {
				fmt.Println("The site console url is: ", vir.ConsoleUrl)
				if siteConfig.Spec.AuthMode == "internal" {
					fmt.Println("The credentials for internal console-auth mode are held in secret: 'skupper-console-users'")
				}
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

func (s *SkupperKubeSite) UpdateFlags(cmd *cobra.Command) {}

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
