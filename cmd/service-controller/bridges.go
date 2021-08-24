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

func getBridgeName(address string, host string) string {
	if host == "" {
		return address
	} else {
		return address + "@" + host
	}
}

type EgressBindings struct {
	name       string
	selector   string
	service    string
	egressPort int
	informer   cache.SharedIndexInformer
	stopper    chan struct{}
}

type ServiceBindings struct {
	origin       string
	protocol     string
	address      string
	publicPort   int
	ingressPort  int
	aggregation  string
	eventChannel bool
	headless     *types.Headless
	labels       map[string]string
	targets      map[string]*EgressBindings
}

func asServiceInterface(bindings *ServiceBindings) types.ServiceInterface {
	return types.ServiceInterface{
		Address:      bindings.address,
		Protocol:     bindings.protocol,
		Port:         bindings.publicPort,
		Aggregate:    bindings.aggregation,
		EventChannel: bindings.eventChannel,
		Headless:     bindings.headless,
		Labels:       bindings.labels,
		Origin:       bindings.origin,
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

func getTargetPort(service types.ServiceInterface, target types.ServiceInterfaceTarget) int {
	targetPort := target.TargetPort
	if targetPort == 0 {
		targetPort = service.Port
	}
	return targetPort
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

func (c *Controller) updateServiceBindings(required types.ServiceInterface, portAllocations map[string]int) error {
	bindings := c.bindings[required.Address]
	if bindings == nil {
		// create it
		var port int
		// headless services use distinct proxy pods, so don't need to allocate a port
		if required.Headless != nil {
			port = required.Port
		} else {
			if portAllocations != nil {
				// existing bridge configuration is used on initiaising map to recover
				// any previous port allocations
				port = portAllocations[required.Address]
			}
			if port == 0 {
				var err error
				port, err = c.ports.nextFreePort()
				if err != nil {
					return err
				}
			}
		}
		sb := newServiceBindings(required.Origin, required.Protocol, required.Address, required.Port, required.Headless, required.Labels, port, required.Aggregate, required.EventChannel)
		for _, t := range required.Targets {
			if t.Selector != "" {
				sb.addSelectorTarget(t.Name, t.Selector, getTargetPort(required, t), c)
			} else if t.Service != "" {
				sb.addServiceTarget(t.Name, t.Service, getTargetPort(required, t), c)
			}
		}
		c.bindings[required.Address] = sb
	} else {
		// check it is configured correctly
		if bindings.protocol != required.Protocol {
			bindings.protocol = required.Protocol
		}
		if bindings.publicPort != required.Port {
			bindings.publicPort = required.Port
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
				if bindings.headless.TargetPort != required.Headless.TargetPort {
					bindings.headless.TargetPort = required.Headless.TargetPort
				}
			}
			bindings.ingressPort = required.Port
		} else if bindings.headless != nil {
			bindings.headless = nil
		}

		hasSkupperSelector := false
		for _, t := range required.Targets {
			targetPort := getTargetPort(required, t)
			if strings.Contains(t.Selector, "skupper.io/component=router") {
				hasSkupperSelector = true
			}
			if t.Selector != "" {
				target := bindings.targets[t.Selector]
				if target == nil {
					bindings.addSelectorTarget(t.Name, t.Selector, targetPort, c)
				} else if target.egressPort != targetPort {
					target.egressPort = targetPort
				}
			} else if t.Service != "" {
				target := bindings.targets[t.Service]
				if target == nil {
					bindings.addServiceTarget(t.Name, t.Service, targetPort, c)
				} else if target.egressPort != targetPort {
					target.egressPort = targetPort
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

func newServiceBindings(origin string, protocol string, address string, publicPort int, headless *types.Headless, labels map[string]string, ingressPort int, aggregation string, eventChannel bool) *ServiceBindings {
	return &ServiceBindings{
		origin:       origin,
		protocol:     protocol,
		address:      address,
		publicPort:   publicPort,
		ingressPort:  ingressPort,
		aggregation:  aggregation,
		eventChannel: eventChannel,
		headless:     headless,
		labels:       labels,
		targets:      map[string]*EgressBindings{},
	}
}

func (sb *ServiceBindings) addSelectorTarget(name string, selector string, port int, controller *Controller) error {
	sb.targets[selector] = &EgressBindings{
		name:       name,
		selector:   selector,
		egressPort: port,
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

func (sb *ServiceBindings) addServiceTarget(name string, service string, port int, controller *Controller) error {
	sb.targets[service] = &EgressBindings{
		name:       name,
		service:    service,
		egressPort: port,
		stopper:    make(chan struct{}),
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
				addEgressBridge(sb.protocol, pod.Status.PodIP, eb.egressPort, sb.address, eb.name, siteId, "", sb.aggregation, sb.eventChannel, bridges)
			} else {
				event.Recordf(BridgeTargetEvent, "Pod for %s not ready/running: %s", sb.address, pod.ObjectMeta.Name)
			}
		}
	} else if eb.service != "" {
		addEgressBridge(sb.protocol, eb.service, eb.egressPort, sb.address, eb.name, siteId, eb.service, sb.aggregation, sb.eventChannel, bridges)
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

func addEgressBridge(protocol string, host string, port int, address string, target string, siteId string, hostOverride string, aggregation string, eventchannel bool, bridges *qdr.BridgeConfig) (bool, error) {
	if host == "" {
		return false, fmt.Errorf("Cannot add connector without host (%s %s)", address, protocol)
	}
	switch protocol {
	case ProtocolHTTP:
		b := qdr.HttpEndpoint{
			Name:    getBridgeName(address+"."+target, host),
			Host:    host,
			Port:    strconv.Itoa(port),
			Address: address,
			SiteId:  siteId,
		}
		if aggregation != "" {
			b.Aggregation = aggregation
			b.Address = "mc/" + b.Address
		}
		if eventchannel {
			b.EventChannel = eventchannel
			b.Address = "mc/" + b.Address
		}
		if hostOverride != "" {
			b.HostOverride = hostOverride
		}
		bridges.AddHttpConnector(b)
	case ProtocolHTTP2:
		bridges.AddHttpConnector(qdr.HttpEndpoint{
			Name:            getBridgeName(address+"."+target, host),
			Host:            host,
			Port:            strconv.Itoa(port),
			Address:         address,
			SiteId:          siteId,
			ProtocolVersion: qdr.HttpVersion2,
		})
	case ProtocolTCP:
		bridges.AddTcpConnector(qdr.TcpEndpoint{
			Name:    getBridgeName(address+"."+target, host),
			Host:    host,
			Port:    strconv.Itoa(port),
			Address: address,
			SiteId:  siteId,
		})
	default:
		return false, fmt.Errorf("Unrecognised protocol for service %s: %s", address, protocol)
	}
	return true, nil
}

func addIngressBridge(sb *ServiceBindings, siteId string, bridges *qdr.BridgeConfig) (bool, error) {
	switch sb.protocol {
	case ProtocolHTTP:
		address := sb.address
		if sb.aggregation != "" || sb.eventChannel {
			address = "mc/" + address
		}
		bridges.AddHttpListener(qdr.HttpEndpoint{
			Name:         getBridgeName(sb.address, ""),
			Host:         "0.0.0.0",
			Port:         strconv.Itoa(sb.ingressPort),
			Address:      address,
			SiteId:       siteId,
			Aggregation:  sb.aggregation,
			EventChannel: sb.eventChannel,
		})

	case ProtocolHTTP2:
		bridges.AddHttpListener(qdr.HttpEndpoint{
			Name:            getBridgeName(sb.address, ""),
			Host:            "0.0.0.0",
			Port:            strconv.Itoa(sb.ingressPort),
			Address:         sb.address,
			SiteId:          siteId,
			Aggregation:     sb.aggregation,
			EventChannel:    sb.eventChannel,
			ProtocolVersion: qdr.HttpVersion2,
		})
	case ProtocolTCP:
		bridges.AddTcpListener(qdr.TcpEndpoint{
			Name:    getBridgeName(sb.address, ""),
			Host:    "0.0.0.0",
			Port:    strconv.Itoa(sb.ingressPort),
			Address: sb.address,
			SiteId:  siteId,
		})
	default:
		return false, fmt.Errorf("Unrecognised protocol for service %s: %s", sb.address, sb.protocol)
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
