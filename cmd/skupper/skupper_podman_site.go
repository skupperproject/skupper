package main

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/site_podman"
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
		siteName = site_podman.Username
	}
	// fmt.Printf("site name         : %s\n", siteName)
	// fmt.Printf("mode              : %s\n", initFlags.routerMode)
	// fmt.Printf("platform          : %s\n", types.PlatformPodman)
	// fmt.Printf("ingress           : %s\n", routerCreateOpts.Ingress)
	// fmt.Printf("ingress-host      : %s\n", routerCreateOpts.IngressHost)
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
	site := &site_podman.SitePodman{
		SiteCommon: &domain.SiteCommon{
			Name:     siteName,
			Mode:     initFlags.routerMode,
			Platform: types.PlatformPodman,
		},
		IngressBindHost:            routerCreateOpts.IngressHost,
		IngressBindInterRouterPort: s.flags.IngressBindInterRouterPort,
		IngressBindEdgePort:        s.flags.IngressBindEdgePort,
		ContainerNetwork:           s.flags.ContainerNetwork,
		PodmanEndpoint:             s.flags.PodmanEndpoint,
	}

	siteHandler, err := site_podman.NewSitePodmanHandler(site.PodmanEndpoint)
	if err != nil {
		return fmt.Errorf("Unable to initialize Skupper - %w", err)
	}

	// Validating if site is already initialized
	curSite, err := siteHandler.Get()
	if err == nil && curSite != nil {
		return fmt.Errorf("Skupper has already been initialized for user '" + site_podman.Username + "'.")
	}

	// Initializing
	err = siteHandler.Create(site)
	if err != nil {
		return fmt.Errorf("Error initializing Skupper - %w", err)
	}

	fmt.Println("Skupper is now installed for user '" + site_podman.Username + "'.  Use 'skupper status' to get more information.")
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
	siteHandler, err := site_podman.NewSitePodmanHandler("")
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
	fmt.Println("Skupper is now removed for user '" + site_podman.Username + "'.")
	return nil
}

func (s *SkupperPodmanSite) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) List(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanSite) ListFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanSite) Status(cmd *cobra.Command, args []string) error {
	return notImplementedErr
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
	siteHandler, err := site_podman.NewSitePodmanHandler("")
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
