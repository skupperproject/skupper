package service

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func getBridgeName(address string, host string, port ...int) string {
	portSuffix := func(port ...int) string {
		s := ""
		for _, p := range port {
			s += ":" + strconv.Itoa(p)
		}
		return s
	}
	if host == "" {
		return address + portSuffix(port...)
	} else {
		return address + "@" + host + portSuffix(port...)
	}
}

type TargetResolver interface {
	Close()
	List() []string
	HasTarget() bool
}

type ServiceIngress interface {
	Realise(binding *ServiceBindings) error
	Mode() types.ServiceIngressMode
	Matches(def *types.ServiceInterface) bool
}

type ExternalBridge interface {
	Realise() error
	Matches(def *types.ServiceInterface) bool
}

type ServiceBindingContext interface {
	NewTargetResolver(address string, selector string, skipTargetStatus bool, namespace string) (TargetResolver, error)
	NewServiceIngress(def *types.ServiceInterface) ServiceIngress
	NewExternalBridge(def *types.ServiceInterface) ExternalBridge
}

type EgressBindings struct {
	name           string
	Selector       string
	service        string
	egressPorts    map[int]int
	resolver       TargetResolver
	tlsCredentials string
	namespace      string
}

type ServiceBindings struct {
	origin                   string
	protocol                 string
	Address                  string
	publicPorts              []int
	ingressPorts             []int
	ingressBinding           ServiceIngress
	aggregation              string
	eventChannel             bool
	headless                 *types.Headless
	Labels                   map[string]string
	Annotations              map[string]string
	targets                  map[string]*EgressBindings
	TlsCredentials           string
	TlsCertAuthority         string
	PublishNotReadyAddresses bool
	external                 ExternalBridge
	Namespace                string
}

func (s *ServiceBindings) RequiresExternalBridge() bool {
	return s.external != nil
}

func (s *ServiceBindings) GetIngressPorts() []int {
	return s.ingressPorts
}

func (s *ServiceBindings) SetIngressPorts(ports []int) {
	s.ingressPorts = ports
}

func (s *ServiceBindings) FindLocalTarget() *EgressBindings {
	for _, eb := range s.targets {
		if eb.hasLocalTarget() {
			return eb
		}
	}
	return nil
}

func (s *ServiceBindings) PortMap() map[int]int {
	ports := map[int]int{}
	for i := 0; i < len(s.publicPorts); i++ {
		ports[s.publicPorts[i]] = s.ingressPorts[i]
	}
	return ports
}

func (s *ServiceBindings) Protocol() string {
	return s.protocol
}

func (bindings *ServiceBindings) AsServiceInterface() types.ServiceInterface {
	var mode types.ServiceIngressMode
	if bindings.ingressBinding != nil {
		mode = bindings.ingressBinding.Mode()
	}
	return types.ServiceInterface{
		Address:                  bindings.Address,
		Protocol:                 bindings.protocol,
		Ports:                    bindings.publicPorts,
		ExposeIngress:            mode,
		Aggregate:                bindings.aggregation,
		EventChannel:             bindings.eventChannel,
		Headless:                 bindings.headless,
		Labels:                   bindings.Labels,
		Annotations:              bindings.Annotations,
		Origin:                   bindings.origin,
		TlsCredentials:           bindings.TlsCredentials,
		TlsCertAuthority:         bindings.TlsCertAuthority,
		PublishNotReadyAddresses: bindings.PublishNotReadyAddresses,
	}
}

func getTargetPorts(service types.ServiceInterface, target types.ServiceInterfaceTarget) map[int]int {
	targetPorts := target.TargetPorts
	if len(targetPorts) == 0 {
		targetPorts = map[int]int{}
		for _, port := range service.Ports {
			targetPorts[port] = port
		}
	}
	return targetPorts
}

func hasTargetForSelector(si types.ServiceInterface, selector string) bool {
	for _, t := range si.Targets {
		if t.Selector == selector {
			return true
		}
	}
	return false
}

func hasTargetForService(si types.ServiceInterface, service string) bool {
	for _, t := range si.Targets {
		if t.Service == service {
			return true
		}
	}
	return false
}

func NewServiceBindings(required types.ServiceInterface, ports []int, bindingContext ServiceBindingContext) *ServiceBindings {
	sb := &ServiceBindings{
		origin:                   required.Origin,
		protocol:                 required.Protocol,
		Address:                  required.Address,
		publicPorts:              required.Ports,
		ingressPorts:             ports,
		ingressBinding:           bindingContext.NewServiceIngress(&required),
		aggregation:              required.Aggregate,
		eventChannel:             required.EventChannel,
		headless:                 required.Headless,
		Labels:                   required.Labels,
		Annotations:              required.Annotations,
		targets:                  map[string]*EgressBindings{},
		TlsCredentials:           required.TlsCredentials,
		TlsCertAuthority:         required.TlsCertAuthority,
		PublishNotReadyAddresses: required.PublishNotReadyAddresses,
	}
	if required.RequiresExternalBridge() {
		sb.external = bindingContext.NewExternalBridge(&required)
	}
	for _, t := range required.Targets {
		if t.Selector != "" {
			sb.addSelectorTarget(t.Name, t.Selector, getTargetPorts(required, t), t.Namespace, bindingContext)
		} else if t.Service != "" {
			sb.addServiceTarget(t.Name, t.Service, getTargetPorts(required, t), t.Namespace, required.TlsCredentials)
		}
	}

	return sb
}

func (bindings *ServiceBindings) RealiseIngress() error {
	return bindings.ingressBinding.Realise(bindings)
}

func (bindings *ServiceBindings) RealiseExternalBridge() error {
	if bindings.external == nil {
		return nil
	}
	return bindings.external.Realise()
}

func (bindings *ServiceBindings) Update(required types.ServiceInterface, bindingContext ServiceBindingContext) {
	if !bindings.ingressBinding.Matches(&required) {
		bindings.ingressBinding = bindingContext.NewServiceIngress(&required)
	}
	if bindings.protocol != required.Protocol {
		bindings.protocol = required.Protocol
	}
	if !reflect.DeepEqual(bindings.publicPorts, required.Ports) {
		bindings.publicPorts = required.Ports
	}
	if bindings.aggregation != required.Aggregate {
		bindings.aggregation = required.Aggregate
	}
	if bindings.eventChannel != required.EventChannel {
		bindings.eventChannel = required.EventChannel
	}
	if required.Headless != nil {
		if bindings.headless == nil {
			bindings.headless = required.Headless
		} else {
			if bindings.headless.Name != required.Headless.Name {
				bindings.headless.Name = required.Headless.Name
			}
			if bindings.headless.Size != required.Headless.Size {
				bindings.headless.Size = required.Headless.Size
			}
			if !reflect.DeepEqual(bindings.headless.TargetPorts, required.Headless.TargetPorts) {
				bindings.headless.TargetPorts = required.Headless.TargetPorts
			}
		}
		bindings.ingressPorts = required.Ports
	} else if bindings.headless != nil {
		bindings.headless = nil
	}

	if bindings.TlsCredentials != required.TlsCredentials {

		// Credentials will be overridden only if there are no value for them,
		// and in that case a new secret has to be generated in that site.
		if len(bindings.TlsCredentials) == 0 {
			bindings.TlsCredentials = required.TlsCredentials
		}
	}

	if bindings.PublishNotReadyAddresses != required.PublishNotReadyAddresses {
		bindings.PublishNotReadyAddresses = required.PublishNotReadyAddresses
	}

	if bindings.TlsCertAuthority != required.TlsCertAuthority {
		bindings.TlsCertAuthority = required.TlsCertAuthority
	}

	if bindings.Namespace != required.Namespace {
		bindings.Namespace = required.Namespace
	}

	hasSkupperSelector := false
	for _, t := range required.Targets {
		targetPort := getTargetPorts(required, t)
		if strings.Contains(t.Selector, "skupper.io/component=router") {
			hasSkupperSelector = true
		}
		if t.Selector != "" {
			target := bindings.targets[t.Selector]
			if target == nil {
				bindings.addSelectorTarget(t.Name, t.Selector, targetPort, t.Namespace, bindingContext)
			} else {
				if !reflect.DeepEqual(target.egressPorts, targetPort) {
					target.egressPorts = targetPort
				}
				if target.namespace != t.Namespace {
					target.namespace = t.Namespace
				}
			}
		} else if t.Service != "" {
			target := bindings.targets[t.Service]
			if target == nil {
				bindings.addServiceTarget(t.Name, t.Service, targetPort, t.Namespace, required.TlsCredentials)
			} else {
				if !reflect.DeepEqual(target.egressPorts, targetPort) {
					target.egressPorts = targetPort
				}
				if target.namespace != t.Namespace {
					target.namespace = t.Namespace
				}
			}
		}
	}
	for k, v := range bindings.targets {
		if v.Selector != "" {
			if !hasTargetForSelector(required, k) && !hasSkupperSelector {
				bindings.removeSelectorTarget(k)
			}
		} else if v.service != "" {
			if !hasTargetForService(required, k) {
				bindings.removeServiceTarget(k)
			}
		}
	}
	if !reflect.DeepEqual(bindings.Labels, required.Labels) {
		if bindings.Labels == nil {
			bindings.Labels = map[string]string{}
		} else if len(required.Labels) == 0 {
			bindings.Labels = nil
		}
		for k, v := range required.Labels {
			bindings.Labels[k] = v
		}
	}
	if !reflect.DeepEqual(bindings.Annotations, required.Annotations) {
		if bindings.Annotations == nil {
			bindings.Annotations = map[string]string{}
		} else if len(required.Annotations) == 0 {
			bindings.Annotations = nil
		}
		for k, v := range required.Annotations {
			bindings.Annotations[k] = v
		}
	}
	if bindings.external == nil || !bindings.external.Matches(&required) {
		if required.RequiresExternalBridge() {
			bindings.external = bindingContext.NewExternalBridge(&required)
		} else {
			bindings.external = nil
		}
	}
}

type NullTargetResolver struct {
	targets []string
}

func (o *NullTargetResolver) Close() {
}

func (o *NullTargetResolver) List() []string {
	return o.targets
}

func (o *NullTargetResolver) HasTarget() bool {
	return len(o.targets) > 0
}

func NewNullTargetResolver(targets []string) TargetResolver {
	return &NullTargetResolver{
		targets: targets,
	}
}
func (sb *ServiceBindings) IsHeadless() bool {
	return sb.headless != nil
}

func (sb *ServiceBindings) HeadlessName() string {
	if sb.headless == nil {
		return ""
	}
	return sb.headless.Name
}

func (sb *ServiceBindings) addSelectorTarget(name string, selector string, port map[int]int, namespace string, controller ServiceBindingContext) error {
	resolver, err := controller.NewTargetResolver(sb.Address, selector, sb.PublishNotReadyAddresses, namespace)
	sb.targets[selector] = &EgressBindings{
		name:        name,
		Selector:    selector,
		egressPorts: port,
		resolver:    resolver,
		namespace:   namespace,
	}
	return err
}

func (sb *ServiceBindings) removeSelectorTarget(selector string) {
	sb.targets[selector].stop()
	delete(sb.targets, selector)
}

func (sb *ServiceBindings) addServiceTarget(name string, service string, port map[int]int, namespace string, tlsCredentials string) error {
	sb.targets[service] = &EgressBindings{
		name:           name,
		service:        service,
		egressPorts:    port,
		resolver:       NewNullTargetResolver([]string{service}),
		tlsCredentials: tlsCredentials,
		namespace:      namespace,
	}
	return nil
}

func (sb *ServiceBindings) removeServiceTarget(service string) {
	delete(sb.targets, service)
}

func (sb *ServiceBindings) Stop() {
	for _, v := range sb.targets {
		if v != nil {
			v.stop()
		}
	}
}

func (sb *ServiceBindings) updateBridgeConfiguration(siteId string, bridges *qdr.BridgeConfig) {
	if sb.headless == nil && !sb.RequiresExternalBridge() {
		addIngressBridge(sb, siteId, bridges)
		for _, eb := range sb.targets {
			eb.updateBridgeConfiguration(sb, siteId, bridges)
		}
	} // headless proxies are not specified through the main bridge configuration
}

func (eb *EgressBindings) stop() {
	eb.resolver.Close()
}

func (eb *EgressBindings) hasLocalTarget() bool {
	return eb.resolver.HasTarget()
}

func (eb *EgressBindings) updateBridgeConfiguration(sb *ServiceBindings, siteId string, bridges *qdr.BridgeConfig) {
	for _, target := range eb.resolver.List() {
		addEgressBridge(sb.protocol, target, eb.egressPorts, sb.Address, eb.name, siteId, eb.service, sb.aggregation, sb.eventChannel, sb.TlsCertAuthority, bridges)
	}
}

func (target *EgressBindings) GetLocalTargetPorts(desired *ServiceBindings) map[int]int {
	ports := map[int]int{}
	for i := 0; i < len(desired.publicPorts); i++ {
		publicPort := desired.publicPorts[i]
		ports[publicPort] = target.egressPorts[publicPort]
	}
	return ports
}

func newBridgeConfiguration() *qdr.BridgeConfig {
	v := qdr.NewBridgeConfig()
	return &v
}

const (
	ProtocolTCP   string = "tcp"
	ProtocolHTTP  string = "http"
	ProtocolHTTP2 string = "http2"
)

func addEgressBridge(protocol string, host string, port map[int]int, address string, target string, siteId string, hostOverride string, aggregation string, eventchannel bool, tlsCertAuthority string, bridges *qdr.BridgeConfig) (bool, error) {
	if host == "" {
		return false, fmt.Errorf("Cannot add connector without host (%s %s)", address, protocol)
	}
	for sPort, tPort := range port {
		endpointName := getBridgeName(address+"."+target, host, sPort, tPort)
		endpointAddr := fmt.Sprintf("%s:%d", address, sPort)
		switch protocol {
		case ProtocolHTTP:
			b := qdr.HttpEndpoint{
				Name:    endpointName,
				Host:    host,
				Port:    strconv.Itoa(tPort),
				Address: endpointAddr,
				SiteId:  siteId,
			}
			if aggregation != "" {
				b.Aggregation = aggregation
				b.Address = "mc/" + endpointAddr
			}
			if eventchannel {
				b.EventChannel = eventchannel
				b.Address = "mc/" + endpointAddr
			}
			if hostOverride != "" {
				b.HostOverride = hostOverride
			}
			bridges.AddHttpConnector(b)
		case ProtocolHTTP2:
			httpConnector := qdr.HttpEndpoint{
				Name:            endpointName,
				Host:            host,
				Port:            strconv.Itoa(tPort),
				Address:         endpointAddr,
				SiteId:          siteId,
				ProtocolVersion: qdr.HttpVersion2,
			}

			if len(tlsCertAuthority) > 0 {
				verifyHostName := new(bool)
				*verifyHostName = false
				httpConnector.SslProfile = tlsCertAuthority
				httpConnector.VerifyHostname = verifyHostName
			}
			bridges.AddHttpConnector(httpConnector)
		case ProtocolTCP:
			tcpConnector := qdr.TcpEndpoint{
				Name:    endpointName,
				Host:    host,
				Port:    strconv.Itoa(tPort),
				Address: endpointAddr,
				SiteId:  siteId,
			}

			if len(tlsCertAuthority) > 0 {
				verifyHostName := new(bool)
				*verifyHostName = false
				tcpConnector.SslProfile = tlsCertAuthority
				tcpConnector.VerifyHostname = verifyHostName
			}

			bridges.AddTcpConnector(tcpConnector)

		default:
			return false, fmt.Errorf("Unrecognised protocol for service %s: %s", address, protocol)
		}
	}
	return true, nil
}

func addIngressBridge(sb *ServiceBindings, siteId string, bridges *qdr.BridgeConfig) (bool, error) {
	for i := 0; i < len(sb.publicPorts); i++ {
		pPort := sb.publicPorts[i]
		iPort := sb.ingressPorts[i]
		endpointName := getBridgeName(sb.Address, "", pPort)
		endpointAddr := fmt.Sprintf("%s:%d", sb.Address, pPort)

		switch sb.protocol {
		case ProtocolHTTP:
			if sb.aggregation != "" || sb.eventChannel {
				endpointAddr = "mc/" + endpointAddr
			}
			bridges.AddHttpListener(qdr.HttpEndpoint{
				Name:         endpointName,
				Port:         strconv.Itoa(iPort),
				Address:      endpointAddr,
				SiteId:       siteId,
				Aggregation:  sb.aggregation,
				EventChannel: sb.eventChannel,
			})

		case ProtocolHTTP2:
			httpListener := qdr.HttpEndpoint{
				Name:            endpointName,
				Port:            strconv.Itoa(iPort),
				Address:         endpointAddr,
				SiteId:          siteId,
				Aggregation:     sb.aggregation,
				EventChannel:    sb.eventChannel,
				ProtocolVersion: qdr.HttpVersion2,
			}

			if len(sb.TlsCredentials) > 0 {
				httpListener.SslProfile = sb.TlsCredentials
			}

			bridges.AddHttpListener(httpListener)
		case ProtocolTCP:
			tcpListener := qdr.TcpEndpoint{
				Name:    endpointName,
				Port:    strconv.Itoa(iPort),
				Address: endpointAddr,
				SiteId:  siteId,
			}

			if len(sb.TlsCredentials) > 0 {
				tcpListener.SslProfile = sb.TlsCredentials
			}

			bridges.AddTcpListener(tcpListener)

		default:
			return false, fmt.Errorf("Unrecognised protocol for service %s: %s", sb.Address, sb.protocol)
		}
	}
	return true, nil
}

func RequiredBridges(services map[string]*ServiceBindings, siteId string) *qdr.BridgeConfig {
	bridges := newBridgeConfiguration()
	for _, service := range services {
		service.updateBridgeConfiguration(siteId, bridges)
	}
	return bridges
}
