package site

import (
	"fmt"
	"log"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/pkg/qdr"
)

type Connector struct {
	Name            string
	RoutingKey      string `key:"routing-key" required:"true"`
	Selector        string `key:"selector"`
	Host            string `key:"host"`
	Port            int    `key:"port" required:"true"`
	Type            string `key:"type"`
	BridgeImage     string `key:"bridge-image"`
	TlsCredentials  string `key:"tls-credentials"`
	IncludeNotReady bool   `key:"include-not-ready"`
}

type Listener struct {
	Name            string
	RoutingKey      string `key:"routing-key" required:"true"`
	Host            string `key:"host" required:"true"`
	Port            int    `key:"port" required:"true"`
	Type            string `key:"type"`
	BridgeImage     string `key:"bridge-image"`
	TlsCredentials  string `key:"tls-credentials"`
}

type TargetSelection interface {
	List() []string
	Close()
}

type BindingContext interface {
	Select(name string, selector string, includeNotReady bool) TargetSelection
	Expose(ports *ExposedPortSet)
	Unexpose(host string)
}

type SelectedConnector struct {
	Spec      Connector
	Selection TargetSelection
}

func (sc *SelectedConnector) expand() []Connector {
	if sc.Selection == nil {
		return []Connector{sc.Spec}
	}
	hosts := sc.Selection.List()
	expanded := []Connector{}
	for _, host := range hosts {
		c := sc.Spec
		c.Host = host
		expanded = append(expanded, c)
	}
	return expanded
}


func (sc *SelectedConnector) init(name string, context BindingContext) bool {
	if sc.Selection != nil {
		sc.Selection.Close()
	}
	if sc.Spec.Selector != "" && context != nil {
		sc.Selection = context.Select(name, sc.Spec.Selector, sc.Spec.IncludeNotReady)
		return false
	}
	return true
}

func readConnector(cm *corev1.ConfigMap) (*Connector, error) {
	connector := &Connector{}
	if err := readFromConfigMap(connector, cm); err != nil {
		return nil, err
	}
	return connector, nil
}

func readListener(cm *corev1.ConfigMap) (*Listener, error) {
	listener := &Listener{}
	if err := readFromConfigMap(listener, cm); err != nil {
		return nil, err
	}
	return listener, nil
}

func readFromConfigMap(o interface{}, cm *corev1.ConfigMap) error {
	s := reflect.ValueOf(o).Elem()
	for i:= 0; i < s.NumField(); i++ {
		fieldInfo := s.Type().Field(i)
		if fieldInfo.Name == "Name" {
			s.Field(i).SetString(cm.ObjectMeta.Name)
			continue
		}
		key := fieldInfo.Tag.Get("key")
		if key == "" {
			key = strings.ToLower(fieldInfo.Name)
		}
		value, ok := cm.Data[key]
		required := fieldInfo.Tag.Get("required")
		if !ok && required == "true" {
			return fmt.Errorf("In ConfigMap %s, a value for %s is required", cm.ObjectMeta.Name, key)
		}
		field := s.Field(i)
		switch field.Kind() {
		case reflect.String:
			field.SetString(value)
		case reflect.Int:
			i, err := strconv.ParseInt(value, 10, 32)
			if err != nil {
				return fmt.Errorf("In ConfigMap %s, cannot convert value of %s into int: %q", cm.ObjectMeta.Name, key, value)
			}
			field.SetInt(i)
		case reflect.Bool:
			if value != "" {
				b, err := strconv.ParseBool(value)
				if err != nil {
					return fmt.Errorf("In ConfigMap %s, cannot convert value of %s into bool: %q", cm.ObjectMeta.Name, key, value)
				}
				field.SetBool(b)
			}
		}
	}
	return nil
}
type Port struct {
	Name       string
	Port       int
	TargetPort int
	Protocol   corev1.Protocol
}

type ExposedPortSet struct {
	Host  string
	Ports map[string]Port
}

func (p *ExposedPortSet) add(port Port) bool {
	if existing, ok := p.Ports[port.Name]; !ok || existing != port {
		p.Ports[port.Name] = port
		return true
	}
	return false
}

func (p *ExposedPortSet) remove(portname string) bool {
	if _, ok := p.Ports[portname]; ok {
		delete(p.Ports, portname)
		return true
	}
	return false
}

type ExposedPorts map[string]*ExposedPortSet

func (p ExposedPorts) Expose(host string, port Port) *ExposedPortSet {
	if existing, ok := p[host]; ok {
		if existing.add(port) {
			return existing
		} else {
			//no change was needed
			return nil
		}
	} else {
		portset := &ExposedPortSet{
			Host: host,
			Ports: map[string]Port{
				port.Name: port,
			},
		}
		p[host] = portset
		return portset
	}
}

func (p ExposedPorts) Unexpose(host string, portname string) *ExposedPortSet {
	if existing, ok := p[host]; ok && existing.remove(portname) {
		return existing
	}
	//no change was required
	return nil
}

type Bindings struct {
	SiteId     string
	context    BindingContext
	mapping    *qdr.PortMapping
	connectors map[string]*SelectedConnector
	listeners  map[string]*Listener
	exposed    ExposedPorts
}

func NewBindings() *Bindings {
	return &Bindings{
		connectors: map[string]*SelectedConnector{},
		listeners:  map[string]*Listener{},
		exposed:    ExposedPorts{},
	}
}

func (b *Bindings) SetBindingContext(context BindingContext) {
	b.context = context
	for name, sc := range b.connectors {
		sc.init(name, context)
	}
	for name, l := range b.listeners {
		b.expose(name, l)
	}
}

func (b *Bindings) CloseAllSelectedConnectors() {
	for _, c := range b.connectors {
		if c.Selection != nil {
			c.Selection.Close()
		}
	}
}

func (b *Bindings) UpdateConnector(name string, cm *corev1.ConfigMap) (qdr.ConfigUpdate, error) {
	if cm == nil {
		return b.deleteConnector(name), nil
	}
	return b.updateConnector(cm)
}


func (b *Bindings) updateConnector(cm *corev1.ConfigMap) (qdr.ConfigUpdate, error) {
	latest, err := readConnector(cm)
	if err != nil {
		return nil, err
	}
	name := cm.ObjectMeta.Name
	if sc, ok := b.connectors[name]; !ok || sc.Spec != *latest {
		if !ok {
			sc = &SelectedConnector{}
			b.connectors[name] = sc
		}
		sc.Spec = *latest
		if sc.init(name, b.context) {
			return b, nil
		}
	}
	return nil, nil
}

func (b *Bindings) deleteConnector(name string) qdr.ConfigUpdate {
	if existing, ok := b.connectors[name]; ok {
		if existing.Selection != nil {
			existing.Selection.Close()
		}
		delete(b.connectors, name)
		return b
	}
	return nil
}

func (b *Bindings) UpdateListener(name string, cm *corev1.ConfigMap) (qdr.ConfigUpdate, error) {
	if cm == nil {
		return b.deleteListener(name), nil
	}
	return b.updateListener(cm)
}

func (b *Bindings) updateListener(cm *corev1.ConfigMap) (qdr.ConfigUpdate, error) {
	latest, err := readListener(cm)
	if err != nil {
		return nil, err
	}
	name := cm.ObjectMeta.Name
	if existing, ok := b.listeners[name]; !ok || existing != latest {
		b.listeners[name] = latest
		b.expose(name, latest)
		return b, nil
	}
	return nil, nil
}

func (b *Bindings) deleteListener(name string) qdr.ConfigUpdate {
	if _, ok := b.listeners[name]; ok {
		delete(b.listeners, name)
		if b.context != nil {
			b.context.Unexpose(name)
		}
		return b
	}
	return nil
}

func (b *Bindings) ToBridgeConfig(mapping *qdr.PortMapping) qdr.BridgeConfig {
	config := qdr.BridgeConfig {
		TcpListeners:   qdr.TcpEndpointMap{},
		TcpConnectors:  qdr.TcpEndpointMap{},
		HttpListeners:  qdr.HttpEndpointMap{},
		HttpConnectors: qdr.HttpEndpointMap{},
	}
	for key, sc := range b.connectors {
		for _, c := range sc.expand() {
			name := key + "_" + c.Host
			if c.Type == "http" {
				config.HttpConnectors[name] = c.AsHttpEndpoint(name, b.SiteId)
			} else if c.Type == "http2" {
				config.HttpConnectors[name] = c.AsHttp2Endpoint(name, b.SiteId)
			} else if c.Type == "tcp" || c.Type == "" {
				config.TcpConnectors[name] = c.AsTcpEndpoint(name, b.SiteId)
			}
		}
	}
	for key, l := range b.listeners {
		if l.Type == "http" {
			config.HttpListeners[key] = l.AsHttpEndpoint(key, b.SiteId, mapping)
		} else if l.Type == "http2" {
			config.HttpListeners[key] = l.AsHttp2Endpoint(key, b.SiteId, mapping)
		} else if l.Type == "tcp" || l.Type == "" {
			config.TcpListeners[key] = l.AsTcpEndpoint(key, b.SiteId, mapping)
		}
	}

	return config
}

func (b *Bindings) RecoverPortMapping(config *qdr.RouterConfig) {
	if b.mapping == nil {
		b.mapping = qdr.RecoverPortMapping(config)
	}
}

func (b *Bindings) Apply(config *qdr.RouterConfig) bool {
	config.UpdateBridgeConfig(b.ToBridgeConfig(b.mapping))
	return true //TODO: can optimise by indicating if no change was required
}

func (b *Bindings) expose(name string, l *Listener)  {
	if b.mapping != nil {
		allocatedRouterPort, err := b.mapping.GetPortForAddress(l.RoutingKey)
		if err != nil {
			log.Printf("Unable to get port for listener %q: %s", name, err)
		} else {
			port := Port {
				Name:       name,
				Port:       l.Port,
				TargetPort: allocatedRouterPort,
				Protocol:   l.protocol(),
			}
			exposed := b.exposed.Expose(l.Host, port)
			if exposed != nil && b.context != nil{
				b.context.Expose(exposed)
			}
		}
	}
}

func (b *Bindings) unexpose(name string, l *Listener)  {
	exposed := b.exposed.Unexpose(l.Host, name)
	if exposed != nil && b.context != nil {
		if len(exposed.Ports) > 0 {
			b.context.Expose(exposed)
		} else {
			b.context.Unexpose(exposed.Host)
		}
	}
}

func (c *Connector) AsTcpEndpoint(name string, siteId string) qdr.TcpEndpoint {
	return qdr.TcpEndpoint {
		Name:    name,
		Host:    c.Host,
		Port:    strconv.Itoa(c.Port),
		Address: c.RoutingKey,
		SiteId:  siteId,
		//TODO:
		//SslProfile
		//VerifyHostname
	}
}

func (c *Connector) AsHttpEndpoint(name string, siteId string) qdr.HttpEndpoint {
	return qdr.HttpEndpoint {
		Name:            name,
		Host:            c.Host,
		Port:            strconv.Itoa(c.Port), //TODO: should port be a string to allow for wll known service names in binding definitions?
		Address:         c.RoutingKey,
		SiteId:          siteId,
	        //TODO:
	        //Aggregation
	        //EventChannel
	        //HostOverride
		//SslProfile
		//VerifyHostname
	}
}

func (c *Connector) AsHttp2Endpoint(name string, siteId string) qdr.HttpEndpoint {
	endpoint := c.AsHttpEndpoint(name, siteId)
	endpoint.ProtocolVersion = qdr.HttpVersion2
	return endpoint
}

func (l *Listener) AsTcpEndpoint(name string, siteId string, mapping *qdr.PortMapping) qdr.TcpEndpoint {
	port, err := mapping.GetPortForAddress(l.RoutingKey)
	if err != nil {
		log.Printf("Could not allocate port for %s: %s", name, err)
	}
	return qdr.TcpEndpoint {
		Name:    name,
		Host:    "0.0.0.0",
		Port:    strconv.Itoa(port),
		Address: l.RoutingKey,
		SiteId:  siteId,
		//TODO:
		//SslProfile
		//VerifyHostname
	}
}

func (l *Listener) AsHttpEndpoint(name string, siteId string, mapping *qdr.PortMapping) qdr.HttpEndpoint {
	port, err := mapping.GetPortForAddress(l.RoutingKey)
	if err != nil {
		log.Printf("Could not allocate port for %s: %s", name, err)
	}
	return qdr.HttpEndpoint {
		Name:            name,
		Host:            "0.0.0.0",
		Port:            strconv.Itoa(port), //TODO: should port be a string to allow for wll known service names in binding definitions?
		Address:         l.RoutingKey,
		SiteId:          siteId,
	        //TODO:
	        //Aggregation
	        //EventChannel
	        //HostOverride
		//SslProfile
		//VerifyHostname
	}
}

func (l *Listener) AsHttp2Endpoint(name string, siteId string, mapping *qdr.PortMapping) qdr.HttpEndpoint {
	endpoint := l.AsHttpEndpoint(name, siteId, mapping)
	endpoint.ProtocolVersion = qdr.HttpVersion2
	return endpoint
}

func (l *Listener) protocol() corev1.Protocol {
	if l.Type == "udp" {
		return corev1.ProtocolUDP
	}
	return corev1.ProtocolTCP
}
