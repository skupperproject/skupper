package podman

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/rogpeppe/go-internal/lockedfile"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

type serviceAction int

const (
	ServiceVolumePrefix                   = "skupper-config-"
	SkupperServicesLockfile               = "skupper-services.json.lock"
	SkupperServicesFilename               = "skupper-services.json"
	serviceCreate           serviceAction = iota
	serviceUpdate
	serviceDelete
)

type ServicePodman struct {
	*domain.ServiceCommon
	ContainerName string
}

func (s *ServicePodman) GetContainerName() string {
	return utils.DefaultStr(s.ContainerName, s.GetAddress())
}

func (s *ServicePodman) AsServiceInterface() *types.ServiceInterface {
	svc := &types.ServiceInterface{
		Address:        s.Address,
		Protocol:       s.Protocol,
		Ports:          s.Ports,
		EventChannel:   s.EventChannel,
		Aggregate:      s.Aggregate,
		Labels:         s.Labels,
		Targets:        []types.ServiceInterfaceTarget{},
		Origin:         s.Origin,
		EnableTls:      s.Tls,
		TlsCredentials: s.TlsCredentials,
	}

	for _, egressResolver := range s.EgressResolvers {
		egresses, _ := egressResolver.Resolve()
		for _, egress := range egresses {
			svc.Targets = append(svc.Targets, types.ServiceInterfaceTarget{
				Name:        egressResolver.String(),
				TargetPorts: egress.GetPorts(),
				Service:     egress.GetHost(),
			})
		}
	}

	return svc
}

func (s *ServicePodman) ContainerPorts() []container.Port {
	ports := []container.Port{}
	for port, hostPort := range s.GetIngress().GetPorts() {
		if hostPort == 0 {
			continue
		}
		ports = append(ports, container.Port{
			Host:   strconv.Itoa(hostPort),
			HostIP: s.GetIngress().GetHost(),
			Target: strconv.Itoa(port),
		})
	}
	return ports
}

type ServiceHandlerPodman struct {
	cli     *podman.PodmanRestClient
	handler *ServiceInterfaceHandlerPodman
}

func NewServiceHandlerPodman(cli *podman.PodmanRestClient) *ServiceHandlerPodman {
	return &ServiceHandlerPodman{
		cli:     cli,
		handler: NewServiceInterfaceHandlerPodman(cli),
	}
}

func (s *ServiceHandlerPodman) Create(service domain.Service) error {
	// Validate service interface definition exists
	servicePodman := service.(*ServicePodman)
	if err := s.validateNewService(servicePodman); err != nil {
		return err
	}
	return s.createService(servicePodman)
}

func (s *ServiceHandlerPodman) validateNewService(servicePodman *ServicePodman) error {
	// Validating if service exists
	_, err := s.handler.Get(servicePodman.GetAddress())
	if err == nil {
		return fmt.Errorf("Service %s already defined", servicePodman.GetAddress())
	}

	// Validating service definition
	if err := domain.ValidateService(servicePodman); err != nil {
		return err
	}

	// Validate if target container name already exists
	svcContainer, err := s.cli.ContainerInspect(servicePodman.GetContainerName())
	if err == nil && svcContainer != nil {
		return fmt.Errorf("a container named %s already exists", servicePodman.GetContainerName())
	}

	// Validating if ingress ports are available
	if servicePodman.Ingress != nil && servicePodman.Ingress.GetPorts() != nil && len(servicePodman.Ingress.GetPorts()) > 0 {
		for port, hostPort := range servicePodman.Ingress.GetPorts() {
			if utils.TcpPortInUse(servicePodman.Ingress.GetHost(), hostPort) {
				return fmt.Errorf("ingress port %d is already in use", hostPort)
			}
			if !utils.IntSliceContains(servicePodman.Ports, port) {
				return fmt.Errorf("service does not specify mapped port %d", port)
			}
		}
	}

	if servicePodman.Ingress == nil {
		servicePodman.Ingress = &domain.AddressIngressCommon{
			Ports: map[int]int{},
		}
		for _, port := range servicePodman.Ports {
			servicePodman.Ingress.GetPorts()[port] = 0
		}
	}

	return nil
}

func (s *ServiceHandlerPodman) createService(servicePodman *ServicePodman) error {
	// rollback in case of error
	var cleanupFns []func()
	var err error
	defer func() {
		if err == nil {
			return
		}
		for _, fn := range cleanupFns {
			fn()
		}
	}()

	// Site handler instance
	siteHandler, err := NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("error verifying site - %w", err)
	}
	site, err := siteHandler.Get()
	if err != nil {
		return fmt.Errorf("error retrieving site info - %w", err)
	}

	// Router config file handler
	routerConfigHandler := NewRouterConfigHandlerPodman(s.cli)
	routerConfig, err := routerConfigHandler.GetRouterConfig()
	if err != nil {
		return fmt.Errorf("error retrieving router config - %w", err)
	}

	// Cert handler
	credHandler := NewPodmanCredentialHandler(s.cli)

	//
	// Starting creation process
	//

	// Create skupper-services entry
	if err = s.handler.Create(servicePodman.AsServiceInterface()); err != nil {
		return err
	}
	cleanupFns = append(cleanupFns, func() {
		_ = s.handler.Delete(servicePodman.GetAddress())
	})

	// Creating the router config
	var svcRouterConfig *qdr.RouterConfig
	var svcRouterConfigStr string
	var configVolume *container.Volume
	svcRouterConfig, svcRouterConfigStr, err = domain.CreateRouterServiceConfig(site, routerConfig, servicePodman)

	// Creating volume to store config for service router
	volumeName := ServiceVolumePrefix + servicePodman.Address
	if configVolume, err = s.cli.VolumeCreate(&container.Volume{Name: volumeName}); err != nil {
		return fmt.Errorf("error creating volume to store router configuration file - %w", err)
	}
	cleanupFns = append(cleanupFns, func() {
		_ = s.cli.VolumeRemove(configVolume.Name)
	})
	if _, err = configVolume.CreateFile(types.TransportConfigFile, []byte(svcRouterConfigStr), false); err != nil {
		return fmt.Errorf("error saving router configuration file - %w", err)
	}
	configVolume.Destination = "/etc/skupper-router/config/"

	// Create TLS credentials (optional)
	if servicePodman.IsTls() {
		v, err := s.cli.VolumeInspect(SharedTlsCertificates)
		if err != nil {
			return err
		}

		ca, err := credHandler.GetSecret(types.ServiceCaSecret)
		if err != nil {
			return fmt.Errorf("unable to find ca %s - %w", types.ServiceCaSecret, err)
		}

		tlsCredentialName := types.SkupperServiceCertPrefix + servicePodman.GetAddress()
		svcSecret := certs.GenerateSecret(tlsCredentialName, servicePodman.GetAddress(), servicePodman.GetAddress(), ca)
		err = v.CreateDirectory(tlsCredentialName)
		if err != nil {
			return fmt.Errorf("error creating directory for service secret %s - %w", tlsCredentialName, err)
		}
		cleanupFns = append(cleanupFns, func() {
			_ = v.DeleteFile(tlsCredentialName, true)
		})

		for filename, data := range svcSecret.Data {
			_, err = v.CreateFile(path.Join(tlsCredentialName, filename), data, true)
			if err != nil {
				return fmt.Errorf("error creating credential file %s/%s - %w", tlsCredentialName, filename, err)
			}
		}
	}

	// Create service container (edge)
	routerContainer, err := s.cli.ContainerInspect(types.TransportDeploymentName)
	if err != nil {
		return fmt.Errorf("error retrieving %s container - %w", types.TransportDeploymentName, err)
	}
	mounts := []container.Volume{}
	for _, v := range routerContainer.Mounts {
		if v.Name != types.TransportConfigMapName {
			mounts = append(mounts, v)
		}
	}
	mounts = append(mounts, *configVolume)
	site.GetDeployments()[0].GetComponents()[0].GetImage()
	c := &container.Container{
		Name:  servicePodman.GetContainerName(),
		Image: utils.DefaultStr(routerContainer.Image, types.GetRouterImageName()),
		Env: map[string]string{
			"APPLICATION_NAME":    svcRouterConfig.GetSiteMetadata().Id,
			"QDROUTERD_CONF":      "/etc/skupper-router/config/" + types.TransportConfigFile,
			"QDROUTERD_CONF_TYPE": "json",
			"SKUPPER_SITE_ID":     svcRouterConfig.GetSiteMetadata().Id,
			"QDROUTERD_DEBUG":     routerContainer.Env["QDROUTERD_DEBUG"],
		},
		Labels: map[string]string{
			types.AddressQualifier: servicePodman.GetAddress(),
		},
		Mounts:        mounts,
		Networks:      map[string]container.ContainerNetworkInfo{},
		Ports:         servicePodman.ContainerPorts(),
		RestartPolicy: "always",
	}
	for netName, _ := range routerContainer.Networks {
		c.Networks[netName] = container.ContainerNetworkInfo{
			ID: netName,
		}
	}
	for l, v := range servicePodman.Labels {
		c.Labels[l] = v
	}
	err = s.cli.ContainerCreate(c)
	if err != nil {
		return fmt.Errorf("error creating container %s - %w", c.Name, err)
	}
	cleanupFns = append(cleanupFns, func() {
		_ = s.cli.ContainerRemove(c.Name)
	})

	// Start container
	err = s.cli.ContainerStart(c.Name)
	if err != nil {
		return fmt.Errorf("error starting container %s - %w", c.Name, err)
	}
	return nil
}

func (s *ServiceHandlerPodman) Delete(address string) error {
	// Check service exists
	svc, err := s.Get(address)
	if err != nil {
		return err
	}
	svcPodman := svc.(*ServicePodman)

	// Stop container
	_ = s.cli.ContainerStop(svcPodman.GetContainerName())

	// Remove container
	_ = s.cli.ContainerRemove(svcPodman.GetContainerName())

	// Removing config volume
	_ = s.cli.VolumeRemove(ServiceVolumePrefix + address)

	// If tls credentials were defined, remove the respective directory
	if svcPodman.IsTls() {
		v, err := s.cli.VolumeInspect(SharedTlsCertificates)
		if err != nil {
			return err
		}
		tlsCredentialName := types.SkupperServiceCertPrefix + svcPodman.GetAddress()
		_ = v.DeleteFile(tlsCredentialName, true)
	}

	// Remove skupper-services entry
	_ = s.handler.Delete(address)

	return nil
}

func (s *ServiceHandlerPodman) Get(address string) (domain.Service, error) {
	svcs, err := s.handler.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving service list - %w", err)
	}
	if err != nil {
		return nil, err
	}
	for _, svc := range svcs {
		if svc.Address == address {
			return s.handler.ToServicePodman(svc)
		}
	}
	return nil, fmt.Errorf("Service %s not defined", address)
}

func (s *ServiceHandlerPodman) List() ([]domain.Service, error) {
	var services []domain.Service
	list, err := s.handler.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving service list - %w", err)
	}
	for _, svc := range list {
		// for local services, retrieve respective containers
		svcPodman, err := s.handler.ToServicePodman(svc)
		if err != nil {
			return nil, err
		}
		services = append(services, svcPodman)
	}
	return services, nil
}

func (s *ServiceHandlerPodman) GetServiceRouterConfig(address string) (*qdr.RouterConfig, error) {
	var err error
	volumeName := ServiceVolumePrefix + address

	var vol *container.Volume
	vol, err = s.cli.VolumeInspect(volumeName)
	if err != nil {
		return nil, fmt.Errorf("error retrieving config volume - %w", err)
	}
	var configStr string
	if configStr, err = vol.ReadFile(types.TransportConfigFile); err != nil {
		return nil, fmt.Errorf("error reading config file - %w", err)
	}
	var config qdr.RouterConfig
	config, err = qdr.UnmarshalRouterConfig(configStr)
	return &config, err
}

func (s *ServiceHandlerPodman) SaveServiceRouterConfig(address string, config *qdr.RouterConfig) error {
	var err error
	volumeName := ServiceVolumePrefix + address

	var vol *container.Volume
	vol, err = s.cli.VolumeInspect(volumeName)
	if err != nil {
		return fmt.Errorf("error retrieving config volume - %w", err)
	}
	var configStr string
	configStr, err = qdr.MarshalRouterConfig(*config)
	_, err = vol.CreateFile(types.TransportConfigFile, []byte(configStr), true)

	return err
}

func (s *ServiceHandlerPodman) AddEgressResolver(address string, egressResolver domain.EgressResolver) error {
	siteHandler, err := NewSitePodmanHandler("")
	if err != nil {
		return fmt.Errorf("error preparing site handler - %w", err)
	}
	site, err := siteHandler.Get()
	if err != nil {
		return fmt.Errorf("error retrieving site info - %w", err)
	}
	egresses, err := egressResolver.Resolve()
	if err != nil {
		return fmt.Errorf("error resolving egresses - %w", err)
	}

	// Retrieve service definition
	svc, err := s.Get(address)
	if err != nil {
		return err
	}
	svcPodman := svc.(*ServicePodman)
	routerEntityMgr := NewRouterEntityManagerPodmanFor(s.cli, svcPodman.ContainerName)

	// Verify egress resolver is not already defined
	egressResolverStr := egressResolver.String()
	for _, resolver := range svc.GetEgressResolvers() {
		if resolver.String() == egressResolverStr {
			return fmt.Errorf("egress resolver already defined")
		}
	}

	// Retrieve router configuration for service
	config, err := s.GetServiceRouterConfig(address)
	if err != nil {
		return fmt.Errorf("error retrieving service config - %w", err)
	}

	// Add the egress to the service definition
	svc.AddEgressResolver(egressResolver)

	// Update skupper-services
	if err = s.handler.Update(svcPodman.AsServiceInterface()); err != nil {
		return fmt.Errorf("error updating service definition - %w", err)
	}

	// Add egresses to the router config
	if err = domain.ServiceRouterConfigAddTargets(site, config, svcPodman, egressResolver); err != nil {
		return fmt.Errorf("error adding targets to router config - %w", err)
	}

	// Update router config file
	if err = s.SaveServiceRouterConfig(address, config); err != nil {
		return fmt.Errorf("error updating router config for service %s - %w", address, err)
	}

	// Update router entities
	for _, egress := range egresses {
		connectorNames := domain.RouterConnectorNamesForEgress(address, egress)
		for port, _ := range egress.GetPorts() {
			connectorName := connectorNames[port]
			switch svcPodman.GetProtocol() {
			case "tcp":
				tcpConnector := config.Bridges.TcpConnectors[connectorName]
				err = routerEntityMgr.CreateTcpConnector(tcpConnector)
			case "http":
				fallthrough
			case "http2":
				httpConnector := config.Bridges.HttpConnectors[connectorName]
				err = routerEntityMgr.CreateHttpConnector(httpConnector)
			}
			if err != nil {
				return fmt.Errorf("error creating %s connector - %w", svcPodman.GetProtocol(), err)
			}
		}
	}
	return nil
}

func (s *ServiceHandlerPodman) RemoveEgressResolver(address string, egressResolver domain.EgressResolver) error {
	var egresses []domain.AddressEgress
	var err error

	// if nil, remove all
	if egressResolver != nil {
		egresses, err = egressResolver.Resolve()
		if err != nil {
			return fmt.Errorf("error resolving egresses - %w", err)
		}
	}

	// Retrieve service definition
	svc, err := s.Get(address)
	if err != nil {
		return err
	}
	svcPodman := svc.(*ServicePodman)
	origEgressResolvers := svcPodman.GetEgressResolvers()
	routerEntityMgr := NewRouterEntityManagerPodmanFor(s.cli, svcPodman.ContainerName)

	// Verify egress resolver is defined
	updatedResolvers := []domain.EgressResolver{}
	if egressResolver != nil {
		egressResolverStr := egressResolver.String()
		found := false
		for _, resolver := range svc.GetEgressResolvers() {
			if resolver.String() == egressResolverStr {
				found = true
			} else {
				updatedResolvers = append(updatedResolvers, resolver)
			}
		}
		if !found {
			return fmt.Errorf("egress resolver not defined")
		}
	}

	// Retrieve router configuration for service
	config, err := s.GetServiceRouterConfig(address)
	if err != nil {
		return fmt.Errorf("error retrieving service config - %w", err)
	}

	// Remove the egress from the service definition
	svc.SetEgressResolvers(updatedResolvers)

	// Update skupper-services
	if err = s.handler.Update(svcPodman.AsServiceInterface()); err != nil {
		return fmt.Errorf("error updating service definition - %w", err)
	}

	// Remove egresses from the router config
	if err = domain.ServiceRouterConfigRemoveTargets(config, svcPodman, egressResolver); err != nil {
		return fmt.Errorf("error removing targets to router config - %w", err)
	}

	// Update router config file
	if err = s.SaveServiceRouterConfig(address, config); err != nil {
		return fmt.Errorf("error updating router config for service %s - %w", address, err)
	}

	// Update router entities
	if len(egresses) == 0 && len(origEgressResolvers) > 0 {
		for _, resolver := range origEgressResolvers {
			resolved, err := resolver.Resolve()
			if err != nil {
				return fmt.Errorf("error resolving egresses - %w", err)
			}
			egresses = append(egresses, resolved...)
		}
	}
	for _, egress := range egresses {
		connectorNames := domain.RouterConnectorNamesForEgress(address, egress)
		for port, _ := range egress.GetPorts() {
			connectorName := connectorNames[port]
			switch svcPodman.GetProtocol() {
			case "tcp":
				err = routerEntityMgr.DeleteTcpConnector(connectorName)
			case "http":
				fallthrough
			case "http2":
				err = routerEntityMgr.DeleteHttpConnector(connectorName)
			}
			if err != nil {
				return fmt.Errorf("error deleting %s connector - %w", svcPodman.GetProtocol(), err)
			}
		}
	}
	return nil
}

func (s *ServiceHandlerPodman) RemoveAllEgressResolvers(address string) error {
	return s.RemoveEgressResolver(address, nil)
}

type ServiceInterfaceHandlerPodman struct {
	cli *podman.PodmanRestClient
}

func NewServiceInterfaceHandlerPodman(cli *podman.PodmanRestClient) *ServiceInterfaceHandlerPodman {
	return &ServiceInterfaceHandlerPodman{
		cli: cli,
	}
}

func (s *ServiceInterfaceHandlerPodman) manipulateService(service *types.ServiceInterface, action serviceAction) error {
	// Saving content
	vol, err := s.cli.VolumeInspect(types.ServiceInterfaceConfigMap)
	if err != nil {
		return fmt.Errorf("error reading volume %s - %w", types.ServiceInterfaceConfigMap, err)
	}
	lockfile := path.Join(vol.Source, SkupperServicesLockfile)
	unlockFn, err := lockedfile.MutexAt(lockfile).Lock()
	if err != nil {
		return fmt.Errorf("unable to lock %s - %w", lockfile, err)
	}
	defer unlockFn()

	data, err := s.List()
	if err != nil {
		return fmt.Errorf("error retrieving existing services - %w", err)
	}
	if _, ok := data[service.Address]; action == serviceCreate && ok {
		return fmt.Errorf("service %s already defined", service.Address)
	} else if action != serviceCreate && !ok {
		return fmt.Errorf("service %s does not exit", service.Address)
	}
	if action != serviceDelete {
		data[service.Address] = service
	} else {
		delete(data, service.Address)
	}

	content, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("error serializing %s - %w", types.ServiceInterfaceConfigMap, err)
	}
	_, err = vol.CreateFile(SkupperServicesFilename, content, true)
	if err != nil {
		return fmt.Errorf("error writing to %s - %w", SkupperServicesFilename, err)
	}

	return nil
}

func (s *ServiceInterfaceHandlerPodman) Create(service *types.ServiceInterface) error {
	return s.manipulateService(service, serviceCreate)
}

func (s *ServiceInterfaceHandlerPodman) List() (map[string]*types.ServiceInterface, error) {
	res := map[string]*types.ServiceInterface{}
	servicesVolume, err := s.cli.VolumeInspect(types.ServiceInterfaceConfigMap)
	if err != nil {
		return res, fmt.Errorf("cannot read volume %s - %w", types.ServiceInterfaceConfigMap, err)
	}
	data, err := servicesVolume.ReadFile(SkupperServicesFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return res, nil
		}
		return res, fmt.Errorf("error reading skupper services - %w", err)
	}
	if len(data) == 0 {
		return res, nil
	}
	err = json.Unmarshal([]byte(data), &res)
	if err != nil {
		return nil, fmt.Errorf("error decoding %s - %w", SkupperServicesFilename, err)
	}
	return res, nil
}

func (s *ServiceInterfaceHandlerPodman) Get(address string) (*types.ServiceInterface, error) {
	svcMap, err := s.List()
	if err != nil {
		return nil, err
	}
	notDefined := fmt.Errorf("Service %s not defined", address)
	if svcMap == nil {
		return nil, notDefined
	}
	svc, ok := svcMap[address]
	if !ok {
		return nil, notDefined
	}
	return svc, nil
}

func (s *ServiceInterfaceHandlerPodman) Update(service *types.ServiceInterface) error {
	return s.manipulateService(service, serviceUpdate)
}

func (s *ServiceInterfaceHandlerPodman) Delete(address string) error {
	return s.manipulateService(&types.ServiceInterface{Address: address}, serviceDelete)
}

func (s *ServiceInterfaceHandlerPodman) ToServicePodman(svcIface *types.ServiceInterface) (*ServicePodman, error) {
	svc := &ServicePodman{
		ServiceCommon: &domain.ServiceCommon{
			Address:        svcIface.Address,
			Protocol:       svcIface.Protocol,
			Ports:          svcIface.Ports,
			EventChannel:   svcIface.EventChannel,
			Aggregate:      svcIface.Aggregate,
			Labels:         svcIface.Labels,
			Origin:         svcIface.Origin,
			Tls:            svcIface.EnableTls,
			TlsCredentials: svcIface.TlsCredentials,
			Ingress:        &domain.AddressIngressCommon{},
		},
	}

	// set default ingress
	ingressPorts := map[int]int{}
	for _, port := range svcIface.Ports {
		ingressPorts[port] = 0
	}
	svc.Ingress.SetPorts(ingressPorts)

	// local service
	if svcIface.Origin == "" {
		// retrieve container for respective address
		containers, err := s.cli.ContainerList()
		if err != nil {
			return nil, fmt.Errorf("error retrieving containers - %w", err)
		}
		var svcContainer *container.Container
		for _, c := range containers {
			if addr, ok := c.Labels[types.AddressQualifier]; ok && addr == svcIface.Address {
				svcContainer, err = s.cli.ContainerInspect(c.Name)
				if err != nil {
					return nil, fmt.Errorf("error reading container info %s - %w", c.Name, err)
				}
				break
			}
		}
		if svcContainer == nil {
			return nil, fmt.Errorf("service container could not be found")
		}

		// setting remaining information
		svc.ContainerName = svcContainer.Name

		// reading ingress info from container spec
		if len(svcContainer.Ports) > 0 {
			for _, port := range svcContainer.Ports {
				svcPort, _ := strconv.Atoi(port.Target)
				hostPort, _ := strconv.Atoi(port.Host)
				svc.Ingress.GetPorts()[svcPort] = hostPort
				svc.Ingress.SetHost(port.HostIP)
			}
		}
		for _, target := range svcIface.Targets {
			svc.EgressResolvers = append(svc.EgressResolvers, domain.EgressResolverFromString(target.Name))
		}
	}

	return svc, nil
}
