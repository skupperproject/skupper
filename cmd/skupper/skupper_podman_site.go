package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils/formatter"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	podman "github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

type SkupperPodmanSite struct {
	podman *SkupperPodman
	flags  PodmanInitFlags
	up     *domain.UpdateProcessor
}

type PodmanInitFlags struct {
	IngressHosts                 []string
	IngressBindIPs               []string
	IngressBindInterRouterPort   int
	IngressBindEdgePort          int
	IngressBindFlowCollectorPort int
	ContainerNetwork             string
	EnableIPV6                   bool
	PodmanEndpoint               string
	Timeout                      time.Duration
}

func (s *SkupperPodmanSite) Create(cmd *cobra.Command, args []string) error {
	siteName := routerCreateOpts.SkupperName
	if siteName == "" {
		var err error
		siteName, err = getUserDefaultPodmanName()
		if err != nil {
			return err
		}
	}

	// Validating ingress mode
	routerCreateOpts.Platform = types.PlatformPodman
	if err := routerCreateOpts.CheckIngress(); err != nil {
		return err
	}

	// Site initialization
	site := &podman.Site{
		SiteCommon: &domain.SiteCommon{
			Name:     siteName,
			Mode:     initFlags.routerMode,
			Platform: types.PlatformPodman,
		},
		IngressHosts:                 s.flags.IngressHosts,
		IngressBindIPs:               s.flags.IngressBindIPs,
		IngressBindInterRouterPort:   s.flags.IngressBindInterRouterPort,
		IngressBindEdgePort:          s.flags.IngressBindEdgePort,
		IngressBindFlowCollectorPort: s.flags.IngressBindFlowCollectorPort,
		ContainerNetwork:             s.flags.ContainerNetwork,
		EnableIPV6:                   s.flags.EnableIPV6,
		PodmanEndpoint:               s.flags.PodmanEndpoint,
		EnableFlowCollector:          routerCreateOpts.EnableFlowCollector,
		EnableConsole:                routerCreateOpts.EnableConsole,
		AuthMode:                     routerCreateOpts.AuthMode,
		ConsoleUser:                  routerCreateOpts.User,
		ConsolePassword:              routerCreateOpts.Password,
		RouterOpts:                   routerCreateOpts.Router,
		ControllerOpts:               routerCreateOpts.Controller,
		FlowCollectorOpts:            routerCreateOpts.FlowCollector,
		PrometheusOpts:               routerCreateOpts.PrometheusServer,
	}

	siteHandler, err := podman.NewSitePodmanHandler(site.PodmanEndpoint)
	if err != nil {
		initErr := fmt.Errorf("Unable to initialize Skupper - %w", err)

		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		fmt.Println("Error:", initErr)
		return initErr
	}

	// Validating ingress type
	if routerCreateOpts.Ingress != types.IngressNoneString {
		// Validating ingress hosts (required as certificates must have valid hosts)
		if len(site.IngressHosts) == 0 {
			var ingressHosts []string
			// Get hostname of the machine
			hostname, hostnameErr := os.Hostname()
			if hostnameErr == nil {
				ingressHosts = append(ingressHosts, hostname)
			}
			// Get all system's unicast interface addresses
			addresses, addressesErr := net.InterfaceAddrs()
			if addressesErr == nil {
				for _, address := range addresses {
					ipnet, ok := address.(*net.IPNet)
					if ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
						ipv4ValidAddress := ipnet.IP.String()
						ingressHosts = append(ingressHosts, ipv4ValidAddress)
						// Try a reverse lookup of a valid IPv4 address
						fqdns, err := net.LookupAddr(ipv4ValidAddress)
						if err == nil {
							for _, fqdn := range fqdns {
								if !utils.StringSliceContains(ingressHosts, fqdn) {
									ingressHosts = append(ingressHosts, fqdn)
								}
							}
						}
					}
				}
			}
			if addressesErr != nil && hostnameErr != nil {
				return fmt.Errorf("Cannot get a default ingress host")
			}
			site.IngressHosts = append(site.IngressHosts, ingressHosts...)
		}
	} else {
		// If none is set, do not allow any ingress host (ignore those provided via CLI)
		site.IngressHosts = []string{}
	}

	// Initializing
	cmd.SilenceUsage = true
	ctx, cn := context.WithTimeout(context.Background(), s.flags.Timeout)
	defer cn()
	err = siteHandler.Create(ctx, site)
	if err != nil {
		return fmt.Errorf("Error initializing Skupper - %w", err)
	}

	err = utils.NewSpinner("Waiting for status...", 50, func() error {
		networkStatusHandler := new(podman.NetworkStatusHandler).WithClient(s.podman.cli)
		if networkStatusHandler != nil {
			statusInfo, statusError := networkStatusHandler.Get()
			if statusError != nil {
				return statusError
			} else if statusInfo == nil || len(statusInfo.SiteStatus) == 0 || len(statusInfo.SiteStatus[0].RouterStatus) == 0 {
				return fmt.Errorf("network status not loaded yet")
			}
		}

		return nil
	})

	if err != nil {
		fmt.Println("Skupper status is not loaded yet.")
	}

	fmt.Println("Skupper is now installed for user '" + podman.Username + "'.  Use 'skupper status' to get more information.")
	return nil
}

func getUserDefaultPodmanName() (string, error) {
	hostname, _ := os.Hostname()
	defaultName := fmt.Sprintf("%s-%s-%s", hostname, strings.ToLower(podman.Username), uuid.NewString()[:5])
	return defaultName, nil
}

func (s *SkupperPodmanSite) CreateFlags(cmd *cobra.Command) {
	// --ingress-host (multiple)
	cmd.Flags().StringSliceVarP(&s.flags.IngressHosts, "ingress-host", "", []string{},
		"Hostname or alias by which the ingress route or proxy can be reached.\n"+
			"Tokens can only be generated for addresses provided through ingress-hosts,\n"+
			"so it can be used multiple times.",
	)

	// --ingress-bind-ip
	cmd.Flags().StringSliceVarP(&s.flags.IngressBindIPs, "ingress-bind-ip", "", []string{},
		"IP addresses in the host machines that will be bound to the inter-router and edge ports.")

	// --bind-port (interior)
	cmd.Flags().IntVar(&s.flags.IngressBindInterRouterPort, "bind-port", int(types.InterRouterListenerPort),
		"ingress host binding port used for incoming links from sites using interior mode")
	// --bind-port-edge
	cmd.Flags().IntVar(&s.flags.IngressBindEdgePort, "bind-port-edge", int(types.EdgeListenerPort),
		"ingress host binding port used for incoming links from sites using edge mode")
	// --container-network
	cmd.Flags().StringVar(&s.flags.ContainerNetwork, "container-network", container.ContainerNetworkName,
		"container network name to be used")
	// --enable-ipv6
	cmd.Flags().BoolVarP(&s.flags.EnableIPV6, "enable-ipv6", "", false,
		"Enable IPV6 on the container network to be created (ignored when using an existing container network)")
	// --podman-endpoint
	cmd.Flags().StringVar(&s.flags.PodmanEndpoint, "podman-endpoint", "",
		"local podman endpoint to use")

	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console must be used in conjunction with '--enable-flow-collector' flag")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "internal", "Authentication mode for console(s). One of: 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console password. Valid only when --console-auth=internal")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableFlowCollector, "enable-flow-collector", "", false, "Enable cross-site flow collection for the application network")
	// --bind-port-flow-collector
	cmd.Flags().IntVar(&s.flags.IngressBindFlowCollectorPort, "bind-port-flow-collector", int(types.FlowCollectorDefaultServicePort),
		"ingress host binding port used for flow-collector and console")
	cmd.Flags().DurationVar(&routerCreateOpts.FlowCollector.FlowRecordTtl, "flow-collector-record-ttl", 0, "Time after which terminated flow records are deleted, i.e. those flow records that have an end time set. Default is 30 minutes.")

	// limits
	cmd.Flags().StringVar(&routerCreateOpts.Router.CpuLimit, "router-cpu-limit", "", "CPU limit for router container (decimal)")
	cmd.Flags().StringVar(&routerCreateOpts.Router.MemoryLimit, "router-memory-limit", "", "Memory limit for router container (bytes)")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.CpuLimit, "controller-cpu-limit", "", "CPU limit for controller container (decimal)")
	cmd.Flags().StringVar(&routerCreateOpts.Controller.MemoryLimit, "controller-memory-limit", "", "Memory limit for controller container (bytes)")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.CpuLimit, "flow-collector-cpu-limit", "", "CPU limit for flow collector container (decimal)")
	cmd.Flags().StringVar(&routerCreateOpts.FlowCollector.MemoryLimit, "flow-collector-memory-limit", "", "Memory limit for flow collector container (bytes)")
	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.CpuLimit, "prometheus-cpu-limit", "", "CPU limit for prometheus container (decimal)")
	cmd.Flags().StringVar(&routerCreateOpts.PrometheusServer.MemoryLimit, "prometheus-memory-limit", "", "Memory limit for prometheus container (bytes)")

	cmd.Flags().DurationVar(&s.flags.Timeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for site initialization")

}

func (s *SkupperPodmanSite) Delete(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("Unable to delete Skupper - %w", err)
	}
	if s.podman.currentSite == nil && !siteHandler.AnyResourceLeft() {
		fmt.Printf("Skupper is not enabled for user '%s'", podman.Username)
		fmt.Println()
		return nil
	}
	err = siteHandler.Delete()
	if err != nil {
		return err
	}
	fmt.Println("Skupper is now removed for user '" + podman.Username + "'.")
	return nil
}

func (s *SkupperPodmanSite) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) List(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanSite) ListFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) Status(cmd *cobra.Command, args []string) error {
	silenceCobra(cmd)

	siteHandler, err := podman.NewSitePodmanHandler("")

	podmanSiteVersion := s.podman.currentSite.Version
	if podmanSiteVersion != "" && !utils.IsValidFor(podmanSiteVersion, network.MINIMUM_PODMAN_VERSION) {
		fmt.Printf(network.MINIMUM_VERSION_MESSAGE, podmanSiteVersion, network.MINIMUM_PODMAN_VERSION)
		fmt.Println()
		return nil
	}

	svcIfaceHandler := podman.NewServiceInterfaceHandlerPodman(s.podman.cli)
	localServices, err := svcIfaceHandler.List()
	if err != nil {
		return fmt.Errorf("error retrieving service list - %w", err)
	}

	currentStatus, errStatus := siteHandler.NetworkStatusHandler().Get()
	if errStatus != nil && strings.HasPrefix(errStatus.Error(), "Skupper is not installed") {
		fmt.Printf("Skupper is not enabled\n")
		return nil
	} else if errStatus != nil && errStatus.Error() == "status not ready" {
		fmt.Println("Status pending...")
		return nil
	} else if errStatus != nil {
		return errStatus
	}

	statusManager := network.SkupperStatus{
		NetworkStatus: currentStatus,
	}

	site := s.podman.currentSite

	// Preparing output
	statusOutput := formatter.StatusData{}

	if site.GetName() != "" && site.GetName() != podman.Username {
		statusOutput.SiteName = site.GetName()
	}

	statusOutput.Mode = site.GetMode()
	statusOutput.EnabledIn = formatter.PlatformSupport{
		SupportType: "podman",
		SupportName: podman.Username,
	}

	var currentSite = statusManager.GetSiteById(site.Id)
	if currentSite == nil {
		fmt.Println("Site information is not yet available")
		return nil
	}

	err, index := statusManager.GetRouterIndex(currentSite)
	if err != nil {
		return err
	}

	peerSites := statusManager.GetPeerSites(&currentSite.RouterStatus[index], currentSite.Site.Identity)

	totalSites := len(currentStatus.SiteStatus)
	// the current site does not count as a connection
	connections := totalSites - 1
	directConnections := len(peerSites)
	statusOutput.TotalConnections = connections
	statusOutput.DirectConnections = directConnections
	statusOutput.IndirectConnections = connections - directConnections

	statusOutput.ExposedServices = len(localServices)

	if site.EnableFlowCollector {
		statusOutput.ConsoleUrl = site.GetConsoleUrl()
		if site.AuthMode == "internal" {
			statusOutput.Credentials = formatter.PlatformSupport{
				SupportType: "podman volume",
				SupportName: "'skupper-console-users'",
			}
		}
	}

	if verboseStatus {
		err := formatter.PrintVerboseStatus(statusOutput)
		if err != nil {
			return err
		}
	} else {
		err := formatter.PrintStatus(statusOutput)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SkupperPodmanSite) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) NewClient(cmd *cobra.Command, args []string) {
	var initArgs []string
	if cmd.Name() == "init" && len(s.flags.PodmanEndpoint) > 0 {
		initArgs = append(initArgs, s.flags.PodmanEndpoint)
	}
	s.podman.NewClient(cmd, initArgs)
}

func (s *SkupperPodmanSite) Platform() types.Platform {
	return s.podman.Platform()
}

func (s *SkupperPodmanSite) Update(cmd *cobra.Command, args []string) error {
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("Unable to communicate with Skupper site - %w", err)
	}
	siteHandler.SetUpdateProcessor(s.up)
	return siteHandler.Update()
}

func (s *SkupperPodmanSite) UpdateFlags(cmd *cobra.Command) {
	s.up = &domain.UpdateProcessor{}
	cmd.Flags().BoolVar(&s.up.DryRun, "dry-run", false, "only prints the tasks to be performed, but does not run any action")
	cmd.Flags().BoolVar(&s.up.Verbose, "verbose", false, "displays tasks and post tasks being executed")
	cmd.Flags().DurationVar(&s.up.Timeout, "timeout", types.DefaultTimeoutDuration, "Configurable timeout for site update")
}

func (s *SkupperPodmanSite) Version(cmd *cobra.Command, args []string) error {
	site := s.podman.currentSite
	if site == nil {
		fmt.Printf("Skupper is not enabled for user '%s'", podman.Username)
		fmt.Println()
		return nil
	}
	for _, deploy := range site.GetDeployments() {
		for _, component := range deploy.GetComponents() {
			if component.Name() == types.TransportDeploymentName {
				img, err := s.podman.cli.ImageInspect(component.GetImage())
				if err != nil {
					return fmt.Errorf("error retrieving image info for %s - %w", component.GetImage(), err)
				}
				fmt.Printf("%-30s %s (%s)\n", "transport version", img.Repository, img.Digest[:19])
			}
		}
	}
	return nil
}

func (s *SkupperPodmanSite) RevokeAccess(cmd *cobra.Command, args []string) error {
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("Unable to communicate with Skupper site - %w", err)
	}
	return siteHandler.RevokeAccess()
}
