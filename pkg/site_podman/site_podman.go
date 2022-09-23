package site_podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SitePodman struct {
	*domain.SiteCommon
	IngressBindHost            string
	IngressBindInterRouterPort int
	IngressBindEdgePort        int
	ContainerNetwork           string
	PodmanEndpoint             string
}

func (s *SitePodman) GetPlatform() string {
	return "podman"
}

func (s *SitePodman) GetIngressClasses() []string {
	return []string{"host"}
}

type SitePodmanHandler struct {
	cli *podman.PodmanRestClient
}

func NewSitePodmanHandler(endpoint, basePath string) (*SitePodmanHandler, error) {
	c, err := podman.NewPodmanClient(endpoint, basePath)
	if err != nil {
		return nil, err
	}

	return &SitePodmanHandler{
		cli: c,
	}, nil
}

func (s *SitePodmanHandler) prepare(site domain.Site) (domain.Site, error) {
	podmanSite, ok := site.(*SitePodman)
	if !ok {
		return nil, fmt.Errorf("not a valid podman site definition")
	}

	// Validating basic info
	if err := podmanSite.ValidateMinimumRequirements(); err != nil {
		return nil, err
	}

	// Preparing site
	domain.ConfigureSiteCredentials(podmanSite, podmanSite.IngressBindHost)
	s.ConfigurePodmanDeployments(podmanSite)

	if err := s.canCreate(podmanSite); err != nil {
		return nil, err
	}

	return podmanSite, nil
}

func (s *SitePodmanHandler) ConfigurePodmanDeployments(site *SitePodman) {
	// Router Deployment
	volumeMounts := map[string]string{
		types.LocalServerSecret:      "/etc/skupper-router-certs/skupper-amqps/",
		types.TransportConfigMapName: "/etc/skupper-router/config/",
		"skupper-router-certs":       "/etc/skupper-router-certs",
	}
	if !site.IsEdge() {
		volumeMounts[types.SiteServerSecret] = "/etc/skupper-router-certs/skupper-internal/"
	}
	routerDepl := &SkupperDeploymentPodman{
		Name: types.TransportDeploymentName,
		SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{
			Components: []domain.SkupperComponent{
				&domain.Router{
					// TODO ADD Labels
					Labels: map[string]string{},
					Env: map[string]string{
						"APPLICATION_NAME":    "skupper-router",
						"QDROUTERD_CONF":      "/etc/skupper-router/config/" + types.TransportConfigFile,
						"QDROUTERD_CONF_TYPE": "json",
						"SKUPPER_SITE_ID":     site.Id,
					},
					SiteIngresses: []domain.SiteIngress{
						&SiteIngressPodmanHost{
							SiteIngressCommon: &domain.SiteIngressCommon{
								Name: types.InterRouterIngressPrefix,
								Host: site.IngressBindHost,
								Port: site.IngressBindInterRouterPort,
								Target: &domain.PortCommon{
									Name: types.InterRouterIngressPrefix,
									Port: int(types.InterRouterListenerPort),
								},
							},
						},
						&SiteIngressPodmanHost{
							SiteIngressCommon: &domain.SiteIngressCommon{
								Name: types.EdgeIngressPrefix,
								Host: site.IngressBindHost,
								Port: site.IngressBindEdgePort,
								Target: &domain.PortCommon{
									Name: types.EdgeIngressPrefix,
									Port: int(types.EdgeListenerPort),
								},
							},
						},
					},
				},
			},
		},
		Aliases:      []string{types.TransportServiceName, types.LocalTransportServiceName},
		VolumeMounts: volumeMounts,
		Networks:     []string{site.ContainerNetwork},
	}
	site.Deployments = append(site.Deployments, routerDepl)
}

func (s *SitePodmanHandler) Create(site domain.Site) error {
	var err error
	var cleanupFns []func()

	var preparedSite domain.Site
	podmanSite := site.(*SitePodman)
	preparedSite, err = s.prepare(podmanSite)
	if err != nil {
		return err
	}
	podmanSite = preparedSite.(*SitePodman)

	// cleanup on error
	defer func() {
		if err != nil {
			for _, fn := range cleanupFns {
				fn()
			}
		}
	}()

	// Create network
	err = s.createNetwork(podmanSite)
	if err != nil {
		return err
	}
	cleanupFns = append(cleanupFns, func() {
		_ = s.cli.NetworkRemove(podmanSite.ContainerNetwork)
	})

	// Create cert authorities and credentials
	var credHandler types.CredentialHandler
	credHandler = NewPodmanCredentialHandler(s.cli)

	// - creating cert authorities
	cleanupFns = append(cleanupFns, func() {
		for _, ca := range podmanSite.GetCertAuthorities() {
			_ = credHandler.DeleteCertAuthority(ca.Name)
		}
	})
	for _, ca := range podmanSite.GetCertAuthorities() {
		if _, err = credHandler.NewCertAuthority(ca); err != nil {
			return err
		}
	}

	// - create credentials
	cleanupFns = append(cleanupFns, func() {
		for _, cred := range podmanSite.GetCredentials() {
			_ = credHandler.DeleteCredential(cred.Name)
		}
	})
	for _, cred := range podmanSite.GetCredentials() {
		if _, err = credHandler.NewCredential(cred); err != nil {
			return err
		}
	}

	// Create initial transport config file
	// TODO add log and debug options
	initialRouterConfig := qdr.InitialConfigSkupperRouter(podmanSite.GetName(), podmanSite.GetId(), client.Version, podmanSite.IsEdge(), 3, types.RouterOptions{})
	var routerConfigHandler qdr.RouterConfigHandler
	routerConfigHandler = NewRouterConfigHandlerPodman(s.cli)
	err = routerConfigHandler.SaveRouterConfig(&initialRouterConfig)
	cleanupFns = append(cleanupFns, func() {
		_ = routerConfigHandler.RemoveRouterConfig()
	})
	if err != nil {
		return err
	}

	// Verify volumes not yet created and create them
	for _, volumeName := range SkupperContainerVolumes {
		var vol *container.Volume
		vol, err = s.cli.VolumeInspect(volumeName)
		if vol == nil && err != nil {
			_, err = s.cli.VolumeCreate(&container.Volume{Name: volumeName})
			if err != nil {
				return err
			}
			cleanupFns = append(cleanupFns, func() {
				_ = s.cli.VolumeRemove(volumeName)
			})
		}
	}

	// Deploy container(s)
	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	for _, depl := range podmanSite.GetDeployments() {
		err = deployHandler.Deploy(depl)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *SitePodmanHandler) canCreate(site *SitePodman) error {

	// Validating podman endpoint
	if s.cli == nil {
		cli, err := podman.NewPodmanClient(site.PodmanEndpoint, "")
		if err != nil {
			// TODO try to start podman's user service instance?
			return fmt.Errorf("unable to communicate with podman service through %s - %v", site.PodmanEndpoint, err)
		}
		s.cli = cli
	}

	// Validate podman version
	cli := s.cli
	if err := cli.Validate(); err != nil {
		return err
	}

	// TODO improve on container and network already exists
	// Validating any of the required deployment exists
	for _, skupperDepl := range site.Deployments {
		container, err := cli.ContainerInspect(skupperDepl.GetName())
		if err == nil && container != nil {
			return fmt.Errorf("%s container already defined", skupperDepl.GetName())
		}
	}

	// Validating skupper networks available
	net, err := cli.NetworkInspect(site.ContainerNetwork)
	if err == nil && net != nil {
		return fmt.Errorf("network %s already exists", site.ContainerNetwork)
	}

	// Validating bind ports
	for _, skupperDepl := range site.GetDeployments() {
		for _, skupperComp := range skupperDepl.GetComponents() {
			for _, ingress := range skupperComp.GetSiteIngresses() {
				if utils.TcpPortInUse(ingress.GetHost(), ingress.GetPort()) {
					return fmt.Errorf("ingress port already bound %s:%d", ingress.GetHost(), ingress.GetPort())
				}

			}
		}
	}

	// Validate network ability to resolve names
	createdNetwork, err := cli.NetworkCreate(&container.Network{
		Name:     site.ContainerNetwork,
		DNS:      true,
		Internal: false,
	})
	if err != nil {
		return fmt.Errorf("error validating network creation - %v", err)
	}
	defer func(cli *podman.PodmanRestClient, id string) {
		err := cli.NetworkRemove(id)
		if err != nil {
			fmt.Printf("ERROR removing network %s - %v\n", id, err)
		}
	}(cli, site.ContainerNetwork)
	if !createdNetwork.DNS {
		return fmt.Errorf("network %s cannot resolve names - podman plugins must be installed", site.ContainerNetwork)
	}

	// Validating existing volumes
	for _, v := range SkupperContainerVolumes {
		_, err := cli.VolumeInspect(v)
		if err == nil {
			return fmt.Errorf("required volume already exists %s", v)
		}
	}

	return nil
}

func (s *SitePodmanHandler) createNetwork(site *SitePodman) error {
	_, err := s.cli.NetworkCreate(&container.Network{
		Name:     site.ContainerNetwork,
		DNS:      true,
		Internal: false,
	})
	if err != nil {
		return fmt.Errorf("error creating network %s - %v", site.ContainerNetwork, err)
	}
	return nil
}

func (s *SitePodmanHandler) Get() (domain.Site, error) {
	site := &SitePodman{
		SiteCommon:     &domain.SiteCommon{},
		PodmanEndpoint: s.cli.RestClient.Host,
	}

	// getting router config
	configHandler := NewRouterConfigHandlerPodman(s.cli)
	config, err := configHandler.GetRouterConfig()
	if err != nil {
		return nil, fmt.Errorf("Skupper is not enabled for user '%s'", Namespace)
	}

	// Setting basic site info
	site.Name = config.Metadata.Id
	site.Mode = string(config.Metadata.Mode)
	site.Id = config.GetSiteMetadata().Id
	site.Platform = types.PlatformPodman

	// Reading cert authorities
	credHandler := NewPodmanCredentialHandler(s.cli)
	cas, err := credHandler.ListCertAuthorities()
	if err != nil {
		return nil, fmt.Errorf("error reading certificate authorities - %w", err)
	}
	site.CertAuthorities = cas

	// Reading credentials
	creds, err := credHandler.ListCredentials()
	if err != nil {
		return nil, fmt.Errorf("error reading credentials - %w", err)
	}
	site.Credentials = creds

	// Reading deployments
	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	deps, err := deployHandler.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving deployments - %w", err)
	}
	site.Deployments = deps

	for _, dep := range site.GetDeployments() {
		for _, comp := range dep.GetComponents() {
			depPodman := dep.(*SkupperDeploymentPodman)
			site.ContainerNetwork = depPodman.Networks[0]
			for _, siteIng := range comp.GetSiteIngresses() {
				if siteIng.GetTarget().GetPort() == int(types.InterRouterListenerPort) {
					site.IngressBindHost = siteIng.GetHost()
					site.IngressBindInterRouterPort = siteIng.GetPort()
				} else if siteIng.GetTarget().GetPort() == int(types.EdgeListenerPort) {
					site.IngressBindEdgePort = siteIng.GetPort()
				}
			}
		}
	}

	return site, nil
}

func (s *SitePodmanHandler) Delete() error {
	site, err := s.Get()
	if err != nil {
		return err
	}
	podmanSite := site.(*SitePodman)

	// Stopping and removing containers
	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	deploys, err := deployHandler.List()
	if err != nil {
		return fmt.Errorf("error retrieving deployments - %w", err)
	}
	for _, dep := range deploys {
		err = deployHandler.Undeploy(dep.GetName())
		if err != nil {
			return fmt.Errorf("error removing deployment %s - %w", dep.GetName(), err)
		}
	}

	// Removing networks
	if err = s.cli.NetworkRemove(podmanSite.ContainerNetwork); err != nil {
		return fmt.Errorf("error removing container network %s - %w", podmanSite.ContainerNetwork, err)
	}

	// Removing volumes
	volumeList, err := s.cli.VolumeList()
	if err != nil {
		return fmt.Errorf("error retrieving volume list - %w", err)
	}
	for _, v := range volumeList {
		if app, ok := v.GetLabels()["application"]; ok && app == types.AppName {
			if err = s.cli.VolumeRemove(v.Name); err != nil {
				if _, ok := err.(*volumes.VolumeDeleteLibpodNotFound); !ok {
					return fmt.Errorf("error removing volume %s - %w", v.Name, err)
				}
			}
		}
	}

	return nil
}

func (s *SitePodmanHandler) Update() error {
	return fmt.Errorf("not implemented")
}
