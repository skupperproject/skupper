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
	return svc
}

func (s *ServicePodman) ContainerPorts() []container.Port {
	ports := []container.Port{}
	for port, hostPort := range s.GetIngress().GetPorts() {
		ports = append(ports, container.Port{
			Host:     strconv.Itoa(hostPort),
			HostIP:   s.GetIngress().GetHost(),
			Target:   strconv.Itoa(port),
			Protocol: s.GetProtocol(),
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
		return fmt.Errorf("Service %s already defined")
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
	if servicePodman.Ingress.GetPorts() != nil && len(servicePodman.Ingress.GetPorts()) > 0 {
		for port, _ := range servicePodman.Ingress.GetPorts() {
			if utils.TcpPortInUse(servicePodman.Ingress.GetHost(), port) {
				return fmt.Errorf("ingress port %d is already in use", port)
			}
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

	// Will be used as an env var
	var svcRouterConfig *qdr.RouterConfig
	var svcRouterConfigStr string
	svcRouterConfig, svcRouterConfigStr, err = domain.CreateRouterServiceConfig(site, routerConfig, servicePodman)

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
	site.GetDeployments()[0].GetComponents()[0].GetImage()
	c := &container.Container{
		Name:  servicePodman.GetContainerName(),
		Image: utils.DefaultStr(routerContainer.Image, types.GetRouterImageName()),
		Env: map[string]string{
			"APPLICATION_NAME":    svcRouterConfig.GetSiteMetadata().Id,
			"QDROUTERD_CONF":      svcRouterConfigStr,
			"QDROUTERD_CONF_TYPE": "json",
			"SKUPPER_SITE_ID":     svcRouterConfig.GetSiteMetadata().Id,
			"QDROUTERD_DEBUG":     routerContainer.Env["QDROUTERD_DEBUG"],
		},
		Labels: map[string]string{
			types.AddressQualifier: servicePodman.GetAddress(),
		},
		Networks:      routerContainer.Networks,
		Mounts:        routerContainer.Mounts,
		Ports:         servicePodman.ContainerPorts(),
		RestartPolicy: "always",
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
	// TODO implement me
	panic("implement me")
}

func (s *ServiceHandlerPodman) AddTargets(address string, targets []types.ServiceInterfaceTarget) error {
	// TODO implement me
	panic("implement me")
}

func (s *ServiceHandlerPodman) RemoveTargets(address string, targets []types.ServiceInterfaceTarget) error {
	// TODO implement me
	panic("implement me")
}

func (s *ServiceHandlerPodman) RemoveAllTargets() error {
	// TODO implement me
	panic("implement me")
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
	if svcMap == nil {
		return nil, nil
	}
	svc := svcMap[address]
	return svc, nil
}

func (s *ServiceInterfaceHandlerPodman) Update(service *types.ServiceInterface) error {
	return s.manipulateService(service, serviceUpdate)
}

func (s *ServiceInterfaceHandlerPodman) Delete(address string) error {
	return s.manipulateService(&types.ServiceInterface{Address: address}, serviceDelete)
}
