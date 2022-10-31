package main

import (
	"fmt"

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
	IngressBindInterRouterPort int
	IngressBindEdgePort        int
	ContainerNetwork           string
	PodmanEndpoint             string
}

func (s *SkupperPodmanSite) Create(cmd *cobra.Command, args []string) error {
	siteName := routerCreateOpts.SkupperName
	if siteName == "" {
		siteName = podman.Username
	}
	// fmt.Printf("site name         : %s\n", siteName)
	// fmt.Printf("mode              : %s\n", initFlags.routerMode)
	// fmt.Printf("platform          : %s\n", types.PlatformPodman)
	// fmt.Printf("ingress           : %s\n", routerCreateOpts.Ingress)
	// fmt.Printf("ingress-host      : %s\n", routerCreateOpts.IngressHost)
	// fmt.Printf("router-logging    : %s\n", routerCreateOpts.Router.Logging)
	// fmt.Printf("debug-mode        : %s\n", routerCreateOpts.Router.DebugMode)
	// fmt.Printf("inter-router-port : %d\n", s.flags.IngressBindInterRouterPort)
	// fmt.Printf("edge-port         : %d\n", s.flags.IngressBindEdgePort)
	// fmt.Printf("container-network : %s\n", s.flags.ContainerNetwork)
	// fmt.Printf("podman-endpoint   : %s\n", s.flags.PodmanEndpoint)

	// Validating ingress mode
	routerCreateOpts.Platform = types.PlatformPodman
	if err := routerCreateOpts.CheckIngress(); err != nil {
		return err
	}

	// Site initialization
	site := &podman.SitePodman{
		SiteCommon: &domain.SiteCommon{
			Name:     siteName,
			Mode:     initFlags.routerMode,
			Platform: types.PlatformPodman,
		},
		RouterOpts:                 routerCreateOpts.Router,
		IngressBindHost:            routerCreateOpts.IngressHost,
		IngressBindInterRouterPort: s.flags.IngressBindInterRouterPort,
		IngressBindEdgePort:        s.flags.IngressBindEdgePort,
		ContainerNetwork:           s.flags.ContainerNetwork,
		PodmanEndpoint:             s.flags.PodmanEndpoint,
	}

	siteHandler, err := podman.NewSitePodmanHandler(site.PodmanEndpoint)
	if err != nil {
		return fmt.Errorf("Unable to initialize Skupper - %w", err)
	}

	// Validating if site is already initialized
	curSite, err := siteHandler.Get()
	if err == nil && curSite != nil {
		return fmt.Errorf("Skupper has already been initialized for user '" + podman.Username + "'.")
	}

	// Initializing
	err = siteHandler.Create(site)
	if err != nil {
		return fmt.Errorf("Error initializing Skupper - %w", err)
	}

	fmt.Println("Skupper is now installed for user '" + podman.Username + "'.  Use 'skupper status' to get more information.")
	return nil
}

func (s *SkupperPodmanSite) CreateFlags(cmd *cobra.Command) {
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
	siteHandler, err := podman.NewSitePodmanHandler("")
	site, err := siteHandler.Get()
	if err != nil {
		fmt.Printf("Skupper is not enabled for '%s'\n", podman.Username)
		return nil
	}

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
	return nil
}

func (s *SkupperPodmanSite) StatusFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) NewClient(cmd *cobra.Command, args []string) {
	s.podman.NewClient(cmd, args)
}

func (s *SkupperPodmanSite) Platform() types.Platform {
	return s.podman.Platform()
}

func (s *SkupperPodmanSite) Update(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanSite) UpdateFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) Version(cmd *cobra.Command, args []string) error {
	siteHandler, err := podman.NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("Unable to communicate with Skupper - %w", err)
	}

	site, err := siteHandler.Get()
	if err != nil {
		return fmt.Errorf("Unable to retrieve site information - %w", err)
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
	return notImplementedErr
}
