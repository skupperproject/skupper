package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
)

type Bridge struct {
	Name    string
	Host    string
	Port    int
	Address string
	SiteId  string
}

func getBridgeName(address string, host string) string {
	if host == "" {
		return address
	} else {
		return address + "@" + host
	}
}

func (b *Bridge) checkName() string {
	if b.Name == "" {
		b.Name = getBridgeName(b.Address, b.Host)
	}
	return b.Name
}

func (b *Bridge) toMap() map[string]interface{} {
	return map[string]interface{}{
		"name":    b.Name,
		"host":    b.Host,
		"port":    b.Port,
		"address": b.Address,
		"siteId":  b.SiteId,
	}
}

func (a Bridge) equivalent(b Bridge) bool {
	return a.Host == b.Host && a.Port == b.Port && a.Address == b.Address && a.SiteId == b.SiteId
}

type HttpBridge struct {
	Bridge
	Http2        bool
	Aggregation  string
	EventChannel bool
}

func (b *HttpBridge) toMap() map[string]interface{} {
	return map[string]interface{}{
		"name":         b.Name,
		"host":         b.Host,
		"port":         b.Port,
		"address":      b.Address,
		"siteId":       b.SiteId,
		"http2":        b.Http2,
		"aggregation":  b.Aggregation,
		"eventChannel": b.EventChannel,
	}
}

func (a HttpBridge) equivalent(b HttpBridge) bool {
	return a.Host == b.Host && a.Port == b.Port && a.Address == b.Address && a.SiteId == b.SiteId && a.Http2 == b.Http2 && a.Aggregation == b.Aggregation && a.EventChannel == b.EventChannel
}

type BridgeMap map[string]Bridge
type HttpBridgeMap map[string]HttpBridge
type NestedBridgeMap map[string]BridgeMap
type NestedHttpBridgeMap map[string]HttpBridgeMap

type BridgeConfiguration struct {
	HttpConnectors  NestedHttpBridgeMap
	HttpListeners   HttpBridgeMap
	TcpConnectors   NestedBridgeMap
	TcpListeners    BridgeMap
	Http2Connectors NestedHttpBridgeMap
	Http2Listeners  HttpBridgeMap
}

type EgressBindings struct {
	selector   string
	egressPort int
	informer   cache.SharedIndexInformer
	stopper    chan struct{}
}

type ServiceBindings struct {
	origin      string
	protocol    string
	address     string
	publicPort  int
	ingressPort int
	headless    *types.Headless
	targets     map[string]*EgressBindings
}

func asServiceInterface(bindings *ServiceBindings) types.ServiceInterface {
	return types.ServiceInterface{
		Address:  bindings.address,
		Protocol: bindings.protocol,
		Port:     bindings.publicPort,
		Headless: bindings.headless,
		Origin:   bindings.origin,
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

func (c *Controller) updateServiceBindings(required types.ServiceInterface, portAllocations map[string]int) error {
	bindings := c.bindings[required.Address]
	if bindings == nil {
		//create it
		var port int
		if portAllocations != nil {
			//existing bridge configuration is used on initiaising map to recover
			//any previous port allocations
			port = portAllocations[required.Address]
		}
		if port == 0 {
			var err error
			port, err = c.ports.nextFreePort()
			if err != nil {
				return err
			}
		}
		sb := newServiceBindings(required.Origin, required.Protocol, required.Address, required.Port, required.Headless, port)
		for _, t := range required.Targets {
			sb.addTarget(t.Selector, getTargetPort(required, t), c)
		}
		c.bindings[required.Address] = sb
	} else {
		//check it is configured correctly
		if bindings.protocol != required.Protocol {
			bindings.protocol = required.Protocol
		}
		if bindings.publicPort != required.Port {
			bindings.publicPort = required.Port
		}
		if required.Headless != nil {
			if bindings.headless == nil {
				bindings.headless = required.Headless
			} else if bindings.headless.Name != required.Headless.Name {
				bindings.headless.Name = required.Headless.Name
			} else if bindings.headless.Size != required.Headless.Size {
				bindings.headless.Size = required.Headless.Size
			} else if bindings.headless.TargetPort != required.Headless.TargetPort {
				bindings.headless.TargetPort = required.Headless.TargetPort
			}
		} else if bindings.headless != nil {
			bindings.headless = nil
		}
		for _, t := range required.Targets {
			targetPort := getTargetPort(required, t)
			target := bindings.targets[t.Selector]
			if target == nil {
				bindings.addTarget(t.Selector, targetPort, c)
			} else if target.egressPort != targetPort {
				target.egressPort = targetPort
			}
		}
		for k, _ := range bindings.targets {
			if !hasTargetForSelector(required, k) {
				bindings.removeTarget(k)
			}
		}
	}
	return nil
}

func newServiceBindings(origin string, protocol string, address string, publicPort int, headless *types.Headless, ingressPort int) *ServiceBindings {
	return &ServiceBindings{
		origin:      origin,
		protocol:    protocol,
		address:     address,
		publicPort:  publicPort,
		ingressPort: ingressPort,
		headless:    headless,
		targets:     map[string]*EgressBindings{},
	}
}

func (sb *ServiceBindings) addTarget(selector string, port int, controller *Controller) error {
	sb.targets[selector] = &EgressBindings{
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

func (sb *ServiceBindings) removeTarget(selector string) {
	sb.targets[selector].stop()
	delete(sb.targets, selector)
}

func (sb *ServiceBindings) stop() {
	for _, v := range sb.targets {
		if v != nil {
			v.stop()
		}
	}
}

func (sb *ServiceBindings) updateBridgeConfiguration(siteId string, bridges *BridgeConfiguration) {
	if sb.headless == nil {
		addIngressBridge(sb.protocol, sb.ingressPort, sb.address, siteId, bridges)
		for _, eb := range sb.targets {
			eb.updateBridgeConfiguration(sb.protocol, sb.address, siteId, bridges)
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

func (eb *EgressBindings) updateBridgeConfiguration(protocol string, address string, siteId string, bridges *BridgeConfiguration) {
	pods := eb.informer.GetStore().List()
	for _, p := range pods {
		pod := p.(*corev1.Pod)
		log.Printf("Adding pod for %s: %s", address, pod.ObjectMeta.Name)
		addEgressBridge(protocol, pod.Status.PodIP, eb.egressPort, address, siteId, bridges)
	}
}

func newBridgeConfiguration() *BridgeConfiguration {
	return &BridgeConfiguration{
		HttpConnectors:  make(map[string]HttpBridgeMap),
		HttpListeners:   make(map[string]HttpBridge),
		Http2Connectors: make(map[string]HttpBridgeMap),
		Http2Listeners:  make(map[string]HttpBridge),
		TcpConnectors:   make(map[string]BridgeMap),
		TcpListeners:    make(map[string]Bridge),
	}
}

func getStringByKey(attributes map[string]interface{}, key string) string {
	s, ok := attributes[key].(string)
	if !ok {
		return ""
	}
	return s
}

func getIntByKey(attributes map[string]interface{}, key string) int {
	i, ok := attributes[key].(int)
	if !ok {
		return 0
	}
	return i
}

func getBoolByKey(attributes map[string]interface{}, key string) bool {
	if b, ok := attributes[key].(bool); ok {
		return b
	} else {
		return false
	}
}

func (bc *BridgeConfiguration) addBridgeFromMap(entityType string, attributes map[string]interface{}) {
	if isHttpBridgeEntity(entityType) {
		bridge := HttpBridge{
			Bridge: Bridge{
				Name:    getStringByKey(attributes, "name"),
				Host:    getStringByKey(attributes, "host"),
				Port:    getIntByKey(attributes, "port"),
				Address: getStringByKey(attributes, "address"),
				SiteId:  getStringByKey(attributes, "siteId"),
			},
			Http2:        getBoolByKey(attributes, "http2"),
			Aggregation:  getStringByKey(attributes, "aggregation"),
			EventChannel: getBoolByKey(attributes, "eventChannel"),
		}
		bridge.checkName()
		switch entityType {
		case "httpConnector":
			bc.HttpConnectors.add(bridge)
		case "httpListener":
			bc.HttpListeners[bridge.Address] = bridge
		case "http2Connector":
			bc.Http2Connectors.add(bridge)
		case "http2Listener":
			bc.Http2Listeners[bridge.Address] = bridge
		default:
		}
	} else {
		bridge := Bridge{
			Name:    getStringByKey(attributes, "name"),
			Host:    getStringByKey(attributes, "host"),
			Port:    getIntByKey(attributes, "port"),
			Address: getStringByKey(attributes, "address"),
			SiteId:  getStringByKey(attributes, "siteId"),
		}
		bridge.checkName()
		switch entityType {
		case "tcpConnector":
			bc.TcpConnectors.add(bridge)
		case "tcpListener":
			bc.TcpListeners[bridge.Address] = bridge
		default:
		}
	}
}

func isBridgeEntity(typename string) bool {
	switch typename {
	case
		"tcpListener",
		"tcpConnector":
		return true
	}
	return isHttpBridgeEntity(typename)
}

func isHttpBridgeEntity(typename string) bool {
	switch typename {
	case
		"httpListener",
		"httpConnector",
		"http2Listener",
		"http2Connector":
		return true
	}
	return false
}

const (
	ProtocolTCP   string = "tcp"
	ProtocolHTTP  string = "http"
	ProtocolHTTP2 string = "http2"
)

func (m NestedBridgeMap) add(b Bridge) {
	nm := m[b.Address]
	if nm == nil {
		m[b.Address] = BridgeMap{
			b.Host: b,
		}
	} else {
		nm[b.Host] = b
	}
}

func (m NestedHttpBridgeMap) add(b HttpBridge) {
	nm := m[b.Address]
	if nm == nil {
		m[b.Address] = HttpBridgeMap{
			b.Host: b,
		}
	} else {
		nm[b.Host] = b
	}
}

func addEgressBridge(protocol string, host string, port int, address string, siteId string, bridges *BridgeConfiguration) (bool, error) {
	switch protocol {
	case ProtocolHTTP:
		bridges.HttpConnectors.add(HttpBridge{
			Bridge: Bridge{
				Name:    getBridgeName(address, host),
				Host:    host,
				Port:    port,
				Address: address,
				SiteId:  siteId,
			},
		})
	case ProtocolHTTP2:
		bridges.Http2Connectors.add(HttpBridge{
			Bridge: Bridge{
				Name:    getBridgeName(address, host),
				Host:    host,
				Port:    port,
				Address: address,
				SiteId:  siteId,
			},
		})
	case ProtocolTCP:
		bridges.TcpConnectors.add(Bridge{
			Name:    getBridgeName(address, host),
			Host:    host,
			Port:    port,
			Address: address,
			SiteId:  siteId,
		})
	default:
		return false, fmt.Errorf("Unrecognised protocol for service %s: %s", address, protocol)
	}
	return true, nil
}

func addIngressBridge(protocol string, port int, address string, siteId string, bridges *BridgeConfiguration) (bool, error) {
	switch protocol {
	case ProtocolHTTP:
		bridges.HttpListeners[address] = HttpBridge{
			Bridge: Bridge{
				Name:    getBridgeName(address, ""),
				Host:    "0.0.0.0",
				Port:    port,
				Address: address,
				SiteId:  siteId,
			},
		}
	case ProtocolHTTP2:
		bridges.Http2Listeners[address] = HttpBridge{
			Bridge: Bridge{
				Name:    getBridgeName(address, ""),
				Host:    "0.0.0.0",
				Port:    port,
				Address: address,
				SiteId:  siteId,
			},
		}
	case ProtocolTCP:
		bridges.TcpListeners[address] = Bridge{
			Name:    getBridgeName(address, ""),
			Host:    "0.0.0.0",
			Port:    port,
			Address: address,
			SiteId:  siteId,
		}
	default:
		return false, fmt.Errorf("Unrecognised protocol for service %s: %s", address, protocol)
	}
	return true, nil
}

func requiredBridges(services map[string]*ServiceBindings, siteId string) *BridgeConfiguration {
	//TODO: headless services not yet handled
	//TODO: update for multicast when merged
	bridges := newBridgeConfiguration()
	for _, service := range services {
		service.updateBridgeConfiguration(siteId, bridges)
	}
	return bridges
}

func readBridgeConfiguration(data []byte) (*BridgeConfiguration, error) {
	bridges := newBridgeConfiguration()
	var obj interface{}
	json.Unmarshal(data, &obj)
	if obj == nil {
		return bridges, nil
	}
	elements, ok := obj.([]interface{})
	if !ok {
		return nil, fmt.Errorf("Invalid JSON for bridge configuration, expected array at top level got %#v", obj)
	}
	for _, e := range elements {
		element, ok := e.([]interface{})
		if !ok || len(element) != 2 {
			return nil, fmt.Errorf("Invalid JSON for bridge configuration, expected array with type and value got %#v", e)
		}
		entityType, ok := element[0].(string)
		if !ok {
			return nil, fmt.Errorf("Invalid JSON for bridge configuration, expected entity type as string got %#v", element[0])
		}
		if !isBridgeEntity(entityType) {
			return nil, fmt.Errorf("Invalid JSON for bridge configuration, unexpected entity type %s", entityType)
		}
		attributes, ok := element[1].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("Invalid JSON for bridge configuration, expected object got %#v", element[1])
		}
		bridges.addBridgeFromMap(entityType, attributes)
	}
	return bridges, nil
}

func writeBridgeConfiguration(bridges *BridgeConfiguration) ([]byte, error) {
	elements := []interface{}{}
	for _, m := range bridges.HttpConnectors {
		for _, b := range m {
			elements = append(elements, []interface{}{
				"httpConnector",
				b.toMap(),
			})
		}
	}
	for _, b := range bridges.HttpListeners {
		elements = append(elements, []interface{}{
			"httpListener",
			b.toMap(),
		})
	}
	for _, m := range bridges.Http2Connectors {
		for _, b := range m {
			elements = append(elements, []interface{}{
				"http2Connector",
				b.toMap(),
			})
		}
	}
	for _, b := range bridges.Http2Listeners {
		elements = append(elements, []interface{}{
			"http2Listener",
			b.toMap(),
		})
	}
	for _, m := range bridges.TcpConnectors {
		for _, b := range m {
			elements = append(elements, []interface{}{
				"tcpConnector",
				b.toMap(),
			})
		}
	}
	for _, b := range bridges.TcpListeners {
		elements = append(elements, []interface{}{
			"tcpListener",
			b.toMap(),
		})
	}
	return json.Marshal(elements)
}

func (a BridgeMap) equivalent(b BridgeMap) bool {
	for k, v := range a {
		if !v.equivalent(b[k]) {
			return false
		}
	}
	for k, v := range b {
		if !v.equivalent(a[k]) {
			return false
		}
	}
	return true
}

func (a HttpBridgeMap) equivalent(b HttpBridgeMap) bool {
	for k, v := range a {
		if !v.equivalent(b[k]) {
			return false
		}
	}
	for k, v := range b {
		if !v.equivalent(a[k]) {
			return false
		}
	}
	return true
}

func (a NestedBridgeMap) equivalent(b NestedBridgeMap) bool {
	for k, v := range a {
		if !v.equivalent(b[k]) {
			return false
		}
	}
	for k, v := range b {
		if !v.equivalent(a[k]) {
			return false
		}
	}
	return true
}

func (a NestedHttpBridgeMap) equivalent(b NestedHttpBridgeMap) bool {
	for k, v := range a {
		if !v.equivalent(b[k]) {
			return false
		}
	}
	for k, v := range b {
		if !v.equivalent(a[k]) {
			return false
		}
	}
	return true
}

func updateBridgeConfiguration(desired *BridgeConfiguration, actual *BridgeConfiguration) bool {
	if !desired.HttpConnectors.equivalent(actual.HttpConnectors) {
		return true
	}
	if !desired.HttpListeners.equivalent(actual.HttpListeners) {
		return true
	}
	if !desired.TcpConnectors.equivalent(actual.TcpConnectors) {
		return true
	}
	if !desired.TcpListeners.equivalent(actual.TcpListeners) {
		return true
	}
	if !desired.Http2Connectors.equivalent(actual.Http2Connectors) {
		return true
	}
	if !desired.Http2Listeners.equivalent(actual.Http2Listeners) {
		return true
	}
	return false
}
