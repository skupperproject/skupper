package podman

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/non_kube/common"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/version"
	yaml "gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/util/rand"
)

type Site struct {
	*domain.SiteCommon
	IngressHosts                 []string
	IngressBindIPs               []string
	IngressBindInterRouterPort   int
	IngressBindEdgePort          int
	IngressBindFlowCollectorPort int
	ContainerNetwork             string
	EnableIPV6                   bool
	PodmanEndpoint               string
	EnableFlowCollector          bool
	EnableConsole                bool
	AuthMode                     string
	ConsoleUser                  string
	ConsolePassword              string
	RouterOpts                   types.RouterOptions
	PrometheusOpts               types.PrometheusServerOptions
	ControllerOpts               types.ControllerOptions
	FlowCollectorOpts            types.FlowCollectorOptions
}

func (s *Site) GetPlatform() string {
	return "podman"
}

func (s *Site) GetIngressClasses() []string {
	return []string{"host"}
}

func (s *Site) GetConsoleUrl() string {
	if s.EnableFlowCollector {
		ipAddr := "0.0.0.0"
		if len(s.IngressBindIPs) > 0 {
			ipAddr = utils.DefaultStr(s.IngressBindIPs[0], ipAddr)
		}
		port := s.IngressBindFlowCollectorPort
		return fmt.Sprintf("https://%s:%d", ipAddr, port)
	}
	return ""
}

func (s *Site) validateCreate() error {
	validationFunctions := []func() error{
		s.ValidateTuningOpts,
	}
	for _, fn := range validationFunctions {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Site) ValidateTuningOpts() error {
	var err error
	cpuLimits := map[string]string{
		"router":         s.RouterOpts.Tuning.CpuLimit,
		"controller":     s.ControllerOpts.Tuning.CpuLimit,
		"flow-collector": s.FlowCollectorOpts.Tuning.CpuLimit,
		"prometheus":     s.PrometheusOpts.Tuning.CpuLimit,
	}
	memoryLimits := map[string]string{
		"router":         s.RouterOpts.Tuning.MemoryLimit,
		"controller":     s.ControllerOpts.Tuning.MemoryLimit,
		"flow-collector": s.FlowCollectorOpts.Tuning.MemoryLimit,
		"prometheus":     s.PrometheusOpts.Tuning.MemoryLimit,
	}
	for component, cpuLimit := range cpuLimits {
		if cpuLimit != "" {
			if _, err = strconv.Atoi(cpuLimit); err != nil {
				return fmt.Errorf("invalid cpu limit (decimal) for %s: %s", component, cpuLimit)
			}
		}
	}
	for component, memoryLimit := range memoryLimits {
		if memoryLimit != "" {
			if _, err = strconv.ParseInt(memoryLimit, 10, 64); err != nil {
				return fmt.Errorf("invalid memory limit (bytes) for %s: %s", component, memoryLimit)
			}
		}
	}
	return nil
}

type SiteHandler struct {
	cli                  *podman.PodmanRestClient
	endpoint             string
	up                   *domain.UpdateProcessor
	networkStatusHandler *NetworkStatusHandler
}

func NewSitePodmanHandlerFromCli(cli *podman.PodmanRestClient) *SiteHandler {
	return &SiteHandler{
		cli:      cli,
		endpoint: cli.GetEndpoint(),
	}
}

type SiteHandlerFactory func(endpoint string) (domain.SiteHandler, error)

func NewSiteHandler(endpoint string) (domain.SiteHandler, error) {
	return NewSitePodmanHandler(endpoint)
}

func NewSitePodmanHandler(endpoint string) (*SiteHandler, error) {
	if endpoint == "" {
		podmanCfg, err := NewPodmanConfigFileHandler().GetConfig()
		if err != nil {
			return nil, fmt.Errorf("Unable to load local podman configuration - %w", err)
		}
		endpoint = podmanCfg.Endpoint
	}
	c, err := podman.NewPodmanClient(endpoint, "")
	if err != nil {
		return nil, err
	}
	return &SiteHandler{
		cli:      c,
		endpoint: endpoint,
	}, nil
}

func (s *SiteHandler) NetworkStatusHandler() *NetworkStatusHandler {
	if s.networkStatusHandler != nil {
		return s.networkStatusHandler
	}
	if s.cli == nil {
		return nil
	}
	s.networkStatusHandler = new(NetworkStatusHandler).WithClient(s.cli)
	return s.networkStatusHandler
}

func (s *SiteHandler) prepare(ctx context.Context, site domain.Site) (domain.Site, error) {
	podmanSite, ok := site.(*Site)

	if !ok {
		return nil, fmt.Errorf("not a valid podman site definition")
	}
	podmanSite.Platform = types.PlatformPodman
	if podmanSite.Mode == "" {
		podmanSite.Mode = "interior"
	}
	if podmanSite.ContainerNetwork == "" {
		podmanSite.ContainerNetwork = container.ContainerNetworkName
	}

	// Validating basic info
	if err := podmanSite.ValidateMinimumRequirements(); err != nil {
		return nil, err
	}
	// Validating mode (only interior is allowed at this point)
	if podmanSite.Mode == string(types.TransportModeEdge) {
		return nil, fmt.Errorf("edge mode is not yet allowed")
	}
	// Podman site specific validation
	if err := podmanSite.validateCreate(); err != nil {
		return nil, err
	}

	// Preparing site
	domain.ConfigureSiteCredentials(podmanSite, podmanSite.IngressHosts...)
	s.ConfigurePodmanDeployments(podmanSite)

	if err := s.canCreate(ctx, podmanSite); err != nil {
		return nil, err
	}

	return podmanSite, nil
}

func (s *SiteHandler) ConfigurePodmanDeployments(site *Site) {
	site.Deployments = append(site.Deployments, s.prepareRouterDeployment(site))
	site.Deployments = append(site.Deployments, s.prepareControllerDeployment(site))
	if site.EnableFlowCollector {
		site.Deployments = append(site.Deployments, s.prepareFlowCollectorDeployment(site))
		site.Deployments = append(site.Deployments, s.preparePrometheusDeployment(site))
	}
}

func (s *SiteHandler) prepareRouterDeployment(site *Site) *SkupperDeployment {
	// Router Deployment
	volumeMounts := map[string]string{
		types.LocalServerSecret:      "/etc/skupper-router-certs/skupper-amqps/",
		types.TransportConfigMapName: "/etc/skupper-router/config/",
		"skupper-router-certs":       "/etc/skupper-router-certs",
	}
	if !site.IsEdge() {
		volumeMounts[types.SiteServerSecret] = "/etc/skupper-router-certs/skupper-internal/"
	}
	memoryLimit, _ := strconv.ParseInt(site.RouterOpts.MemoryLimit, 10, 64)
	cpus, _ := strconv.Atoi(site.RouterOpts.CpuLimit)
	routerComponent := &domain.Router{
		// TODO ADD Labels
		Labels: map[string]string{},
		Env: map[string]string{
			"APPLICATION_NAME":    "skupper-router",
			"QDROUTERD_CONF":      "/etc/skupper-router/config/" + types.TransportConfigFile,
			"QDROUTERD_CONF_TYPE": "json",
			"SKUPPER_SITE_ID":     site.Id,
		},
		MemoryLimit: memoryLimit,
		Cpus:        cpus,
	}
	routerDepl := &SkupperDeployment{
		Name: types.TransportDeploymentName,
		SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{
			Components: []domain.SkupperComponent{
				routerComponent,
			},
		},
		Aliases:      []string{types.TransportServiceName, types.LocalTransportServiceName},
		VolumeMounts: volumeMounts,
		Networks:     []string{site.ContainerNetwork},
	}

	// If ingress mode is none, then ingress hosts will be empty
	if len(site.IngressHosts) > 0 {
		// Defining site ingresses
		ingressBindIps := site.IngressBindIPs
		if len(ingressBindIps) == 0 {
			ingressBindIps = append(ingressBindIps, "")
		}
		for _, ingressBindIp := range ingressBindIps {
			routerComponent.SiteIngresses = append(routerComponent.SiteIngresses, &SiteIngressHost{
				SiteIngressCommon: &domain.SiteIngressCommon{
					Name: types.InterRouterIngressPrefix,
					Host: ingressBindIp,
					Port: site.IngressBindInterRouterPort,
					Target: &domain.PortCommon{
						Name: types.InterRouterIngressPrefix,
						Port: int(types.InterRouterListenerPort),
					},
				},
			})
			routerComponent.SiteIngresses = append(routerComponent.SiteIngresses, &SiteIngressHost{
				SiteIngressCommon: &domain.SiteIngressCommon{
					Name: types.EdgeIngressPrefix,
					Host: ingressBindIp,
					Port: site.IngressBindEdgePort,
					Target: &domain.PortCommon{
						Name: types.EdgeIngressPrefix,
						Port: int(types.EdgeListenerPort),
					},
				},
			})
		}
	}

	return routerDepl
}

func (s *SiteHandler) Create(ctx context.Context, site domain.Site) error {
	var err error
	var cleanupFns []func()

	var preparedSite domain.Site
	podmanSite := site.(*Site)
	preparedSite, err = s.prepare(ctx, podmanSite)
	if err != nil {
		return err
	}
	podmanSite = preparedSite.(*Site)

	// cleanup on error
	defer func() {
		if err != nil {
			for i := len(cleanupFns) - 1; i >= 0; i-- {
				fn := cleanupFns[i]
				fn()
			}
		}
	}()

	// Save podman local configuration
	err = NewPodmanConfigFileHandler().Save(&Config{
		Endpoint: s.endpoint,
	})
	if err != nil {
		return err
	}

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
	podmanSite.RouterOpts.MaxFrameSize = types.RouterMaxFrameSizeDefault
	podmanSite.RouterOpts.MaxSessionFrames = types.RouterMaxSessionFramesDefault
	initialRouterConfig := qdr.InitialConfigSkupperRouter(podmanSite.GetName(), podmanSite.GetId(), version.Version, podmanSite.IsEdge(), 3, podmanSite.RouterOpts, "/etc/skupper-router-certs") //Is this the right path??
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
			vol, err = s.cli.VolumeCreate(&container.Volume{Name: volumeName})
			if err != nil {
				return err
			}
		}
		if OwnedBySkupper("volume", vol.Labels) == nil {
			cleanupFns = append(cleanupFns, func() {
				_ = s.cli.VolumeRemove(vol.Name)
			})
		}
	}

	// Create console user
	if err = s.createConsoleUser(podmanSite); err != nil {
		return err
	}

	// Create prometheus config
	if err = s.createPrometheusConfigFiles(podmanSite); err != nil {
		return err
	}

	// Deploy container(s)
	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	for _, depl := range podmanSite.GetDeployments() {
		err = deployHandler.Deploy(ctx, depl)
		if err != nil {
			return err
		}
		cleanupFns = append(cleanupFns, func() {
			_ = deployHandler.Undeploy(depl.GetName())
		})
	}

	// Creating startup scripts first
	// TODO REMOVE pkd/domain/podman once V2 is stable - IT NO LONGER WORKS
	scripts, _ := common.GetStartupScripts(&v1alpha1.Site{}, "")
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w\n", err)
	}

	// Creating systemd user service
	// TODO IT NO LONGER WORKS
	systemd, err := common.NewSystemdServiceInfo(&v1alpha1.Site{})
	if err = systemd.Create(); err != nil {
		fmt.Printf("Unable to create startup service - %v\n", err)
		fmt.Printf("The startup scripts: %s and %s are available at %s\n,",
			scripts.GetStartFileName(), scripts.GetStopFileName(), scripts.GetPath())
	}

	// Validate if lingering is enabled for current user
	if Username != "root" && !common.IsLingeringEnabled(Username) {
		fmt.Printf("It is recommended to enable lingering for %s, otherwise Skupper may not start on boot.\n", Username)
	}

	return nil
}

func (s *SiteHandler) canCreate(ctx context.Context, site *Site) error {

	// Validating podman endpoint
	if s.cli == nil {
		cli, err := podman.NewPodmanClient(site.PodmanEndpoint, "")
		if err != nil {
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
		containerInspect, err := cli.ContainerInspect(skupperDepl.GetName())
		if err == nil && containerInspect != nil {
			return fmt.Errorf("%s container already defined", skupperDepl.GetName())
		}
	}

	// Validating skupper networks available
	net, err := cli.NetworkInspect(site.ContainerNetwork)
	if err == nil && net != nil {
		if !net.DNS {
			return fmt.Errorf("network %s cannot be used as DNS is not enabled, fix the existing network or use a different one", site.ContainerNetwork)
		}
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
	if net == nil {
		createdNetwork, err := cli.NetworkCreate(&container.Network{
			Name:     site.ContainerNetwork,
			IPV6:     site.EnableIPV6,
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
	}

	// Validating existing volumes
	for _, v := range SkupperContainerVolumes {
		_, err := cli.VolumeInspect(v)
		if err == nil {
			return fmt.Errorf("required volume already exists %s", v)
		}
	}

	// Validating podman endpoint refers to local machine
	testVolumeName := "skupper-test-" + rand.String(5)
	v, err := cli.VolumeCreate(&container.Volume{Name: testVolumeName})
	if err != nil {
		return fmt.Errorf("unable to validate volume creation - %w", err)
	}
	defer cli.VolumeRemove(testVolumeName)
	if _, err = v.ListFiles(); err != nil {
		return fmt.Errorf("You cannot use a remote podman endpoint - %w", err)
	}

	// Validating router container runs without errors
	err = s.runTempContainer(ctx)
	if err != nil {
		return fmt.Errorf("site cannot be created: %s", err)
	}
	return nil
}

func (s *SiteHandler) createNetwork(site *Site) error {
	existingNet, err := s.cli.NetworkInspect(site.ContainerNetwork)
	if err == nil && existingNet != nil {
		return nil
	}
	_, err = s.cli.NetworkCreate(&container.Network{
		Name:     site.ContainerNetwork,
		IPV6:     site.EnableIPV6,
		DNS:      true,
		Internal: false,
	})
	if err != nil {
		return fmt.Errorf("error creating network %s - %v", site.ContainerNetwork, err)
	}
	return nil
}

func (s *SiteHandler) Get() (domain.Site, error) {
	site := &Site{
		SiteCommon:     &domain.SiteCommon{},
		PodmanEndpoint: s.endpoint,
	}

	// getting router config
	configHandler := NewRouterConfigHandlerPodman(s.cli)
	routerConfig, err := configHandler.GetRouterConfig()
	if err != nil {
		return nil, err
	}

	// Setting basic site info
	site.Name = routerConfig.Metadata.Id
	site.Mode = string(routerConfig.Metadata.Mode)
	site.Id = routerConfig.GetSiteMetadata().Id
	site.Version = routerConfig.GetSiteMetadata().Version
	site.Platform = types.PlatformPodman

	// Reading cert authorities
	credHandler := NewPodmanCredentialHandler(s.cli)
	cas, err := credHandler.ListCertAuthorities()
	if err != nil {
		return nil, fmt.Errorf("error reading certificate authorities - %w", err)
	}
	if len(cas) == 0 {
		return nil, fmt.Errorf("skupper certificate authorities not found")
	}
	site.CertAuthorities = cas

	// Reading credentials
	creds, err := credHandler.ListCredentials()
	if err != nil {
		return nil, fmt.Errorf("error reading credentials - %w", err)
	}
	if len(creds) == 0 {
		return nil, fmt.Errorf("skupper credentials not found")
	}
	site.Credentials = creds
	for _, cred := range creds {
		if cred.Name == types.SiteServerSecret {
			site.IngressHosts = cred.Hosts
		}
	}

	// Reading deployments
	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	deps, err := deployHandler.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving deployments - %w", err)
	}
	if len(deps) == 0 {
		return nil, fmt.Errorf("skupper containers not found")
	}
	site.Deployments = deps

	routerFound := false
	ctrlFound := false
	for _, dep := range site.GetDeployments() {
		for _, comp := range dep.GetComponents() {
			depPodman := dep.(*SkupperDeployment)
			site.ContainerNetwork = depPodman.Networks[0]
			for _, siteIng := range comp.GetSiteIngresses() {
				if siteIng.GetTarget().GetPort() == int(types.InterRouterListenerPort) {
					site.IngressBindIPs = append(site.IngressBindIPs, siteIng.GetHost())
					site.IngressBindInterRouterPort = siteIng.GetPort()
				} else if siteIng.GetTarget().GetPort() == int(types.EdgeListenerPort) {
					site.IngressBindEdgePort = siteIng.GetPort()
				} else if siteIng.GetTarget().GetPort() == int(types.FlowCollectorDefaultServicePort) {
					site.IngressBindFlowCollectorPort = siteIng.GetPort()
				}
			}
			switch c := comp.(type) {
			case *domain.Router:
				routerFound = true
				c.GetSiteIngresses()
				site.RouterOpts.Logging = qdr.GetRouterLogging(routerConfig)
				site.RouterOpts.MemoryLimit = strconv.FormatInt(c.MemoryLimit, 10)
				site.RouterOpts.CpuLimit = strconv.Itoa(c.Cpus)
			case *domain.FlowCollector:
				enableConsole, _ := strconv.ParseBool(c.Env["ENABLE_CONSOLE"])
				consoleUsers, _ := c.Env["FLOW_USERS"]
				if consoleUsers != "" {
					site.AuthMode = types.ConsoleAuthModeInternal
				} else {
					site.AuthMode = types.ConsoleAuthModeUnsecured
				}
				site.EnableConsole = enableConsole
				site.EnableFlowCollector = true
				site.FlowCollectorOpts.FlowRecordTtl, _ = time.ParseDuration(c.Env["FLOW_RECORD_TTL"])
				site.FlowCollectorOpts.MemoryLimit = strconv.FormatInt(c.MemoryLimit, 10)
				site.FlowCollectorOpts.CpuLimit = strconv.Itoa(c.Cpus)
				user, password, err := s.getConsoleUserPass()
				if err != nil {
					fmt.Println("error retrieving console user and password -", err)
				}
				site.ConsoleUser = user
				site.ConsolePassword = password
			case *domain.Controller:
				ctrlFound = true
				site.ControllerOpts.MemoryLimit = strconv.FormatInt(c.MemoryLimit, 10)
				site.ControllerOpts.CpuLimit = strconv.Itoa(c.Cpus)
			case *domain.Prometheus:
				site.PrometheusOpts, err = s.getPrometheusServerOptions()
				site.PrometheusOpts.MemoryLimit = strconv.FormatInt(c.MemoryLimit, 10)
				site.PrometheusOpts.CpuLimit = strconv.Itoa(c.Cpus)
				if err != nil {
					fmt.Println("error retrieving prometheus options -", err)
				}
			}
		}
	}
	if !routerFound {
		return nil, fmt.Errorf("%s component not found", types.TransportDeploymentName)
	}
	if !ctrlFound {
		return nil, fmt.Errorf("%s component not found", types.ControllerPodmanContainerName)
	}

	// Router options from router config
	site.RouterOpts.MaxFrameSize = routerConfig.Listeners["interior-listener"].MaxFrameSize
	site.RouterOpts.MaxSessionFrames = routerConfig.Listeners["interior-listener"].MaxSessionFrames

	return site, nil
}

func (s *SiteHandler) Delete() error {
	site, err := s.Get()
	if err != nil {
		// removing eventual resources from an incomplete initialization
		if s.AnyResourceLeft() {
			removeErr := s.removePodmanResources()
			if removeErr != nil {
				return fmt.Errorf("error cleaning up resources: %s - %s", removeErr, err)
			}
			return nil
		}
		return nil
	}
	podmanSite := site.(*Site)

	deployHandler := NewSkupperDeploymentHandlerPodman(s.cli)
	deploys, err := deployHandler.List()
	if err != nil {
		return fmt.Errorf("error retrieving deployments - %w", err)
	}

	// Stopping and removing containers
	for _, dep := range deploys {
		err = deployHandler.Undeploy(dep.GetName())
		if err != nil {
			return fmt.Errorf("error removing deployment %s - %w", dep.GetName(), err)
		}
	}

	// Removing any eventual container/volume left
	if err = s.removePodmanResources(); err != nil {
		return err
	}

	// Removing networks
	_ = s.cli.NetworkRemove(podmanSite.ContainerNetwork)

	return nil
}

func (s *SiteHandler) removePodmanResources() error {
	// removing containers owned by Skupper
	containers, err := s.cli.ContainerList()
	if err != nil {
		return fmt.Errorf("error listing containers - %w", err)
	}
	for _, c := range containers {
		if OwnedBySkupper("container", c.Labels) == nil {
			_ = s.cli.ContainerStop(c.Name)
			if err = s.cli.ContainerRemove(c.Name); err != nil {
				return err
			}
		}
	}

	// Removing volumes
	volumeList, err := s.cli.VolumeList()
	if err != nil {
		return fmt.Errorf("error retrieving volume list - %w", err)
	}
	for _, v := range volumeList {
		if app, ok := v.GetLabels()["application"]; ok && app == types.AppName {
			if err = s.cli.VolumeRemove(v.Name); err != nil {
				return err
			}
		}
	}

	// Removing startup files and service
	// TODO REMOVE IT - V1 podman wont work anymore
	scripts, _ := common.GetStartupScripts(&v1alpha1.Site{}, "")
	scripts.Remove()
	systemd, _ := common.NewSystemdServiceInfo(&v1alpha1.Site{})
	if err = systemd.Remove(); err != nil {
		fmt.Printf("Unable to remove systemd service - %v\n", err)
	}

	return nil
}

func (s *SiteHandler) AnyResourceLeft() bool {
	containers, _ := s.cli.ContainerList()
	for _, c := range containers {
		if OwnedBySkupper("container", c.Labels) == nil {
			return true
		}
	}
	volumeList, _ := s.cli.VolumeList()
	for _, v := range volumeList {
		if OwnedBySkupper("volume", v.GetLabels()) == nil {
			return true
		}
	}
	return false
}

func (s *SiteHandler) SetUpdateProcessor(up *domain.UpdateProcessor) {
	s.up = up
}

func (s *SiteHandler) Update() error {
	/*
	  Registering tasks to be analyzed against current site version
	*/

	// updates images for all skupper containers
	s.up.RegisterTasks(NewContainerImagesTask(s.cli))
	// updates site version number (always the last one)
	s.up.RegisterTasks(NewVersionUpdateTask(s.cli))
	// 1.5.4 tasks
	s.up.RegisterTasks(new(SkupperNetworkStatusVolume).WithCli(s.cli))
	// process eligible tasks
	ctx, cn := context.WithTimeout(context.Background(), s.up.Timeout)
	defer cn()

	// current version
	site, err := s.Get()
	if err != nil {
		return err
	}

	return s.up.Process(ctx, site.GetVersion())
}

func (s *SiteHandler) RevokeAccess() error {
	site, err := s.Get()
	if err != nil {
		return err
	}
	podmanSite := site.(*Site)

	credHandler := NewPodmanCredentialHandler(s.cli)
	// Regenerating Site CA
	_, err = credHandler.NewCertAuthority(types.CertAuthority{Name: types.SiteCaSecret})
	if err != nil {
		return fmt.Errorf("error creating site CA - %w", err)
	}

	// Regenerating Site Server
	_, err = credHandler.NewCredential(types.Credential{
		CA:          types.SiteCaSecret,
		Name:        types.SiteServerSecret,
		Subject:     types.TransportServiceName,
		Hosts:       podmanSite.IngressHosts,
		ConnectJson: false,
	})
	if err != nil {
		return fmt.Errorf("error creating site credential - %w", err)
	}

	// Restarting router
	err = s.cli.ContainerRestart(types.TransportDeploymentName)
	if err != nil {
		return fmt.Errorf("error starting %s - %w", types.TransportDeploymentName, err)
	}
	return nil
}

func (s *SiteHandler) prepareFlowCollectorDeployment(site *Site) *SkupperDeployment {
	// Flow Collector Deployment
	volumeMounts := map[string]string{
		types.LocalClientSecret:   "/etc/messaging",
		types.ConsoleUsersSecret:  "/etc/console-users",
		types.ConsoleServerSecret: "/etc/service-controller/console",
	}
	endpoint := site.PodmanEndpoint
	if s.cli.IsSockEndpoint() {
		sockFile := strings.TrimPrefix(s.cli.GetEndpoint(), "unix://")
		endpoint = "/tmp/podman.sock"
		volumeMounts[sockFile] = endpoint
	}
	memoryLimit, _ := strconv.ParseInt(site.FlowCollectorOpts.MemoryLimit, 10, 64)
	cpus, _ := strconv.Atoi(site.FlowCollectorOpts.CpuLimit)
	flowComponent := &domain.FlowCollector{
		// TODO ADD Labels
		Labels: map[string]string{},
		Env: map[string]string{
			"ENABLE_CONSOLE":   fmt.Sprintf("%v", site.EnableConsole),
			"FLOW_RECORD_TTL":  site.FlowCollectorOpts.FlowRecordTtl.String(),
			"SKUPPER_PLATFORM": types.PlatformPodman,
			"PODMAN_ENDPOINT":  endpoint,
		},
		MemoryLimit: memoryLimit,
		Cpus:        cpus,
	}
	if site.AuthMode != types.ConsoleAuthModeUnsecured {
		flowComponent.Env["FLOW_USERS"] = "/etc/console-users"
		site.AuthMode = types.ConsoleAuthModeInternal
	}
	flowDeployment := &SkupperDeployment{
		Name: types.FlowCollectorContainerName,
		SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{
			Components: []domain.SkupperComponent{
				flowComponent,
			},
		},
		Aliases:        []string{types.FlowCollectorContainerName},
		VolumeMounts:   volumeMounts,
		Networks:       []string{site.ContainerNetwork},
		SELinuxDisable: true,
	}

	// Defining site ingresses
	ingressBindIps := site.IngressBindIPs
	if len(ingressBindIps) == 0 {
		ingressBindIps = append(ingressBindIps, "")
	}
	for _, ingressBindIp := range ingressBindIps {
		flowComponent.SiteIngresses = append(flowComponent.SiteIngresses, &SiteIngressHost{
			SiteIngressCommon: &domain.SiteIngressCommon{
				Name: types.FlowCollectorContainerName,
				Host: ingressBindIp,
				Port: site.IngressBindFlowCollectorPort,
				Target: &domain.PortCommon{
					Name: types.FlowCollectorContainerName,
					Port: int(types.FlowCollectorDefaultServiceTargetPort),
				},
			},
		})
	}

	return flowDeployment
}

func (s *SiteHandler) createConsoleUser(site *Site) error {
	v, err := s.cli.VolumeInspect(types.ConsoleUsersSecret)
	if err != nil {
		return err
	}
	user := utils.DefaultStr(site.ConsoleUser, "admin")
	password := utils.DefaultStr(site.ConsolePassword, utils.RandomId(10))
	_, err = v.CreateFiles(map[string]string{user: password}, false)
	if err != nil {
		return fmt.Errorf("error creating console user - %w", err)
	}
	return nil
}

func (s *SiteHandler) getConsoleUserPass() (string, string, error) {
	v, err := s.cli.VolumeInspect(types.ConsoleUsersSecret)
	if err != nil {
		return "", "", err
	}

	var files []os.DirEntry

	if !s.cli.IsRunningInContainer() {
		files, err = v.ListFiles()
	} else {
		files, err = os.ReadDir("/etc/console-users")
	}
	if err != nil {
		return "", "", err
	}
	if len(files) == 0 {
		return "", "", fmt.Errorf("console user is not defined")
	}
	f := files[0]
	user := f.Name()
	var pass string
	if !s.cli.IsRunningInContainer() {
		pass, err = v.ReadFile(user)
	} else {
		var passData []byte
		passData, err = os.ReadFile(path.Join("/etc/console-users", user))
		pass = string(passData)
	}
	if err != nil {
		return user, "", fmt.Errorf("error reading console password: %w", err)
	}
	return user, pass, nil
}

func (s *SiteHandler) prepareControllerDeployment(site *Site) *SkupperDeployment {
	// Service Controller Deployment
	volumeMounts := map[string]string{
		types.ServiceInterfaceConfigMap:  "/etc/skupper-services",
		types.NetworkStatusConfigMapName: "/etc/skupper-network-status",
		types.LocalClientSecret:          "/etc/messaging",
		types.ConsoleUsersSecret:         "/etc/console-users",
		types.ConsoleServerSecret:        "/etc/service-controller/console",
		types.LocalServerSecret:          "/etc/skupper-router-certs/skupper-amqps/",
		types.TransportConfigMapName:     "/etc/skupper-router/config/",
		"skupper-router-certs":           "/etc/skupper-router-certs",
		types.SiteServerSecret:           "/etc/skupper-router-certs/skupper-internal/",
	}

	endpoint := site.PodmanEndpoint
	if s.cli.IsSockEndpoint() {
		sockFile := strings.TrimPrefix(s.cli.GetEndpoint(), "unix://")
		endpoint = "/tmp/podman.sock"
		volumeMounts[sockFile] = endpoint
	}
	memoryLimit, _ := strconv.ParseInt(site.ControllerOpts.MemoryLimit, 10, 64)
	cpus, _ := strconv.Atoi(site.ControllerOpts.CpuLimit)
	ctrlComponent := &domain.Controller{
		// TODO ADD Labels
		Labels: map[string]string{},
		Env: map[string]string{
			"SKUPPER_SITE_NAME":   site.GetName(),
			"SKUPPER_SITE_ID":     site.GetId(),
			"SKUPPER_ROUTER_MODE": site.GetMode(),
			"SKUPPER_PLATFORM":    types.PlatformPodman,
			"PODMAN_ENDPOINT":     endpoint,
		},
		MemoryLimit: memoryLimit,
		Cpus:        cpus,
	}
	if site.AuthMode != types.ConsoleAuthModeUnsecured {
		ctrlComponent.Env["FLOW_USERS"] = "/etc/console-users"
		ctrlComponent.Env["METRICS_USERS"] = "/etc/console-users"
		site.AuthMode = types.ConsoleAuthModeInternal
	}
	ctrlDeployment := &SkupperDeployment{
		Name: types.ControllerPodmanContainerName,
		SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{
			Components: []domain.SkupperComponent{
				ctrlComponent,
			},
		},
		Aliases:        []string{types.ControllerServiceName, types.ControllerPodmanContainerName},
		VolumeMounts:   volumeMounts,
		Networks:       []string{site.ContainerNetwork},
		SELinuxDisable: true,
	}

	// Defining site ingresses
	// TODO add along with claims and rest support

	return ctrlDeployment
}

func (s *SiteHandler) preparePrometheusDeployment(site *Site) domain.SkupperDeployment {
	// Prometheus Server Deployment
	volumeMounts := map[string]string{
		"prometheus-server-config":  "/etc/prometheus",
		"prometheus-storage-volume": "/prometheus",
	}
	memoryLimit, _ := strconv.ParseInt(site.PrometheusOpts.MemoryLimit, 10, 64)
	cpus, _ := strconv.Atoi(site.PrometheusOpts.CpuLimit)
	prometheusComponent := &domain.Prometheus{
		// TODO ADD Labels
		Labels:      map[string]string{},
		MemoryLimit: memoryLimit,
		Cpus:        cpus,
	}
	prometheusDeployment := &SkupperDeployment{
		Name: types.PrometheusDeploymentName,
		SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{
			Components: []domain.SkupperComponent{
				prometheusComponent,
			},
		},
		Aliases:        []string{types.PrometheusDeploymentName},
		VolumeMounts:   volumeMounts,
		Networks:       []string{site.ContainerNetwork},
		SELinuxDisable: true,
	}
	return prometheusDeployment
}

func (s *SiteHandler) getPrometheusServerOptions() (types.PrometheusServerOptions, error) {
	var prometheusConfig types.PrometheusServerOptions
	v, err := s.cli.VolumeInspect("prometheus-server-config")
	if err != nil {
		return prometheusConfig, err
	}
	prometheusConfigStr, err := v.ReadFile("skupper-prometheus.yml")
	if err != nil {
		return prometheusConfig, fmt.Errorf("error reading skupper-prometheus.yml - %s", err)
	}
	err = yaml.Unmarshal([]byte(prometheusConfigStr), &prometheusConfig)
	if err != nil {
		return prometheusConfig, fmt.Errorf("error parsing prometheus options - %s", err)
	}
	return prometheusConfig, nil
}

func (s *SiteHandler) savePrometheusServerConfigFile(name string, data interface{}) error {
	v, err := s.cli.VolumeInspect("prometheus-server-config")
	if err != nil {
		return err
	}
	var dataStr string
	var ok bool
	if dataStr, ok = data.(string); !ok {
		var yamlData []byte
		yamlData, err = yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("error serializing prometheus options - %s", err)
		}
		dataStr = string(yamlData)
	}
	_, err = v.CreateFile(name, []byte(dataStr), true)
	return err
}

func (s *SiteHandler) createPrometheusConfigFiles(site *Site) error {
	promInfo := config.PrometheusInfo{
		BasicAuth:   false,
		TlsAuth:     false,
		ServiceName: types.FlowCollectorContainerName,
		Port:        strconv.Itoa(int(types.FlowCollectorDefaultServicePort)),
		User:        utils.DefaultStr(site.PrometheusOpts.User, "admin"),
		Password:    utils.DefaultStr(site.PrometheusOpts.Password, "admin"),
		Hash:        "",
	}

	if site.PrometheusOpts.AuthMode == string(types.PrometheusAuthModeBasic) {
		promInfo.BasicAuth = true
		promInfo.User = site.PrometheusOpts.User
		promInfo.Password = site.PrometheusOpts.Password
		hash, _ := config.HashPrometheusPassword(promInfo.Password)
		promInfo.Hash = string(hash)
	} else if site.PrometheusOpts.AuthMode == string(types.PrometheusAuthModeTls) {
		promInfo.TlsAuth = true
	}

	prometheusConfigFiles := map[string]interface{}{
		"skupper-prometheus.yml": site.PrometheusOpts,
		"prometheus.yml":         config.ScrapeConfigForPrometheus(promInfo),
		"web-config.yml":         config.ScrapeWebConfigForPrometheus(promInfo),
	}

	var err error
	for name, data := range prometheusConfigFiles {
		err = s.savePrometheusServerConfigFile(name, data)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SiteHandler) runTempContainer(ctx context.Context) error {
	cli := s.cli
	err := cli.ImagePull(ctx, images.GetRouterImageName())
	if err != nil {
		return err
	}
	tempName := fmt.Sprintf("skupper-temp-%s", uuid.NewString()[:5])
	err = cli.ContainerCreate(
		&container.Container{
			Name:          tempName,
			Image:         images.GetRouterImageName(),
			RestartPolicy: "no",
			Command:       []string{"skrouterd", "-version"},
		},
	)
	if err != nil {
		return fmt.Errorf("unable to validate container creation: %s", err)
	}
	defer func() {
		_ = cli.ContainerRemove(tempName)
	}()
	if err = cli.ContainerStart(tempName); err != nil {
		return fmt.Errorf("error starting validation container: %s", err)
	}
	ci, err := cli.ContainerInspect(tempName)
	if err != nil {
		return fmt.Errorf("error inspecting validation container: %s", err)
	}
	if ci.ExitCode != 0 {
		logs, err := cli.ContainerLogs(tempName)
		if err != nil {
			return fmt.Errorf("error executing validation container (unable to read logs) - exit code: %d", ci.ExitCode)
		}
		return fmt.Errorf("unable to run %s - reason: %s", ci.Image, logs)
	}
	return nil
}
