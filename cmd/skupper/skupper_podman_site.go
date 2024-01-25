package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	podman "github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/qdr"
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
		FlowCollectorRecordTtl:       routerCreateOpts.FlowCollector.FlowRecordTtl,
		RouterOpts:                   routerCreateOpts.Router,
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
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableFlowCollector, "enable-flow-collector", "", false, "Enable cross-site flow collection for the application network")
	// --bind-port-flow-collector
	cmd.Flags().IntVar(&s.flags.IngressBindFlowCollectorPort, "bind-port-flow-collector", int(types.FlowCollectorDefaultServicePort),
		"ingress host binding port used for flow-collector and console")
	cmd.Flags().DurationVar(&routerCreateOpts.FlowCollector.FlowRecordTtl, "flow-collector-record-ttl", 0, "Time after which terminated flow records are deleted, i.e. those flow records that have an end time set. Default is 30 minutes.")

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
	site := s.podman.currentSite
	routerMgr := podman.NewRouterEntityManagerPodman(s.podman.cli)
	routers, err := routerMgr.QueryAllRouters()
	if err != nil {
		return fmt.Errorf("error verifying network - %w", err)
	}
	connectedSites := qdr.ConnectedSitesInfo(site.GetId(), routers)

	// Preparing output
	statusOutput := StatusData{}

	if site.GetName() != "" && site.GetName() != podman.Username {
		statusOutput.siteName = site.GetName()
	}

	statusOutput.mode = site.GetMode()
	statusOutput.enabledIn = PlatformSupport{"podman", podman.Username}

	if len(connectedSites.Warnings) > 0 {
		var warnings []string
		for _, w := range connectedSites.Warnings {
			warnings = append(warnings, w)
		}

		statusOutput.warnings = warnings
	}

	statusOutput.totalConnections = connectedSites.Total
	statusOutput.directConnections = connectedSites.Direct
	statusOutput.indirectConnections = connectedSites.Indirect

	svcIfaceHandler := podman.NewServiceInterfaceHandlerPodman(s.podman.cli)
	list, err := svcIfaceHandler.List()
	if err != nil {
		return fmt.Errorf("error retrieving service list - %w", err)
	}

	statusOutput.exposedServices = len(list)

	if site.EnableFlowCollector {
		statusOutput.consoleUrl = site.GetConsoleUrl()
		statusOutput.credentials = PlatformSupport{"podman volume", "'skupper-console-users'"}
	}

	if verboseStatus {
		err := PrintVerboseStatus(statusOutput)
		if err != nil {
			return err
		}
	} else {
		err := PrintStatus(statusOutput)
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
