package main

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/network"
	"github.com/skupperproject/skupper/pkg/utils/formatter"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/domain"
	"github.com/skupperproject/skupper/pkg/domain/podman"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	ValidBindTypes = []string{BindTypeHost}
)

const (
	BindTypeHost string = "host"
)

type PodmanServiceCreateFlags struct {
	ContainerName string
	HostIP        string
	HostPorts     []string
	Labels        map[string]string
}

type PodmanExposeFlags struct {
	*PodmanServiceCreateFlags
}

func (p *PodmanServiceCreateFlags) HasHostBindings() bool {
	return p.HostIP != "" || len(p.HostPorts) > 0
}

func (p *PodmanServiceCreateFlags) ToPortMapping(service podman.Service) (map[int]int, error) {
	if len(p.HostPorts) > 0 && len(service.Ports) != len(p.HostPorts) {
		return nil, fmt.Errorf("service defines %d ports but only %d mapped (all ports must be mapped)",
			len(service.Ports), len(p.HostPorts))
	}
	ports := map[int]int{}
	for _, port := range service.Ports {
		ports[port] = port
	}
	for i, port := range p.HostPorts {
		portSplit := strings.SplitN(port, ":", 2)
		var sPort, hostPort string
		sPort = portSplit[0]
		hostPort = sPort
		mapping := false
		if len(portSplit) == 2 {
			hostPort = portSplit[1]
			mapping = true
		}
		var isp, ihp int
		var err error
		if isp, err = strconv.Atoi(sPort); err != nil {
			return nil, fmt.Errorf("invalid service port: %s", sPort)
		}
		if ihp, err = strconv.Atoi(hostPort); err != nil {
			return nil, fmt.Errorf("invalid host port: %s", hostPort)
		}
		if _, ok := ports[isp]; mapping && !ok {
			return nil, fmt.Errorf("%d is not a valid service port", isp)
		}
		// if service port not mapped, use positional index to determine it
		if !mapping {
			isp = service.Ports[i]
		}
		ports[isp] = ihp
	}
	return ports, nil
}

type SkupperPodmanService struct {
	podman               *SkupperPodman
	svcHandler           *podman.ServiceHandler
	svcIfaceHandler      *podman.ServiceInterfaceHandler
	createFlags          PodmanServiceCreateFlags
	exposeFlags          PodmanExposeFlags
	networkStatusHandler *podman.NetworkStatusHandler
}

func (s *SkupperPodmanService) Create(cmd *cobra.Command, args []string) error {
	servicePodman, err := s.svcIfaceHandler.ToServicePodman(&serviceToCreate, true)
	if err != nil {
		return err
	}
	// Set custom container name and labels
	servicePodman.ContainerName = s.createFlags.ContainerName
	servicePodman.Labels = s.createFlags.Labels

	// Validating if ingress host/ports should be bound
	if s.createFlags.HasHostBindings() {
		hostIp := s.createFlags.HostIP
		portMap, err := s.createFlags.ToPortMapping(*servicePodman)
		if err != nil {
			return err
		}
		servicePodman.Ingress.SetHost(hostIp)
		servicePodman.Ingress.SetPorts(portMap)
	}

	// Create service
	if err = s.svcHandler.Create(servicePodman); err != nil {
		return err
	}
	return nil
}

func (s *SkupperPodmanService) CreateFlags(cmd *cobra.Command) {
	s.createFlags.Labels = map[string]string{}
	cmd.Flags().StringVar(&s.createFlags.ContainerName, "container-name", "", "Use a different container name")
	cmd.Flags().StringVar(&s.createFlags.HostIP, "host-ip", "", "Host IP address used to bind service ports")
	cmd.Flags().StringSliceVar(&s.createFlags.HostPorts, "host-port", []string{}, "The host ports to bind with the service (you can also use colon to map service-port to a host-port).")
	cmd.Flags().StringToStringVar(&s.createFlags.Labels, "label", s.createFlags.Labels, "Labels to the new service (comma separated list of key and value pairs split by equals")
}

func (s *SkupperPodmanService) Delete(cmd *cobra.Command, args []string) error {
	err := s.svcHandler.Delete(args[0])
	if err != nil {
		return err
	}
	return nil
}

func (s *SkupperPodmanService) DeleteFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) List(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) ListFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Status(cmd *cobra.Command, args []string) error {
	services, err := s.svcHandler.List()
	mapServiceLabels := make(map[string]map[string]string)
	localPodmanSiteInfo := network.LocalSiteInfo{SiteId: s.podman.currentSite.Id, ServiceInfo: make(map[string]network.LocalServiceInfo)}

	if err == nil {
		if len(services) > 0 {

			addresses := []string{}
			for _, si := range services {
				addresses = append(addresses, si.GetAddress())
			}

			for _, svc := range services {
				svcPodman := svc.(*podman.Service)

				serviceInfo := make(map[string][]string)
				for _, port := range svcPodman.GetPorts() {
					portStr := strconv.Itoa(port)
					serviceName := svcPodman.GetAddress()

					containerPorts := svcPodman.ContainerPorts()
					if len(containerPorts) > 0 {
						ingressKey := "Host ports:"
						ingressInfo := fmt.Sprintf("ip: %s - ports: ", utils.DefaultStr(svcPodman.Ingress.GetHost(), "*"))
						for i, portInfo := range containerPorts {
							if i > 0 {
								ingressInfo += ", "
							}
							ingressInfo += fmt.Sprintf("%s -> %s", portInfo.Host, portInfo.Target)
						}
						serviceInfo[ingressKey] = append(serviceInfo[ingressKey], ingressInfo)
					}
					if len(svcPodman.GetEgressResolvers()) > 0 {
						targetKey := "Targets:"
						for _, t := range svcPodman.GetEgressResolvers() {
							targetInfo := ""
							if resolverHost, ok := t.(*domain.EgressResolverHost); ok {
								targetInfo = fmt.Sprintf("host: %s - ports: %v", resolverHost.Host, resolverHost.Ports)
							}
							serviceInfo[targetKey] = append(serviceInfo[targetKey], targetInfo)
						}
					}

					address := serviceName + ":" + portStr
					localPodmanSiteInfo.ServiceInfo[address] = network.LocalServiceInfo{Data: serviceInfo}
					mapServiceLabels[address] = svcPodman.GetLabels()

				}
			}

		}
	}

	cli := s.podman.currentSite

	configSyncVersion := utils.GetVersionTag(cli.GetVersion())
	if configSyncVersion != "" && !utils.IsValidFor(configSyncVersion, network.MINIMUM_PODMAN_VERSION) {
		fmt.Printf(network.MINIMUM_VERSION_MESSAGE, configSyncVersion, network.MINIMUM_PODMAN_VERSION)
		fmt.Println()
		return nil
	}

	currentStatus, errStatus := s.NetworkStatusHandler().Get()
	if errStatus != nil && strings.HasPrefix(errStatus.Error(), "Skupper is not installed") {
		fmt.Printf("Skupper is not enabled\n")
		return nil
	} else if errStatus != nil && errStatus.Error() == "status not ready" {
		fmt.Println("Status pending...")
		return nil
	} else if errStatus != nil {
		return errStatus
	}

	var servicesToDisplay []network.AddressInfo
	for _, addr := range currentStatus.Addresses {
		if _, exist := mapServiceLabels[addr.Name]; exist {
			servicesToDisplay = append(servicesToDisplay, addr)
		}

	}
	currentStatus.Addresses = servicesToDisplay

	err = formatter.PrintServiceStatus(currentStatus, mapServiceLabels, verboseServiceStatus, showLabels, &localPodmanSiteInfo)
	if err != nil {
		return err
	}

	return nil
}

func (s *SkupperPodmanService) StatusFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&verboseServiceStatus, "verbose", "v", false, "more detailed output")
}

func (s *SkupperPodmanService) NewClient(cmd *cobra.Command, args []string) {
	s.podman.NewClient(cmd, args)
	s.svcHandler = podman.NewServiceHandlerPodman(s.podman.cli)
	s.svcIfaceHandler = podman.NewServiceInterfaceHandlerPodman(s.podman.cli)
}

func (s *SkupperPodmanService) Platform() types.Platform {
	return s.podman.Platform()
}

func (s *SkupperPodmanService) Label(cmd *cobra.Command, args []string) error {
	return notImplementedErr
}

func (s *SkupperPodmanService) Bind(cmd *cobra.Command, args []string) error {
	address := args[0]
	host := args[2]

	// retrieving service
	service, err := s.svcHandler.Get(address)
	if err != nil {
		return err
	}
	podmanService := service.(*podman.Service)

	// validating ports
	portMapping, err := parsePortMapping(podmanService.AsServiceInterface(), bindOptions.TargetPorts)
	if err != nil {
		return err
	}

	// Setting up the egress info
	egressResolver := &domain.EgressResolverHost{}
	egressResolver.Ports = portMapping
	egressResolver.Host = host

	return s.svcHandler.AddEgressResolver(address, egressResolver)
}

func (s *SkupperPodmanService) BindArgs(cmd *cobra.Command, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("Service name, target type and target value must all be specified (e.g. 'skupper bind <service-name> <target-type> <target-value>')")
	}
	if len(args) > 3 {
		return fmt.Errorf("illegal argument: %s", args[3])
	}
	if !utils.StringSliceContains(ValidBindTypes, args[1]) {
		return fmt.Errorf("invalid target type: %s - valid target types are: %s", args[1], ValidBindTypes)
	}
	switch args[1] {
	case BindTypeHost:
		host := args[2]
		if args[2] == "" {
			return fmt.Errorf("a hostname or IP is required")
		}
		if net.ParseIP(host) == nil {
			parsedUrl, err := url.Parse("http://" + host)
			if err != nil || parsedUrl.Hostname() != host {
				return fmt.Errorf("invalid hostname or ip")
			}
		}
	}
	return nil
}

func (s *SkupperPodmanService) BindFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Unbind(cmd *cobra.Command, args []string) error {
	address := args[0]
	host := args[2]

	// retrieving service
	service, err := s.svcHandler.Get(address)
	if err != nil {
		return err
	}
	podmanService := service.(*podman.Service)

	// Retrieving egress info
	var egressResolver *domain.EgressResolver
	for _, e := range podmanService.GetEgressResolvers() {
		egressResolverHost := e.(*domain.EgressResolverHost)
		if egressResolverHost.Host == host {
			egressResolver = &e
			break
		}
	}
	if egressResolver == nil {
		return fmt.Errorf("Could not find target %s for service interface %s", host, address)
	}
	return s.svcHandler.RemoveEgressResolver(address, *egressResolver)
}

func (s *SkupperPodmanService) UnbindFlags(cmd *cobra.Command) {}

func (s *SkupperPodmanService) Expose(cmd *cobra.Command, args []string) error {
	servicePodman := &podman.Service{
		ServiceCommon: &domain.ServiceCommon{
			Address:      exposeOpts.Address,
			Protocol:     exposeOpts.Protocol,
			Ports:        exposeOpts.Ports,
			EventChannel: exposeOpts.EventChannel,
			Aggregate:    exposeOpts.Aggregate,
			Labels:       s.exposeFlags.Labels,
			Ingress:      &domain.AddressIngressCommon{},
		},
		ContainerName: s.exposeFlags.ContainerName,
	}
	if exposeOpts.GeneratedCerts {
		servicePodman.SetTlsCredentials(exposeOpts.Address)
	}

	// Validating if ingress host/ports should be bound
	if s.exposeFlags.HasHostBindings() {
		hostIp := s.exposeFlags.HostIP
		portMap, err := s.exposeFlags.ToPortMapping(*servicePodman)
		if err != nil {
			return err
		}
		servicePodman.Ingress.SetHost(hostIp)
		servicePodman.Ingress.SetPorts(portMap)
	}

	// Exposed resource
	host := args[1]

	// validating ports
	portMapping, err := parsePortMapping(servicePodman.AsServiceInterface(), exposeOpts.TargetPorts)
	if err != nil {
		return err
	}

	// Setting up the egress info
	egressResolver := &domain.EgressResolverHost{}
	egressResolver.Ports = portMapping
	egressResolver.Host = host
	servicePodman.AddEgressResolver(egressResolver)

	// Create service
	if err := s.svcHandler.Create(servicePodman); err != nil {
		return err
	}

	return nil
}

func (s *SkupperPodmanService) ExposeArgs(cmd *cobra.Command, args []string) error {
	return s.checkCommonExposeArgs(cmd, args)
}

func (s *SkupperPodmanService) ExposeFlags(cmd *cobra.Command) {
	cmd.Use = "expose [host <hostname|ip>]"
	cmd.Short = "Expose one or more network services"
	cmd.Example = `
        # exposing a service running on the local machine
        skupper expose host host.containers.internal --address my-service --port 8080

        # exposing a local network IP
        skupper expose host 10.0.0.1 --address my-service --port 8080

        # exposing a podman container connected to the same podman network
        skupper expose host my-container --address my-service --port 8080`

	s.createFlags.Labels = map[string]string{}
	s.exposeFlags.PodmanServiceCreateFlags = &PodmanServiceCreateFlags{}
	cmd.Flags().StringVar(&s.exposeFlags.ContainerName, "container-name", "", "Use a different container name")
	cmd.Flags().StringVar(&s.exposeFlags.HostIP, "host-ip", "", "Host IP address used to bind service ports")
	cmd.Flags().StringSliceVar(&s.exposeFlags.HostPorts, "host-port", []string{}, "The host ports to bind with the service (you can also use colon to map service-port to a host-port).")
	cmd.Flags().StringToStringVar(&s.exposeFlags.Labels, "label", s.createFlags.Labels, "Labels to the new service (comma separated list of key and value pairs split by equals")
}

func (s *SkupperPodmanService) Unexpose(cmd *cobra.Command, args []string) error {
	address := unexposeAddress
	host := args[1]

	// retrieving service
	service, err := s.svcHandler.Get(address)
	if err != nil {
		return err
	}
	podmanService := service.(*podman.Service)

	// Retrieving egress info
	var egressResolver *domain.EgressResolver
	for _, e := range podmanService.GetEgressResolvers() {
		egressResolverHost := e.(*domain.EgressResolverHost)
		if egressResolverHost.Host == host {
			egressResolver = &e
			break
		}
	}
	if egressResolver == nil {
		return fmt.Errorf("Could not find target %s for service interface %s", host, address)
	}

	if len(podmanService.GetEgressResolvers()) > 1 {
		return s.svcHandler.RemoveEgressResolver(address, *egressResolver)
	} else {
		return s.svcHandler.Delete(address)
	}
}

func (s *SkupperPodmanService) UnexposeFlags(cmd *cobra.Command) error {
	cmd.Use = "unexpose [host <hostname|ip>]"
	cmd.Short = "Unexpose one or more network services"
	cmd.Example = `
        # unexposing a service running on the local machine
        skupper unexpose host host.containers.internal --address my-service

        # unexposing a local network IP
        skupper unexpose host 10.0.0.1 --address my-service

        # unexposing a podman container connected to the same podman network
        skupper unexpose host my-container --address my-service`
	return nil
}

func (s *SkupperPodmanService) UnexposeArgs(cmd *cobra.Command, args []string) error {
	return s.checkCommonExposeArgs(cmd, args)
}

func (s *SkupperPodmanService) checkCommonExposeArgs(cmd *cobra.Command, args []string) error {
	address, err := cmd.Flags().GetString("address")
	if err != nil || address == "" {
		return fmt.Errorf("--address is required")
	}
	if exposeOpts.Address != "" && len(exposeOpts.Ports) == 0 {
		return fmt.Errorf("--port is required")
	}
	if len(args) < 2 {
		return fmt.Errorf("Target type and target value must all be specified (e.g. 'skupper %s <target-type> <target-value>')", cmd.Name())
	}
	if len(args) > 2 {
		return fmt.Errorf("illegal argument: %s", args[2])
	}
	if !utils.StringSliceContains(ValidBindTypes, args[0]) {
		return fmt.Errorf("invalid target type: %s - valid target types are: %s", args[0], ValidBindTypes)
	}
	switch args[0] {
	case BindTypeHost:
		host := args[1]
		if args[1] == "" {
			return fmt.Errorf("a hostname or IP is required")
		}
		if net.ParseIP(host) == nil {
			parsedUrl, err := url.Parse("http://" + host)
			if err != nil || parsedUrl.Hostname() != host {
				return fmt.Errorf("invalid hostname or ip")
			}
		}
	}
	return nil
}

func (s *SkupperPodmanService) NetworkStatusHandler() *podman.NetworkStatusHandler {
	if s.networkStatusHandler != nil {
		return s.networkStatusHandler
	}
	if s.podman.cli == nil {
		return nil
	}
	s.networkStatusHandler = new(podman.NetworkStatusHandler).WithClient(s.podman.cli)
	return s.networkStatusHandler
}
