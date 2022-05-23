package qdr

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/api/types"
)

type RouterConfig struct {
	Metadata    RouterMetadata
	SslProfiles map[string]SslProfile
	Listeners   map[string]Listener
	Connectors  map[string]Connector
	Addresses   map[string]Address
	LogConfig   map[string]LogConfig
	Bridges     BridgeConfig
}

type TcpEndpointMap map[string]TcpEndpoint
type HttpEndpointMap map[string]HttpEndpoint

type BridgeConfig struct {
	TcpListeners   TcpEndpointMap
	TcpConnectors  TcpEndpointMap
	HttpListeners  HttpEndpointMap
	HttpConnectors HttpEndpointMap
}

func InitialConfig(id string, siteId string, version string, edge bool, helloAge int) RouterConfig {
	config := RouterConfig{
		Metadata: RouterMetadata{
			Id:                 id,
			HelloMaxAgeSeconds: strconv.Itoa(helloAge),
			Metadata:           getSiteMetadataString(siteId, version),
		},
		Addresses:   map[string]Address{},
		SslProfiles: map[string]SslProfile{},
		Listeners:   map[string]Listener{},
		Connectors:  map[string]Connector{},
		LogConfig:   map[string]LogConfig{},
		Bridges: BridgeConfig{
			TcpListeners:   map[string]TcpEndpoint{},
			TcpConnectors:  map[string]TcpEndpoint{},
			HttpListeners:  map[string]HttpEndpoint{},
			HttpConnectors: map[string]HttpEndpoint{},
		},
	}
	if edge {
		config.Metadata.Mode = ModeEdge
	} else {
		config.Metadata.Mode = ModeInterior
	}
	return config
}

func NewBridgeConfig() BridgeConfig {
	return BridgeConfig{
		TcpListeners:   map[string]TcpEndpoint{},
		TcpConnectors:  map[string]TcpEndpoint{},
		HttpListeners:  map[string]HttpEndpoint{},
		HttpConnectors: map[string]HttpEndpoint{},
	}
}

func NewBridgeConfigCopy(src BridgeConfig) BridgeConfig {
	newBridges := NewBridgeConfig()
	for k, v := range src.TcpListeners {
		newBridges.TcpListeners[k] = v
	}
	for k, v := range src.TcpConnectors {
		newBridges.TcpConnectors[k] = v
	}
	for k, v := range src.HttpListeners {
		newBridges.HttpListeners[k] = v
	}
	for k, v := range src.HttpConnectors {
		newBridges.HttpConnectors[k] = v
	}
	return newBridges
}

func (r *RouterConfig) AddListener(l Listener) {
	if l.Name == "" {
		l.Name = fmt.Sprintf("%s@%d", l.Host, l.Port)
	}
	r.Listeners[l.Name] = l
}

func (r *RouterConfig) AddConnector(c Connector) {
	r.Connectors[c.Name] = c
}

func (r *RouterConfig) RemoveConnector(name string) (bool, Connector) {
	c, ok := r.Connectors[name]
	if ok {
		delete(r.Connectors, name)
		return true, c
	} else {
		return false, Connector{}
	}
}

func (r *RouterConfig) IsEdge() bool {
	return r.Metadata.Mode == ModeEdge
}

func (r *RouterConfig) AddSslProfileWithPath(path string, s SslProfile) {
	if s.CertFile == "" && s.CaCertFile == "" && s.PrivateKeyFile == "" {
		s.CertFile = fmt.Sprintf(path+"/%s/tls.crt", s.Name)
		s.PrivateKeyFile = fmt.Sprintf(path+"/%s/tls.key", s.Name)
		s.CaCertFile = fmt.Sprintf(path+"/%s/ca.crt", s.Name)
	}
	r.SslProfiles[s.Name] = s
}

func (r *RouterConfig) AddSimpleSslProfileWithPath(path string, s SslProfile) {
	if s.CaCertFile == "" {
		s.CaCertFile = fmt.Sprintf(path+"/%s/ca.crt", s.Name)
	}
	r.SslProfiles[s.Name] = s
}

func (r *RouterConfig) AddSslProfile(s SslProfile) {
	r.AddSslProfileWithPath("/etc/skupper-router-certs", s)
}

func (r *RouterConfig) RemoveSslProfile(name string) bool {
	_, ok := r.SslProfiles[name]
	if ok {
		delete(r.SslProfiles, name)
		return true
	} else {
		return false
	}
}

func (r *RouterConfig) AddAddress(a Address) {
	r.Addresses[a.Prefix] = a
}

func (r *RouterConfig) AddTcpConnector(e TcpEndpoint) {
	r.Bridges.AddTcpConnector(e)
}

func (r *RouterConfig) RemoveTcpConnector(name string) (bool, TcpEndpoint) {
	return r.Bridges.RemoveTcpConnector(name)
}

func (r *RouterConfig) AddTcpListener(e TcpEndpoint) {
	r.Bridges.AddTcpListener(e)
}

func (r *RouterConfig) RemoveTcpListener(name string) (bool, TcpEndpoint) {
	return r.Bridges.RemoveTcpListener(name)
}

func (r *RouterConfig) AddHttpConnector(e HttpEndpoint) {
	r.Bridges.AddHttpConnector(e)
}

func (r *RouterConfig) RemoveHttpConnector(name string) (bool, HttpEndpoint) {
	return r.Bridges.RemoveHttpConnector(name)
}

func (r *RouterConfig) AddHttpListener(e HttpEndpoint) {
	r.Bridges.AddHttpListener(e)
}

func (r *RouterConfig) RemoveHttpListener(name string) (bool, HttpEndpoint) {
	return r.Bridges.RemoveHttpListener(name)
}

func (r *RouterConfig) UpdateBridgeConfig(desired BridgeConfig) bool {
	if reflect.DeepEqual(r.Bridges, desired) {
		return false
	} else {
		r.Bridges = desired
		return true
	}
}

func (r *RouterConfig) GetSiteMetadata() SiteMetadata {
	return getSiteMetadata(r.Metadata.Metadata)
}

func (r *RouterConfig) SetSiteMetadata(site *SiteMetadata) {
	r.Metadata.Metadata = getSiteMetadataString(site.Id, site.Version)
}

func (bc *BridgeConfig) AddTcpConnector(e TcpEndpoint) {
	bc.TcpConnectors[e.Name] = e
}

func (bc *BridgeConfig) RemoveTcpConnector(name string) (bool, TcpEndpoint) {
	tc, ok := bc.TcpConnectors[name]
	if ok {
		delete(bc.TcpConnectors, name)
		return true, tc
	} else {
		return false, TcpEndpoint{}
	}
}

func (bc *BridgeConfig) AddTcpListener(e TcpEndpoint) {
	bc.TcpListeners[e.Name] = e
}

func (bc *BridgeConfig) RemoveTcpListener(name string) (bool, TcpEndpoint) {
	tc, ok := bc.TcpListeners[name]
	if ok {
		delete(bc.TcpListeners, name)
		return true, tc
	} else {
		return false, TcpEndpoint{}
	}
}

func (bc *BridgeConfig) AddHttpConnector(e HttpEndpoint) {
	bc.HttpConnectors[e.Name] = e
}

func (bc *BridgeConfig) RemoveHttpConnector(name string) (bool, HttpEndpoint) {
	tc, ok := bc.HttpConnectors[name]
	if ok {
		delete(bc.HttpConnectors, name)
		return true, tc
	} else {
		return false, HttpEndpoint{}
	}
}

func (bc *BridgeConfig) AddHttpListener(e HttpEndpoint) {
	bc.HttpListeners[e.Name] = e
}

func (bc *BridgeConfig) RemoveHttpListener(name string) (bool, HttpEndpoint) {
	tc, ok := bc.HttpListeners[name]
	if ok {
		delete(bc.HttpListeners, name)
		return true, tc
	} else {
		return false, HttpEndpoint{}
	}
}

func GetHttpConnectors(bridges []BridgeConfig) []HttpEndpoint {
	connectors := []HttpEndpoint{}
	for _, bridge := range bridges {
		for _, connector := range bridge.HttpConnectors {
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func GetHttpListeners(bridges []BridgeConfig) []HttpEndpoint {
	listeners := []HttpEndpoint{}
	for _, bridge := range bridges {
		for _, listener := range bridge.HttpListeners {
			listeners = append(listeners, listener)
		}
	}
	return listeners
}

func GetTcpConnectors(bridges []BridgeConfig) []TcpEndpoint {
	connectors := []TcpEndpoint{}
	for _, bridge := range bridges {
		for _, connector := range bridge.TcpConnectors {
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func (r *RouterConfig) SetLogLevel(module string, level string) bool {
	if level != "" {
		config := LogConfig{
			Module: module,
			Enable: level,
		}
		if module == "" {
			config.Module = "DEFAULT"
		}
		if !strings.HasSuffix(level, "+") {
			config.Enable = level + "+"
		}
		if r.LogConfig == nil {
			r.LogConfig = map[string]LogConfig{}
		}
		if r.LogConfig[config.Module] != config {
			r.LogConfig[config.Module] = config
			return true
		}
	}
	return false
}

func (r *RouterConfig) SetLogLevels(levels map[string]string) bool {
	keys := map[string]bool{}
	for k, _ := range levels {
		if k == "" {
			keys["DEFAULT"] = true
		} else {
			keys[k] = true
		}
	}
	changed := false
	for name, level := range levels {
		if r.SetLogLevel(name, level) {
			changed = true
		}
	}
	for key, _ := range r.LogConfig {
		if _, ok := keys[key]; !ok {
			delete(r.LogConfig, key)
			changed = true
		}
	}
	return changed
}

type Role string

const (
	RoleInterRouter Role = "inter-router"
	RoleEdge             = "edge"
)

type Mode string

const (
	ModeInterior Mode = "interior"
	ModeEdge          = "edge"
)

const (
	HttpVersion1 string = "HTTP1"
	HttpVersion2        = "HTTP2"
)

type RouterMetadata struct {
	Id                 string `json:"id,omitempty"`
	Mode               Mode   `json:"mode,omitempty"`
	HelloMaxAgeSeconds string `json:"helloMaxAgeSeconds,omitempty"`
	Metadata           string `json:"metadata,omitempty"`
}

type SslProfile struct {
	Name           string `json:"name,omitempty"`
	CertFile       string `json:"certFile,omitempty"`
	PrivateKeyFile string `json:"privateKeyFile,omitempty"`
	CaCertFile     string `json:"caCertFile,omitempty"`
}

type LogConfig struct {
	Module string `json:"module"`
	Enable string `json:"enable"`
}

type Listener struct {
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	Role             Role   `json:"role,omitempty" yaml:"role,omitempty"`
	Host             string `json:"host,omitempty" yaml:"host,omitempty"`
	Port             int32  `json:"port" yaml:"port,omitempty"`
	RouteContainer   bool   `json:"routeContainer,omitempty" yaml:"route-container,omitempty"`
	Http             bool   `json:"http,omitempty" yaml:"http,omitempty"`
	Cost             int32  `json:"cost,omitempty" yaml:"cost,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty" yaml:"ssl-profile,omitempty"`
	SaslMechanisms   string `json:"saslMechanisms,omitempty" yaml:"sasl-mechanisms,omitempty"`
	AuthenticatePeer bool   `json:"authenticatePeer,omitempty" yaml:"authenticate-peer,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty" yaml:"link-capacity,omitempty"`
	HttpRootDir      string `json:"httpRootDir,omitempty" yaml:"http-rootdir,omitempty"`
	Websockets       bool   `json:"websockets,omitempty" yaml:"web-sockets,omitempty"`
	Healthz          bool   `json:"healthz,omitempty" yaml:"healthz,omitempty"`
	Metrics          bool   `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	MaxFrameSize     int    `json:"maxFrameSize,omitempty" yaml:"max-frame-size,omitempty"`
	MaxSessionFrames int    `json:"maxSessionFrames,omitempty" yaml:"max-session-frames,omitempty"`
}

func (l *Listener) SetMaxFrameSize(value int) {
	l.MaxFrameSize = value
}

func (l *Listener) SetMaxSessionFrames(value int) {
	l.MaxSessionFrames = value
}

type Connector struct {
	Name             string `json:"name,omitempty"`
	Role             Role   `json:"role,omitempty"`
	Host             string `json:"host"`
	Port             string `json:"port"`
	RouteContainer   bool   `json:"routeContainer,omitempty"`
	Cost             int32  `json:"cost,omitempty"`
	VerifyHostname   bool   `json:"verifyHostname,omitempty"`
	SslProfile       string `json:"sslProfile,omitempty"`
	LinkCapacity     int32  `json:"linkCapacity,omitempty"`
	MaxFrameSize     int    `json:"maxFrameSize,omitempty"`
	MaxSessionFrames int    `json:"maxSessionFrames,omitempty"`
}

func (c *Connector) SetMaxFrameSize(value int) {
	c.MaxFrameSize = value
}

func (c *Connector) SetMaxSessionFrames(value int) {
	c.MaxSessionFrames = value
}

type Distribution string

const (
	DistributionBalanced  Distribution = "balanced"
	DistributionMulticast              = "multicast"
	DistributionClosest                = "closest"
)

type Address struct {
	Prefix       string `json:"prefix,omitempty"`
	Distribution string `json:"distribution,omitempty"`
}

type TcpEndpoint struct {
	Name    string `json:"name,omitempty"`
	Host    string `json:"host,omitempty"`
	Port    string `json:"port,omitempty"`
	Address string `json:"address,omitempty"`
	SiteId  string `json:"siteId,omitempty"`
}

type HttpEndpoint struct {
	Name            string `json:"name,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            string `json:"port,omitempty"`
	Address         string `json:"address,omitempty"`
	SiteId          string `json:"siteId,omitempty"`
	ProtocolVersion string `json:"protocolVersion,omitempty"`
	Aggregation     string `json:"aggregation,omitempty"`
	EventChannel    bool   `json:"eventChannel,omitempty"`
	HostOverride    string `json:"hostOverride,omitempty"`
	SslProfile      string `json:"sslProfile,omitempty"`
	VerifyHostname  *bool  `json:"verifyHostname,omitempty"`
}

func convert(from interface{}, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, to)
	if err != nil {
		return err
	}
	return nil
}

func UnmarshalRouterConfig(config string) (RouterConfig, error) {
	result := RouterConfig{
		Metadata:    RouterMetadata{},
		Addresses:   map[string]Address{},
		SslProfiles: map[string]SslProfile{},
		Listeners:   map[string]Listener{},
		Connectors:  map[string]Connector{},
		LogConfig:   map[string]LogConfig{},
		Bridges: BridgeConfig{
			TcpListeners:   map[string]TcpEndpoint{},
			TcpConnectors:  map[string]TcpEndpoint{},
			HttpListeners:  map[string]HttpEndpoint{},
			HttpConnectors: map[string]HttpEndpoint{},
		},
	}
	var obj interface{}
	err := json.Unmarshal([]byte(config), &obj)
	if err != nil {
		return result, err
	}
	elements, ok := obj.([]interface{})
	if !ok {
		return result, fmt.Errorf("Invalid JSON for router configuration, expected array at top level got %#v", obj)
	}
	for _, e := range elements {
		element, ok := e.([]interface{})
		if !ok || len(element) != 2 {
			return result, fmt.Errorf("Invalid JSON for router configuration, expected array with type and value got %#v", e)
		}
		entityType, ok := element[0].(string)
		if !ok {
			return result, fmt.Errorf("Invalid JSON for router configuration, expected entity type as string got %#v", element[0])
		}
		switch entityType {
		case "router":
			metadata := RouterMetadata{}
			err = convert(element[1], &metadata)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Metadata = metadata
		case "address":
			address := Address{}
			err = convert(element[1], &address)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Addresses[address.Prefix] = address
		case "connector":
			connector := Connector{}
			err = convert(element[1], &connector)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Connectors[connector.Name] = connector
		case "listener":
			listener := Listener{}
			err = convert(element[1], &listener)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Listeners[listener.Name] = listener
		case "sslProfile":
			sslProfile := SslProfile{}
			err = convert(element[1], &sslProfile)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.SslProfiles[sslProfile.Name] = sslProfile
		case "log":
			logConfig := LogConfig{}
			err = convert(element[1], &logConfig)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.LogConfig[logConfig.Module] = logConfig
		case "tcpConnector":
			connector := TcpEndpoint{}
			err = convert(element[1], &connector)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.TcpConnectors[connector.Name] = connector
		case "tcpListener":
			listener := TcpEndpoint{}
			err = convert(element[1], &listener)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.TcpListeners[listener.Name] = listener
		case "httpConnector":
			connector := HttpEndpoint{}
			err = convert(element[1], &connector)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.HttpConnectors[connector.Name] = connector
		case "httpListener":
			listener := HttpEndpoint{}
			err = convert(element[1], &listener)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.Bridges.HttpListeners[listener.Name] = listener
		default:
		}
	}
	return result, nil
}

func MarshalRouterConfig(config RouterConfig) (string, error) {
	elements := [][]interface{}{}
	tuple := []interface{}{
		"router",
		config.Metadata,
	}
	elements = append(elements, tuple)
	for _, e := range config.SslProfiles {
		tuple := []interface{}{
			"sslProfile",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Connectors {
		tuple := []interface{}{
			"connector",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Listeners {
		tuple := []interface{}{
			"listener",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Addresses {
		tuple := []interface{}{
			"address",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.TcpConnectors {
		tuple := []interface{}{
			"tcpConnector",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.TcpListeners {
		tuple := []interface{}{
			"tcpListener",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.HttpConnectors {
		tuple := []interface{}{
			"httpConnector",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.Bridges.HttpListeners {
		tuple := []interface{}{
			"httpListener",
			e,
		}
		elements = append(elements, tuple)
	}
	for _, e := range config.LogConfig {
		tuple := []interface{}{
			"log",
			e,
		}
		elements = append(elements, tuple)
	}
	data, err := json.MarshalIndent(elements, "", "    ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func AsConfigMapData(config string) map[string]string {
	return map[string]string{
		types.TransportConfigFile: config,
	}
}

func (r *RouterConfig) AsConfigMapData() (map[string]string, error) {
	result := map[string]string{}
	marshalled, err := MarshalRouterConfig(*r)
	if err != nil {
		return result, err
	}
	result[types.TransportConfigFile] = marshalled
	return result, nil
}

func (r *RouterConfig) WriteToConfigMap(configmap *corev1.ConfigMap) error {
	var err error
	configmap.Data, err = r.AsConfigMapData()
	return err
}

func (r *RouterConfig) UpdateConfigMap(configmap *corev1.ConfigMap) (bool, error) {
	if configmap.Data != nil && configmap.Data[types.TransportConfigFile] != "" {
		existing, err := UnmarshalRouterConfig(configmap.Data[types.TransportConfigFile])
		if err != nil {
			return false, err
		}
		if reflect.DeepEqual(existing, *r) {
			return false, nil
		}
	}
	err := r.WriteToConfigMap(configmap)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (b *BridgeConfig) UpdateConfigMap(configmap *corev1.ConfigMap) (bool, error) {
	if configmap.Data != nil && configmap.Data[types.TransportConfigFile] != "" {
		existing, err := UnmarshalRouterConfig(configmap.Data[types.TransportConfigFile])
		if err != nil {
			return false, err
		}
		if reflect.DeepEqual(existing.Bridges, *b) {
			return false, nil
		} else {
			existing.Bridges = *b
			configmap.Data, err = existing.AsConfigMapData()
			if err != nil {
				return false, err
			}
			return true, nil
		}
	} else {
		return false, fmt.Errorf("Router config not defined")
	}
}

func GetRouterConfigFromConfigMap(configmap *corev1.ConfigMap) (*RouterConfig, error) {
	if configmap.Data == nil || configmap.Data[types.TransportConfigFile] == "" {
		return nil, nil
	} else {
		routerConfig, err := UnmarshalRouterConfig(configmap.Data[types.TransportConfigFile])
		if err != nil {
			return nil, err
		}
		return &routerConfig, nil
	}
}

func GetBridgeConfigFromConfigMap(configmap *corev1.ConfigMap) (*BridgeConfig, error) {
	routerConfig, err := GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}
	return &routerConfig.Bridges, nil
}

type ConnectorDifference struct {
	Deleted          []Connector
	Added            []Connector
	AddedSslProfiles map[string]SslProfile
}

type TcpEndpointDifference struct {
	Deleted []string
	Added   []TcpEndpoint
}

type HttpEndpointDifference struct {
	Deleted []HttpEndpoint
	Added   []HttpEndpoint
}

type BridgeConfigDifference struct {
	TcpListeners       TcpEndpointDifference
	TcpConnectors      TcpEndpointDifference
	HttpListeners      HttpEndpointDifference
	HttpConnectors     HttpEndpointDifference
	AddedSslProfiles   []string
	DeletedSSlProfiles []string
}

func isAddrAny(host string) bool {
	ip := net.ParseIP(host)
	return ip.Equal(net.IPv4zero) || ip.Equal(net.IPv6zero)
}

func equivalentHost(a string, b string) bool {
	if a == b {
		return true
	} else if a == "" {
		return isAddrAny(b)
	} else if b == "" {
		return isAddrAny(a)
	} else {
		return false
	}
}

func (a TcpEndpoint) Equivalent(b TcpEndpoint) bool {
	if !equivalentHost(a.Host, b.Host) || a.Port != b.Port || a.Address != b.Address ||
		a.SiteId != b.SiteId {
		return false
	}
	return true
}

func (a TcpEndpointMap) Difference(b TcpEndpointMap) TcpEndpointDifference {
	result := TcpEndpointDifference{}
	for key, v1 := range b {
		v2, ok := a[key]
		if !ok {
			result.Added = append(result.Added, v1)
		} else if !v1.Equivalent(v2) {
			result.Deleted = append(result.Deleted, v1.Name)
			result.Added = append(result.Added, v1)
		}
	}
	for key, v1 := range a {
		_, ok := b[key]
		if !ok {
			result.Deleted = append(result.Deleted, v1.Name)
		}
	}
	return result
}

func (a HttpEndpoint) Equivalent(b HttpEndpoint) bool {
	if !equivalentHost(a.Host, b.Host) || a.Port != b.Port || a.Address != b.Address ||
		a.SiteId != b.SiteId || a.Aggregation != b.Aggregation ||
		a.EventChannel != b.EventChannel || a.HostOverride != b.HostOverride {
		return false
	}
	if a.ProtocolVersion == HttpVersion2 && b.ProtocolVersion != HttpVersion2 {
		return false
	}
	return true
}

func (a HttpEndpointMap) Difference(b HttpEndpointMap) HttpEndpointDifference {
	result := HttpEndpointDifference{}
	for key, v1 := range b {
		v2, ok := a[key]
		if !ok {
			result.Added = append(result.Added, v1)
		} else if !v1.Equivalent(v2) {
			result.Deleted = append(result.Deleted, v1)
			result.Added = append(result.Added, v1)
		}
	}
	for key, v1 := range a {
		_, ok := b[key]
		if !ok {
			result.Deleted = append(result.Deleted, v1)
		}
	}
	return result
}

func (a *BridgeConfig) Difference(b *BridgeConfig) *BridgeConfigDifference {
	result := BridgeConfigDifference{
		TcpConnectors:  a.TcpConnectors.Difference(b.TcpConnectors),
		TcpListeners:   a.TcpListeners.Difference(b.TcpListeners),
		HttpConnectors: a.HttpConnectors.Difference(b.HttpConnectors),
		HttpListeners:  a.HttpListeners.Difference(b.HttpListeners),
	}

	result.AddedSslProfiles, result.DeletedSSlProfiles = getSslProfilesDifference(a, b)

	return &result
}

type AddedSslProfiles []string
type DeletedSslProfiles []string

func getSslProfilesDifference(before *BridgeConfig, desired *BridgeConfig) (AddedSslProfiles, DeletedSslProfiles) {
	var addedProfiles AddedSslProfiles
	var deletedProfiles DeletedSslProfiles

	originalSslConfig := make(map[string]string)
	newSslConfig := make(map[string]string)

	for _, httpConnector := range before.HttpConnectors {
		originalSslConfig[httpConnector.SslProfile] = httpConnector.SslProfile
	}
	for _, httpListener := range before.HttpListeners {
		originalSslConfig[httpListener.SslProfile] = httpListener.SslProfile
	}

	for _, httpConnector := range desired.HttpConnectors {
		newSslConfig[httpConnector.SslProfile] = httpConnector.SslProfile
	}
	for _, httpListener := range desired.HttpListeners {
		newSslConfig[httpListener.SslProfile] = httpListener.SslProfile
	}

	for key, name := range originalSslConfig {
		_, ok := newSslConfig[key]

		if !ok && name != types.ServiceClientSecret {
			deletedProfiles = append(deletedProfiles, name)
		}
	}

	for key, name := range newSslConfig {
		_, ok := originalSslConfig[key]

		if !ok && name != types.ServiceClientSecret {
			addedProfiles = append(addedProfiles, name)
		}
	}

	return addedProfiles, deletedProfiles
}

func (a *TcpEndpointDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func (a *HttpEndpointDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func (a *BridgeConfigDifference) Empty() bool {
	return a.TcpConnectors.Empty() && a.TcpListeners.Empty() && a.HttpConnectors.Empty() && a.HttpListeners.Empty()
}

func (a *BridgeConfigDifference) Print() {
	log.Printf("TcpConnectors added=%v, deleted=%v", a.TcpConnectors.Added, a.TcpConnectors.Deleted)
	log.Printf("TcpListeners added=%v, deleted=%v", a.TcpListeners.Added, a.TcpListeners.Deleted)
	log.Printf("HttpConnectors added=%v, deleted=%v", a.HttpConnectors.Added, a.HttpConnectors.Deleted)
	log.Printf("HttpListeners added=%v, deleted=%v", a.HttpListeners.Added, a.HttpListeners.Deleted)
	log.Printf("SslProfiles added=%v, deleted=%v", a.AddedSslProfiles, a.DeletedSSlProfiles)
}

func ConnectorsDifference(actual map[string]Connector, desired *RouterConfig, ignorePrefix *string) *ConnectorDifference {
	result := ConnectorDifference{}
	result.AddedSslProfiles = make(map[string]SslProfile)
	for key, v1 := range desired.Connectors {
		_, ok := actual[key]
		if !ok {
			result.Added = append(result.Added, v1)
			result.AddedSslProfiles[v1.SslProfile] = desired.SslProfiles[v1.SslProfile]
		}
	}
	for key, v1 := range actual {
		_, ok := desired.Connectors[key]

		allowedToDelete := true
		if ignorePrefix != nil && len(*ignorePrefix) > 0 {
			allowedToDelete = !strings.HasPrefix(v1.Name, *ignorePrefix)
		}

		if !ok && allowedToDelete {
			result.Deleted = append(result.Deleted, v1)
		}
	}
	return &result
}

func (a *ConnectorDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func GetRouterConfigForHeadlessProxy(definition types.ServiceInterface, siteId string, version string, namespace string) (string, error) {
	config := InitialConfig("$HOSTNAME", siteId, version, true, 3)
	// add edge-connector
	config.AddSslProfile(SslProfile{
		Name: types.InterRouterProfile,
	})
	config.AddConnector(Connector{
		Name:       "uplink",
		SslProfile: types.InterRouterProfile,
		Host:       types.TransportServiceName + "." + namespace,
		Port:       strconv.Itoa(int(types.EdgeListenerPort)),
		Role:       RoleEdge,
	})
	config.AddListener(Listener{
		Name: "amqp",
		Host: "localhost",
		Port: 5672,
	})
	svcPorts := definition.Ports
	ports := map[int]int{}
	if len(definition.Targets) > 0 {
		ports = definition.Targets[0].TargetPorts
	} else {
		for _, sp := range svcPorts {
			ports[sp] = sp
		}
	}
	for iPort, ePort := range ports {
		address := fmt.Sprintf("%s-%s:%d", definition.Address, "${POD_ID}", iPort)
		if definition.Origin == "" {
			name := fmt.Sprintf("egress:%d", ePort)
			host := definition.Headless.Name + "-${POD_ID}." + definition.Address + "." + namespace
			// in the originating site, just have egress bindings
			switch definition.Protocol {
			case "tcp":
				config.AddTcpConnector(TcpEndpoint{
					Name:    name,
					Host:    host,
					Port:    strconv.Itoa(ePort),
					Address: address,
					SiteId:  siteId,
				})
			case "http":
				config.AddHttpConnector(HttpEndpoint{
					Name:    name,
					Host:    host,
					Port:    strconv.Itoa(ePort),
					Address: address,
					SiteId:  siteId,
				})
			case "http2":
				config.AddHttpConnector(HttpEndpoint{
					Name:            name,
					Host:            host,
					Port:            strconv.Itoa(ePort),
					Address:         address,
					ProtocolVersion: HttpVersion2,
					SiteId:          siteId,
				})
			default:
			}
		} else {
			name := fmt.Sprintf("ingress:%d", ePort)
			// in all other sites, just have ingress bindings
			switch definition.Protocol {
			case "tcp":
				config.AddTcpListener(TcpEndpoint{
					Name:    name,
					Port:    strconv.Itoa(iPort),
					Address: address,
					SiteId:  siteId,
				})
			case "http":
				config.AddHttpListener(HttpEndpoint{
					Name:    name,
					Port:    strconv.Itoa(iPort),
					Address: address,
					SiteId:  siteId,
				})
			case "http2":
				config.AddHttpListener(HttpEndpoint{
					Name:            name,
					Port:            strconv.Itoa(iPort),
					Address:         address,
					ProtocolVersion: HttpVersion2,
					SiteId:          siteId,
				})
			default:
			}
		}
	}
	return MarshalRouterConfig(config)
}
