package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
)

var SkupperKubeCommands = []string{
	"init", "delete", "update", "connection-token", "token", "link", "connect", "disconnect",
	"check-connection", "status", "list-connectors", "expose", "unexpose", "list-exposed",
	"service", "bind", "unbind", "version", "debug", "completion", "gateway", "revoke-access",
	"network", "switch",
}

type SkupperKube struct {
	Cli            types.VanClientInterface
	KubeContext    string
	Namespace      string
	KubeConfigPath string
	kubeInit       kubeInit
}

func (s *SkupperKube) TokenCreate(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	switch tokenType {
	case "cert":
		err := cli.ConnectorTokenCreateFile(context.Background(), clientIdentity, args[0])
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	case "claim":
		name := clientIdentity
		if name == "skupper" {
			name = ""
		}
		if password == "" {
			password = utils.RandomId(24)
		}
		err := cli.TokenClaimCreateFile(context.Background(), name, []byte(password), expiry, uses, args[0])
		if err != nil {
			return fmt.Errorf("Failed to create token: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("invalid token type. Specify cert or claim")
	}
}

func (s *SkupperKube) RevokeAccess(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
	err := cli.RevokeAccess(context.Background())
	if err != nil {
		return fmt.Errorf("Unable to revoke access: %w", err)
	}
	return nil
}

func (s *SkupperKube) NetworkStatus(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	var sites []*types.SiteInfo
	var errStatus error
	err := utils.RetryError(time.Second, 3, func() error {
		sites, errStatus = cli.NetworkStatus()

		if errStatus != nil {
			return errStatus
		}

		return nil
	})

	loadOnlyLocalInformation := false

	if err != nil {
		fmt.Printf("Unable to retrieve network information: %s", err)
		fmt.Println()
		fmt.Println()
		fmt.Println("Loading just local information:")
		loadOnlyLocalInformation = true
	}

	vir, err := cli.RouterInspect(context.Background())
	if err != nil || vir == nil {
		fmt.Printf("The router configuration is not available: %s", err)
		fmt.Println()
		return nil
	}

	siteConfig, err := cli.SiteConfigInspect(nil, nil)
	if err != nil || siteConfig == nil {
		fmt.Printf("The site configuration is not available: %s", err)
		fmt.Println()
		return nil
	}

	currentSite := siteConfig.Reference.UID

	if loadOnlyLocalInformation {
		printLocalStatus(vir.Status.TransportReadyReplicas, vir.Status.ConnectedSites.Warnings, vir.Status.ConnectedSites.Total, vir.Status.ConnectedSites.Direct, vir.ExposedServices)

		serviceInterfaces, err := cli.ServiceInterfaceList(context.Background())
		if err != nil {
			fmt.Printf("Service local configuration is not available: %s", err)
			fmt.Println()
			return nil
		}

		sites = getLocalSiteInfo(serviceInterfaces, currentSite, vir.Status.SiteName, cli.GetNamespace(), vir.TransportVersion)
	}

	if sites != nil && len(sites) > 0 {
		siteList := formatter.NewList()
		siteList.Item("Sites:")
		for _, site := range sites {

			if site.Name != selectedSite && selectedSite != "all" {
				continue
			}

			location := "remote"
			siteVersion := site.Version
			detailsMap := map[string]string{"name": site.Name, "namespace": site.Namespace, "URL": site.Url, "version": siteVersion}

			if len(site.MinimumVersion) > 0 {
				siteVersion = fmt.Sprintf("%s (minimum version required %s)", site.Version, site.MinimumVersion)
			}

			if site.SiteId == currentSite {
				location = "local"
				detailsMap["mode"] = vir.Status.Mode
			}

			newItem := fmt.Sprintf("[%s] %s - %s ", location, site.SiteId[:7], site.Name)

			newItem = newItem + fmt.Sprintln()

			if len(site.Links) > 0 {
				detailsMap["sites linked to"] = fmt.Sprint(strings.Join(site.Links, ", "))
			}

			serviceLevel := siteList.NewChildWithDetail(newItem, detailsMap)
			if len(site.Services) > 0 {
				services := serviceLevel.NewChild("Services:")
				var addresses []string
				svcAuth := map[string]bool{}
				for _, svc := range site.Services {
					addresses = append(addresses, svc.Name)
					svcAuth[svc.Name] = true
				}
				if vc, ok := cli.(*client.VanClient); ok && site.Namespace == cli.GetNamespace() {
					policy := client.NewPolicyValidatorAPI(vc)
					res, _ := policy.Services(addresses...)
					for addr, auth := range res {
						svcAuth[addr] = auth.Allowed
					}
				}
				for _, svc := range site.Services {
					authSuffix := ""
					if !svcAuth[svc.Name] {
						authSuffix = " - not authorized"
					}
					svcItem := "name: " + svc.Name + authSuffix + fmt.Sprintln()
					detailsSvc := map[string]string{"protocol": svc.Protocol, "address": svc.Address}
					targetLevel := services.NewChildWithDetail(svcItem, detailsSvc)

					if len(svc.Targets) > 0 {
						targets := targetLevel.NewChild("Targets:")
						for _, target := range svc.Targets {
							targets.NewChild("name: " + target.Name)

						}
					}

				}
			}
		}

		siteList.Print()
	}

	return nil
}

func (s *SkupperKube) ListConnectors(cmd *cobra.Command, args []string) error {
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
}

func (s *SkupperKube) Version(cmd *cobra.Command, args []string) error {
	if !IsZero(reflect.ValueOf(cli)) {
		fmt.Printf("%-30s %s\n", "transport version", cli.GetVersion(types.TransportComponentName, types.TransportContainerName))
		fmt.Printf("%-30s %s\n", "controller version", cli.GetVersion(types.ControllerComponentName, types.ControllerContainerName))
		fmt.Printf("%-30s %s\n", "config-sync version", cli.GetVersion(types.TransportComponentName, types.ConfigSyncContainerName))
	} else {
		fmt.Printf("%-30s %s\n", "transport version", "not-found (no configuration has been provided)")
		fmt.Printf("%-30s %s\n", "controller version", "not-found (no configuration has been provided)")
	}
	return nil
}

func (s *SkupperKube) Status(cmd *cobra.Command, args []string) error {
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
}

func (s *SkupperKube) Update(cmd *cobra.Command, args []string) error {
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
}

func (s *SkupperKube) Delete(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)
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

func (s *SkupperKube) SupportedCommands() []string {
	return SkupperKubeCommands
}

func (s *SkupperKube) Platform() types.Platform {
	return types.PlatformKubernetes
}

func (s *SkupperKube) Options(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().StringVarP(&s.KubeConfigPath, "kubeconfig", "", "", "Path to the kubeconfig file to use")
	rootCmd.PersistentFlags().StringVarP(&s.KubeContext, "context", "c", "", "The kubeconfig context to use")
	rootCmd.PersistentFlags().StringVarP(&s.Namespace, "namespace", "n", "", "The Kubernetes namespace to use")
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
	s.Cli = NewClientHandleError(s.Namespace, s.KubeContext, s.KubeConfigPath, exitOnError)
	// TODO remove once all methods converted
	cli = s.Cli
}

func (s *SkupperKube) Init(cmd *cobra.Command, args []string) error {
	cli := s.Cli

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
	s.kubeInit = kubeInit{}
	s.kubeInit.ingressAnnotations = []string{}
	s.kubeInit.annotations = []string{}
	s.kubeInit.routerServiceAnnotations = []string{}
	s.kubeInit.controllerServiceAnnotations = []string{}
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
