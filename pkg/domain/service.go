package domain

import (
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/version"
	"k8s.io/apimachinery/pkg/util/validation"
)

// ServiceHandler generic interface to manipulate services across platforms
type ServiceHandler interface {
	Create(service Service) error
	Delete(address string) error
	Get(address string) (Service, error)
	List() ([]Service, error)
	AddEgressResolver(address string, egressResolver EgressResolver) error
	RemoveEgressResolver(address string, egressResolver EgressResolver) error
	RemoveAllEgressResolvers(address string) error
}

// Service defines a generic representation of a Skupper service
type Service interface {
	GetAddress() string
	SetAddress(address string)
	GetProtocol() string
	SetProtocol(protocol string)
	GetPorts() []int
	SetPorts(ports []int)
	IsEventChannel() bool
	SetEventChannel(eventChannel bool)
	GetAggregate() string
	SetAggregate(strategy string)
	GetLabels() map[string]string
	SetLabels(labels map[string]string)
	GetOrigin() string
	SetOrigin(origin string)
	IsTls() bool
	SetTls(tls bool)
	GetTlsCredentials() string
	SetTlsCredentials(credential string)
	GetIngress() AddressIngress
	SetIngress(ingress AddressIngress)
	GetEgressResolvers() []EgressResolver
	AddEgressResolver(resolver EgressResolver)
	SetEgressResolvers(resolvers []EgressResolver)
}

type ServiceCommon struct {
	Address         string
	Protocol        string
	Ports           []int
	EventChannel    bool
	Aggregate       string
	Labels          map[string]string
	Origin          string
	Tls             bool
	TlsCredentials  string
	Ingress         AddressIngress
	EgressResolvers []EgressResolver
}

func (s *ServiceCommon) GetAddress() string {
	return s.Address
}

func (s *ServiceCommon) SetAddress(address string) {
	s.Address = address
}

func (s *ServiceCommon) GetProtocol() string {
	return s.Protocol
}

func (s *ServiceCommon) SetProtocol(protocol string) {
	s.Protocol = protocol
}

func (s *ServiceCommon) GetPorts() []int {
	return s.Ports
}

func (s *ServiceCommon) SetPorts(ports []int) {
	s.Ports = ports
}

func (s *ServiceCommon) IsEventChannel() bool {
	return s.EventChannel
}

func (s *ServiceCommon) SetEventChannel(eventChannel bool) {
	s.EventChannel = eventChannel
}

func (s *ServiceCommon) GetAggregate() string {
	return s.Aggregate
}

func (s *ServiceCommon) SetAggregate(strategy string) {
	s.Aggregate = strategy
}

func (s *ServiceCommon) GetLabels() map[string]string {
	return s.Labels
}

func (s *ServiceCommon) SetLabels(labels map[string]string) {
	s.Labels = labels
}

func (s *ServiceCommon) GetOrigin() string {
	return s.Origin
}

func (s *ServiceCommon) SetOrigin(origin string) {
	s.Origin = origin
}

func (s *ServiceCommon) IsTls() bool {
	return s.Tls
}

func (s *ServiceCommon) SetTls(tls bool) {
	s.Tls = tls
}

func (s *ServiceCommon) GetTlsCredentials() string {
	return s.TlsCredentials
}

func (s *ServiceCommon) SetTlsCredentials(credential string) {
	s.TlsCredentials = credential
}

func (s *ServiceCommon) GetIngress() AddressIngress {
	return s.Ingress
}

func (s *ServiceCommon) SetIngress(ingress AddressIngress) {
	s.Ingress = ingress
}

func (s *ServiceCommon) GetEgressResolvers() []EgressResolver {
	return s.EgressResolvers
}

func (s *ServiceCommon) SetEgressResolvers(resolvers []EgressResolver) {
	s.EgressResolvers = resolvers
}

func (s *ServiceCommon) AddEgressResolver(resolver EgressResolver) {
	s.EgressResolvers = append(s.EgressResolvers, resolver)
}

func ValidateService(service Service) error {
	errs := validation.IsDNS1035Label(service.GetAddress())
	if len(errs) > 0 {
		return fmt.Errorf("Invalid service name: %q", errs)
	}

	for _, resolver := range service.GetEgressResolvers() {
		targets, err := resolver.Resolve()
		if err != nil {
			return fmt.Errorf("error resolving egresses - %w", err)
		}
		for _, target := range targets {
			for _, targetPort := range target.GetPorts() {
				if targetPort < 0 || 65535 < targetPort {
					return fmt.Errorf("Bad target port number. Target: %s  Port: %d", resolver.String(), targetPort)
				}
			}
		}
	}

	for _, port := range service.GetPorts() {
		if port < 0 || 65535 < port {
			return fmt.Errorf("Port %d is outside valid range.", port)
		}
	}
	if service.GetAddress() != "" && service.IsEventChannel() {
		return fmt.Errorf("Only one of aggregate and event-channel can be specified for a given service.")
	} else if service.GetAggregate() != "" && service.GetAggregate() != "json" && service.GetAggregate() != "multipart" {
		return fmt.Errorf("%s is not a valid aggregation strategy. Choose 'json' or 'multipart'.", service.GetAggregate())
	} else if service.GetProtocol() != "" && service.GetProtocol() != "tcp" && service.GetProtocol() != "http" && service.GetProtocol() != "http2" {
		return fmt.Errorf("%s is not a valid mapping. Choose 'tcp', 'http' or 'http2'.", service.GetProtocol())
	} else if service.GetAggregate() != "" && service.GetProtocol() != "http" {
		return fmt.Errorf("The aggregate option is currently only valid for http")
	} else if service.IsEventChannel() && service.GetProtocol() != "http" {
		return fmt.Errorf("The event-channel option is currently only valid for http")
	} else if service.IsTls() && service.GetProtocol() != "http2" {
		return fmt.Errorf("The TLS support is only available for http2")
	} else {
		return nil
	}
}

func CreateRouterServiceConfig(site Site, parentRouterConfig *qdr.RouterConfig, service Service) (*qdr.RouterConfig, string, error) {

	// Create router config
	siteName := fmt.Sprintf("%s-%s", site.GetName(), service.GetAddress())
	siteId := fmt.Sprintf("%s-%s", site.GetId(), service.GetAddress())

	// Adjust logging level
	svcRouterConfig := qdr.InitialConfig(siteName, siteId, version.Version, true, 3)
	svcRouterConfig.LogConfig = parentRouterConfig.LogConfig

	// Setting sslProfiles
	svcRouterConfig.AddSslProfile(qdr.SslProfile{Name: "skupper-internal"})
	tlsCredentialName := types.SkupperServiceCertPrefix + service.GetAddress()
	if service.IsTls() {
		svcRouterConfig.AddSslProfile(qdr.SslProfile{Name: tlsCredentialName})
	}

	// local AMQP listener
	svcRouterConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "127.0.0.1",
		Port: 5672,
	})

	// Setting up edge connector
	svcRouterConfig.AddConnector(qdr.Connector{
		Name:       "uplink",
		Role:       qdr.RoleEdge,
		Host:       "skupper-router",
		Port:       strconv.Itoa(int(types.EdgeListenerPort)),
		SslProfile: "skupper-internal",
	})

	// Setting up addaptor listeners
	for _, port := range service.GetPorts() {
		listenerName := fmt.Sprintf("%s:%d", service.GetAddress(), port)
		listenerPort := strconv.Itoa(port)
		listenerAddr := fmt.Sprintf("%s:%d", service.GetAddress(), port)
		switch service.GetProtocol() {
		case "tcp":
			svcRouterConfig.AddTcpListener(qdr.TcpEndpoint{
				Name:    listenerName,
				Port:    listenerPort,
				Address: listenerAddr,
				SiteId:  siteId,
			})
		case "http":
			svcRouterConfig.AddHttpListener(qdr.HttpEndpoint{
				Name:         listenerName,
				Port:         listenerPort,
				Address:      listenerAddr,
				SiteId:       siteId,
				Aggregation:  service.GetAggregate(),
				EventChannel: service.IsEventChannel(),
				SslProfile:   service.GetTlsCredentials(),
			})
		case "http2":
			svcRouterConfig.AddHttpListener(qdr.HttpEndpoint{
				Name:            listenerName,
				Port:            listenerPort,
				Address:         listenerAddr,
				SiteId:          siteId,
				ProtocolVersion: qdr.HttpVersion2,
				Aggregation:     service.GetAggregate(),
				EventChannel:    service.IsEventChannel(),
				SslProfile:      service.GetTlsCredentials(),
			})
		}
	}

	// If egress resolvers defined, resolve the respective connectors
	for _, egressResolver := range service.GetEgressResolvers() {
		if err := ServiceRouterConfigAddTargets(site, &svcRouterConfig, service, egressResolver); err != nil {
			return nil, "", err
		}
	}

	svcRouterConfigStr, err := qdr.MarshalRouterConfig(svcRouterConfig)
	return &svcRouterConfig, svcRouterConfigStr, err
}

func RouterConnectorNamesForEgress(address string, target AddressEgress) map[int]string {
	names := map[int]string{}
	for port, targetPort := range target.GetPorts() {
		names[port] = fmt.Sprintf("%s@%s:%d:%d", address, target.GetHost(), port, targetPort)
	}
	return names
}

func ServiceRouterConfigAddTargets(site Site, svcRouterConfig *qdr.RouterConfig, service Service, egressResolver EgressResolver) error {
	siteId := fmt.Sprintf("%s-%s", site.GetId(), service.GetAddress())

	boolFalse := false
	targets, err := egressResolver.Resolve()
	if err != nil {
		return fmt.Errorf("error resolving egresses - %w", err)
	}
	for _, target := range targets {
		connectorNames := RouterConnectorNamesForEgress(service.GetAddress(), target)
		for port, targetPort := range target.GetPorts() {
			connectorName := connectorNames[port]
			connectorHost := target.GetHost()
			connectorPort := strconv.Itoa(targetPort)
			connectorAddr := fmt.Sprintf("%s:%d", service.GetAddress(), port)
			switch service.GetProtocol() {
			case "tcp":
				svcRouterConfig.AddTcpConnector(qdr.TcpEndpoint{
					Name:    connectorName,
					Host:    connectorHost,
					Port:    connectorPort,
					Address: connectorAddr,
					SiteId:  siteId,
				})
			case "http":
				svcRouterConfig.AddHttpConnector(qdr.HttpEndpoint{
					Name:           connectorName,
					Host:           connectorHost,
					Port:           connectorPort,
					Address:        connectorAddr,
					SiteId:         siteId,
					Aggregation:    service.GetAggregate(),
					EventChannel:   service.IsEventChannel(),
					SslProfile:     service.GetTlsCredentials(),
					VerifyHostname: &boolFalse,
				})
			case "http2":
				svcRouterConfig.AddHttpConnector(qdr.HttpEndpoint{
					Name:            connectorName,
					Host:            connectorHost,
					Port:            connectorPort,
					Address:         connectorAddr,
					SiteId:          siteId,
					ProtocolVersion: qdr.HttpVersion2,
					Aggregation:     service.GetAggregate(),
					EventChannel:    service.IsEventChannel(),
					SslProfile:      service.GetTlsCredentials(),
					VerifyHostname:  &boolFalse,
				})
			}
		}
	}
	return nil
}

func ServiceRouterConfigRemoveTargets(svcRouterConfig *qdr.RouterConfig, service Service, egressResolver EgressResolver) error {
	if egressResolver == nil {
		svcRouterConfig.Bridges.TcpConnectors = map[string]qdr.TcpEndpoint{}
		svcRouterConfig.Bridges.HttpConnectors = map[string]qdr.HttpEndpoint{}
		return nil
	}
	targets, err := egressResolver.Resolve()
	if err != nil {
		return fmt.Errorf("error resolving egresses - %w", err)
	}
	for _, target := range targets {
		connectorNames := RouterConnectorNamesForEgress(service.GetAddress(), target)
		for port, _ := range target.GetPorts() {
			connectorName := connectorNames[port]
			switch service.GetProtocol() {
			case "tcp":
				svcRouterConfig.RemoveTcpConnector(connectorName)
			case "http":
			case "http2":
				svcRouterConfig.RemoveHttpConnector(connectorName)
			}
		}
	}
	return nil
}
