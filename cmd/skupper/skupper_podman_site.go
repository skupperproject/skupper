package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	podman "github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/spf13/cobra"
)

type SkupperPodmanSite struct {
	podman *SkupperPodman
	flags  PodmanInitFlags
}

type PodmanInitFlags struct {
	IngressHosts                 []string
	IngressBindIPs               []string
	IngressBindInterRouterPort   int
	IngressBindEdgePort          int
	IngressBindFlowCollectorPort int
	ContainerNetwork             string
	PodmanEndpoint               string
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
		PodmanEndpoint:               s.flags.PodmanEndpoint,
		EnableFlowCollector:          routerCreateOpts.EnableFlowCollector,
		EnableConsole:                routerCreateOpts.EnableConsole,
		AuthMode:                     routerCreateOpts.AuthMode,
		ConsoleUser:                  routerCreateOpts.User,
		ConsolePassword:              routerCreateOpts.Password,
		FlowCollectorRecordTtl:       routerCreateOpts.FlowCollector.FlowRecordTtl,
		RouterOpts:                   routerCreateOpts.Router,
	}

	siteHandler, err := podman.NewSitePodmanHandler(site.PodmanEndpoint)
	if err != nil {
		return fmt.Errorf("Unable to initialize Skupper - %w", err)
	}

	// Validating ingress type
	if routerCreateOpts.Ingress != types.IngressNoneString {
		// Validating ingress hosts (required as certificates must have valid hosts)
		if len(site.IngressHosts) == 0 {
			return fmt.Errorf("At least one ingress host is required")
		}
	} else {
		// If none is set, do not allow any ingress host (ignore those provided via CLI)
		site.IngressHosts = []string{}
	}

	// Initializing
	err = siteHandler.Create(site)
	if err != nil {
		return fmt.Errorf("Error initializing Skupper - %w", err)
	}

	fmt.Println("Skupper is now installed for user '" + podman.Username + "'.  Use 'skupper status' to get more information.")
	return nil
}

func getUserDefaultPodmanName() (string, error) {
	hostname, _ := os.Hostname()
	return hostname + "-" + strings.ToLower(podman.Username), nil
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
	// --podman-endpoint
	cmd.Flags().StringVar(&s.flags.PodmanEndpoint, "podman-endpoint", "",
		"local podman endpoint to use")

	cmd.Flags().BoolVarP(&routerCreateOpts.EnableConsole, "enable-console", "", false, "Enable skupper console must be used in conjunction with '--enable-flow-collector' flag")
	cmd.Flags().StringVarP(&routerCreateOpts.AuthMode, "console-auth", "", "", "Authentication mode for console(s). One of: 'internal', 'unsecured'")
	cmd.Flags().StringVarP(&routerCreateOpts.User, "console-user", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().StringVarP(&routerCreateOpts.Password, "console-password", "", "", "Skupper console user. Valid only when --console-auth=internal")
	cmd.Flags().BoolVarP(&routerCreateOpts.EnableFlowCollector, "enable-flow-collector", "", false, "Enable cross-site flow collection for the application network")
	// --bind-port-flow-collector
	cmd.Flags().IntVar(&s.flags.IngressBindFlowCollectorPort, "bind-port-flow-collector", int(types.FlowCollectorDefaultServicePort),
		"ingress host binding port used for flow-collector and console")
	cmd.Flags().DurationVar(&routerCreateOpts.FlowCollector.FlowRecordTtl, "flow-collector-record-ttl", 0, "Time after which terminated flow records are deleted, i.e. those flow records that have an end time set. Default is 30 minutes.")

}

func (s *SkupperPodmanSite) Delete(cmd *cobra.Command, args []string) error {
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("Unable to delete Skupper - %w", err)
	}
	curSite, err := siteHandler.Get()
	if err != nil || curSite == nil {
		return err
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
	sitename := ""
	if site.GetName() != "" && site.GetName() != podman.Username {
		sitename = fmt.Sprintf(" with site name %q", site.GetName())
	}
	var modedesc string = " in interior mode"
	if site.GetMode() == string(types.TransportModeEdge) {
		modedesc = " in edge mode"
	}

	fmt.Printf("Skupper is enabled for %q%s%s.", podman.Username, sitename, modedesc)
	if len(connectedSites.Warnings) > 0 {
		for _, w := range connectedSites.Warnings {
			fmt.Printf("Warning: %s", w)
			fmt.Println()
		}
	}
	if connectedSites.Total == 0 {
		fmt.Printf(" It is not connected to any other sites.")
	} else if connectedSites.Total == 1 {
		fmt.Printf(" It is connected to 1 other site.")
	} else if connectedSites.Total == connectedSites.Direct {
		fmt.Printf(" It is connected to %d other sites.", connectedSites.Total)
	} else {
		fmt.Printf(" It is connected to %d other sites (%d indirectly).", connectedSites.Total, connectedSites.Indirect)
	}

	svcIfaceHandler := podman.NewServiceInterfaceHandlerPodman(s.podman.cli)
	list, err := svcIfaceHandler.List()
	if err != nil {
		return fmt.Errorf("error retrieving service list - %w", err)
	}
	if len(list) == 0 {
		fmt.Printf(" It has no exposed services.")
	} else if len(list) == 1 {
		fmt.Printf(" It has 1 exposed service.")
	} else {
		fmt.Printf(" It has %d exposed services.", len(list))
	}

	fmt.Println()

	if site.EnableFlowCollector {
		fmt.Println("The site console url is: ", site.GetConsoleUrl())
		fmt.Println("The credentials for internal console-auth mode are held in podman volume: 'skupper-console-users'")
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
	return notImplementedErr
}

func (s *SkupperPodmanSite) UpdateFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) Version(cmd *cobra.Command, args []string) error {
	site := s.podman.currentSite
	if site == nil {
		return fmt.Errorf("Skupper is not enabled for user '%s'", podman.Username)
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
