package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
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

type EgressBindings struct {
	name           string
	selector       string
	service        string
	egressPorts    map[int]int
	informer       cache.SharedIndexInformer
	stopper        chan struct{}
	tlsCredentials string
}

type ServiceBindings struct {
	origin         string
	protocol       string
	address        string
	publicPorts    []int
	ingressPorts   []int
	aggregation    string
	eventChannel   bool
	headless       *types.Headless
	labels         map[string]string
	targets        map[string]*EgressBindings
	tlsCredentials string
}

func (s *ServiceBindings) PortMap() map[int]int {
	ports := map[int]int{}
	for i := 0; i < len(s.publicPorts); i++ {
		ports[s.publicPorts[i]] = s.ingressPorts[i]
	}
	return ports
}

func asServiceInterface(bindings *ServiceBindings) types.ServiceInterface {
	return types.ServiceInterface{
		Address:        bindings.address,
		Protocol:       bindings.protocol,
		Ports:          bindings.publicPorts,
		Aggregate:      bindings.aggregation,
		EventChannel:   bindings.eventChannel,
		Headless:       bindings.headless,
		Labels:         bindings.labels,
		Origin:         bindings.origin,
		TlsCredentials: bindings.tlsCredentials,
	}
}

type ServiceController struct {
	bindings map[string]*ServiceBindings
	ports    *FreePorts
}

func newServiceController() *ServiceController {
	return &ServiceController{
		bindings: map[string]*ServiceBindings{},
		ports:    newFreePorts(),
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

func (c *Controller) updateServiceBindings(required types.ServiceInterface, portAllocations map[string][]int) error {
	res := c.policy.ValidateImportService(required.Address)
	bindings := c.bindings[required.Address]
	if bindings == nil {
		if !res.Allowed() {
			event.Recordf(BridgeTargetEvent, "Policy validation error: service %s cannot be created", required.Address)
			return nil
		}
		// create it
		var ports []int
		// headless services use distinct proxy pods, so don't need to allocate a port
		if required.Headless != nil {
			ports = required.Ports
		} else {
			if portAllocations != nil {
				// existing bridge configuration is used on initialising map to recover
				// any previous port allocations
				ports = portAllocations[required.Address]
			}
			if len(ports) == 0 {
				for i := 0; i < len(required.Ports); i++ {
					port, err := c.ports.nextFreePort()
					if err != nil {
						return err
					}
					ports = append(ports, port)
				}
			}
		}
		sb := newServiceBindings(required.Origin, required.Protocol, required.Address, required.Ports, required.Headless, required.Labels, ports, required.Aggregate, required.EventChannel, required.TlsCredentials)
		for _, t := range required.Targets {
			if t.Selector != "" {
				sb.addSelectorTarget(t.Name, t.Selector, getTargetPorts(required, t), c)
			} else if t.Service != "" {
				sb.addServiceTarget(t.Name, t.Service, getTargetPorts(required, t), required.TlsCredentials, c)
			}
		}

		if len(required.TlsCredentials) > 0 {
			sb.tlsCredentials = required.TlsCredentials
		}

		c.bindings[required.Address] = sb

	} else {
		if !res.Allowed() {
			event.Recordf(BridgeTargetEvent, "Policy validation error: service %s has been removed", required.Address)
			delete(c.bindings, required.Address)
			return nil
		}
		// check it is configured correctly
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

		if bindings.tlsCredentials != required.TlsCredentials {

			// Credentials will be overridden only if there are no value for them,
			// and in that case a new secret has to be generated in that site.
			if len(bindings.tlsCredentials) == 0 {
				bindings.tlsCredentials = types.SkupperServiceCertPrefix + required.Address
			}
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
					bindings.addSelectorTarget(t.Name, t.Selector, targetPort, c)
				} else if !reflect.DeepEqual(target.egressPorts, targetPort) {
					target.egressPorts = targetPort
				}
			} else if t.Service != "" {
				target := bindings.targets[t.Service]
				if target == nil {
					bindings.addServiceTarget(t.Name, t.Service, targetPort, required.TlsCredentials, c)
				} else if !reflect.DeepEqual(target.egressPorts, targetPort) {
					target.egressPorts = targetPort
				}
			}
		}
		for k, v := range bindings.targets {
			if v.selector != "" {
				if !hasTargetForSelector(required, k) && !hasSkupperSelector {
					bindings.removeSelectorTarget(k)
				}
			} else if v.service != "" {
				if !hasTargetForService(required, k) {
					bindings.removeServiceTarget(k)
				}
			}
		}
		if !reflect.DeepEqual(bindings.labels, required.Labels) {
			if bindings.labels == nil {
				bindings.labels = map[string]string{}
			} else if len(required.Labels) == 0 {
				bindings.labels = nil
			}
			for k, v := range required.Labels {
				bindings.labels[k] = v
			}
		}
	}
	return nil
}

func newServiceBindings(origin string, protocol string, address string, publicPorts []int, headless *types.Headless, labels map[string]string, ingressPorts []int, aggregation string, eventChannel bool, tlsCredentials string) *ServiceBindings {
	return &ServiceBindings{
		origin:         origin,
		protocol:       protocol,
		address:        address,
		publicPorts:    publicPorts,
		ingressPorts:   ingressPorts,
		aggregation:    aggregation,
		eventChannel:   eventChannel,
		headless:       headless,
		labels:         labels,
		targets:        map[string]*EgressBindings{},
		tlsCredentials: tlsCredentials,
	}
}

func (sb *ServiceBindings) addSelectorTarget(name string, selector string, port map[int]int, controller *Controller) error {
	sb.targets[selector] = &EgressBindings{
		name:        name,
		selector:    selector,
		egressPorts: port,
		informer: corev1informer.NewFilteredPodInformer(
			controller.vanClient.KubeClient,
			controller.vanClient.Namespace,
			time.Second*30,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
				options.LabelSelector = selector
			})),
		stopper: make(chan struct{}),
	}
	sb.targets[selector].informer.AddEventHandler(controller.newEventHandler("targetpods@"+sb.address, FixedKey, PodResourceVersionTest))
	return sb.targets[selector].start()
}

func (sb *ServiceBindings) removeSelectorTarget(selector string) {
	sb.targets[selector].stop()
	delete(sb.targets, selector)
}

func (sb *ServiceBindings) addServiceTarget(name string, service string, port map[int]int, tlsCredentials string, controller *Controller) error {
	sb.targets[service] = &EgressBindings{
		name:           name,
		service:        service,
		egressPorts:    port,
		stopper:        make(chan struct{}),
		tlsCredentials: tlsCredentials,
	}
	return nil
}

func (sb *ServiceBindings) removeServiceTarget(service string) {
	delete(sb.targets, service)
}

func (sb *ServiceBindings) stop() {
	for _, v := range sb.targets {
		if v != nil {
			v.stop()
		}
	}
}

func (sb *ServiceBindings) updateBridgeConfiguration(siteId string, bridges *qdr.BridgeConfig) {
	if sb.headless == nil {
		addIngressBridge(sb, siteId, bridges)
		for _, eb := range sb.targets {
			eb.updateBridgeConfiguration(sb, siteId, bridges)
		}
	} // headless proxies are not specified through the main bridge configuration
}

func (eb *EgressBindings) start() error {
	go eb.informer.Run(eb.stopper)
	if ok := cache.WaitForCacheSync(eb.stopper, eb.informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for service targetcache to sync")
	}
	return nil
}

func (eb *EgressBindings) stop() {
	close(eb.stopper)
}

const (
	BridgeTargetEvent string = "BridgeTargetEvent"
)

func (eb *EgressBindings) updateBridgeConfiguration(sb *ServiceBindings, siteId string, bridges *qdr.BridgeConfig) {
	if eb.selector != "" {
		pods := eb.informer.GetStore().List()
		for _, p := range pods {
			pod := p.(*corev1.Pod)
			if kube.IsPodRunning(pod) && kube.IsPodReady(pod) && pod.DeletionTimestamp == nil {
				event.Recordf(BridgeTargetEvent, "Adding pod for %s: %s", sb.address, pod.ObjectMeta.Name)
				addEgressBridge(sb.protocol, pod.Status.PodIP, eb.egressPorts, sb.address, eb.name, siteId, "", sb.aggregation, sb.eventChannel, sb.tlsCredentials, bridges)
			} else {
				event.Recordf(BridgeTargetEvent, "Pod for %s not ready/running: %s", sb.address, pod.ObjectMeta.Name)
			}
		}
	} else if eb.service != "" {
		addEgressBridge(sb.protocol, eb.service, eb.egressPorts, sb.address, eb.name, siteId, eb.service, sb.aggregation, sb.eventChannel, sb.tlsCredentials, bridges)
	}
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

func addEgressBridge(protocol string, host string, port map[int]int, address string, target string, siteId string, hostOverride string, aggregation string, eventchannel bool, tlsCredentials string, bridges *qdr.BridgeConfig) (bool, error) {
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

			if len(tlsCredentials) > 0 {
				verifyHostName := new(bool)
				*verifyHostName = false
				httpConnector.SslProfile = types.ServiceClientSecret
				httpConnector.VerifyHostname = verifyHostName
			}
			bridges.AddHttpConnector(httpConnector)
		case ProtocolTCP:
			bridges.AddTcpConnector(qdr.TcpEndpoint{
				Name:    endpointName,
				Host:    host,
				Port:    strconv.Itoa(tPort),
				Address: endpointAddr,
				SiteId:  siteId,
			})
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
		endpointName := getBridgeName(sb.address, "", pPort)
		endpointAddr := fmt.Sprintf("%s:%d", sb.address, pPort)

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

			if len(sb.tlsCredentials) > 0 {
				httpListener.SslProfile = sb.tlsCredentials
			}

			bridges.AddHttpListener(httpListener)
		case ProtocolTCP:
			bridges.AddTcpListener(qdr.TcpEndpoint{
				Name:    endpointName,
				Port:    strconv.Itoa(iPort),
				Address: endpointAddr,
				SiteId:  siteId,
			})
		default:
			return false, fmt.Errorf("Unrecognised protocol for service %s: %s", sb.address, sb.protocol)
		}
	}
	return true, nil
}

func requiredBridges(services map[string]*ServiceBindings, siteId string) *qdr.BridgeConfig {
	// TODO: headless services not yet handled
	// TODO: update for multicast when merged
	bridges := newBridgeConfiguration()
	for _, service := range services {
		service.updateBridgeConfiguration(siteId, bridges)
	}
	return bridges
}
