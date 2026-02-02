package qdr

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	path_ "path"
	"reflect"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
)

type RouterConfig struct {
	Metadata    RouterMetadata
	SslProfiles map[string]SslProfile
	Listeners   map[string]Listener
	Connectors  map[string]Connector
	Addresses   map[string]Address
	LogConfig   map[string]LogConfig
	SiteConfig  *SiteConfig
	Bridges     BridgeConfig
}

type RouterConfigHandler interface {
	GetRouterConfig() (*RouterConfig, error)
	SaveRouterConfig(*RouterConfig) error
	RemoveRouterConfig() error
}

type TcpEndpointMap map[string]TcpEndpoint

type BridgeConfig struct {
	TcpListeners  TcpEndpointMap
	TcpConnectors TcpEndpointMap
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
			TcpListeners:  map[string]TcpEndpoint{},
			TcpConnectors: map[string]TcpEndpoint{},
		},
	}
	if edge {
		config.Metadata.Mode = ModeEdge
	} else {
		config.Metadata.Mode = ModeInterior
	}
	return config
}

func (r *RouterConfig) AddHealthAndMetricsListener(port int32) {
	r.AddListener(Listener{
		Port:        port,
		Role:        "normal",
		Http:        true,
		HttpRootDir: "disabled",
		Websockets:  false,
		Healthz:     true,
		Metrics:     true,
	})
}

func NewBridgeConfig() BridgeConfig {
	return BridgeConfig{
		TcpListeners:  map[string]TcpEndpoint{},
		TcpConnectors: map[string]TcpEndpoint{},
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
	return newBridges
}

func (r *RouterConfig) AddListener(l Listener) bool {
	if l.Name == "" {
		l.Name = fmt.Sprintf("%s@%d", l.Host, l.Port)
	}
	if original, ok := r.Listeners[l.Name]; ok && original == l {
		return false
	}
	r.Listeners[l.Name] = l
	return true
}

func (r *RouterConfig) RemoveListener(name string) (bool, Listener) {
	c, ok := r.Listeners[name]
	if ok {
		delete(r.Listeners, name)
		return true, c
	} else {
		return false, Listener{}
	}
}

func (r *RouterConfig) AddConnector(c Connector) bool {
	if original, ok := r.Connectors[c.Name]; ok && original == c {
		return false
	}
	r.Connectors[c.Name] = c
	return true
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

const SSL_PROFILE_PATH = "/etc/skupper-router-certs"

func ConfigureSslProfile(name string, path string, clientAuth bool) SslProfile {
	profile := SslProfile{
		Name:       name,
		CaCertFile: path_.Join(path, name, "ca.crt"),
	}
	if clientAuth {
		profile.CertFile = path_.Join(path, name, "tls.crt")
		profile.PrivateKeyFile = path_.Join(path, name, "tls.key")
	}
	return profile
}

func (r *RouterConfig) AddSslProfile(s SslProfile) bool {
	if original, ok := r.SslProfiles[s.Name]; ok && original == s {
		return false
	}
	r.SslProfiles[s.Name] = s
	return true
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

func (r *RouterConfig) RemoveUnreferencedSslProfiles() bool {
	unreferenced := r.UnreferencedSslProfiles()
	changed := false
	for _, profile := range unreferenced {
		if r.RemoveSslProfile(profile.Name) {
			changed = true
		}
	}
	return changed
}

func (r *RouterConfig) UnreferencedSslProfiles() map[string]SslProfile {
	results := map[string]SslProfile{}
	for _, profile := range r.SslProfiles {
		results[profile.Name] = profile
	}
	//remove any that are referenced
	for _, o := range r.Listeners {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Connectors {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Bridges.TcpListeners {
		delete(results, o.SslProfile)
	}
	for _, o := range r.Bridges.TcpConnectors {
		delete(results, o.SslProfile)
	}

	return results
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

func (r *RouterConfig) UpdateBridgeConfig(desired BridgeConfig) bool {
	if reflect.DeepEqual(r.Bridges, desired) {
		return false
	} else {
		r.Bridges = desired
		return true
	}
}

func (r *RouterConfig) GetSiteMetadata() SiteMetadata {
	return GetSiteMetadata(r.Metadata.Metadata)
}

func (r *RouterConfig) SetSiteMetadata(site *SiteMetadata) {
	r.Metadata.Metadata = getSiteMetadataString(site.Id, site.Version)
}

func (bc *BridgeConfig) AddTcpConnector(e TcpEndpoint) bool {
	var updated = true
	if existing, ok := bc.TcpConnectors[e.Name]; ok {
		if e == existing {
			updated = false
		}
	}
	bc.TcpConnectors[e.Name] = e
	return updated
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

func (bc *BridgeConfig) AddTcpListener(e TcpEndpoint) bool {
	var updated = true
	if existing, ok := bc.TcpListeners[e.Name]; ok {
		if e == existing {
			updated = false
		}
	}
	bc.TcpListeners[e.Name] = e
	return updated
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
	RoleNormal           = "normal"
	RoleDefault          = ""
)

func asRole(name string) Role {
	if name == "edge" {
		return RoleEdge
	}
	if name == "inter-router" {
		return RoleInterRouter
	}
	if name == "normal" {
		return RoleNormal
	}
	return RoleDefault
}

func GetRole(name string) Role {
	if name == "edge" {
		return RoleEdge
	} else if name == "normal" {
		return RoleNormal
	}
	return RoleInterRouter
}

type Mode string

const (
	ModeInterior Mode = "interior"
	ModeEdge          = "edge"
)

type RouterMetadata struct {
	Id                  string `json:"id,omitempty"`
	Mode                Mode   `json:"mode,omitempty"`
	HelloMaxAgeSeconds  string `json:"helloMaxAgeSeconds,omitempty"`
	DataConnectionCount string `json:"dataConnectionCount,omitempty"`
	Metadata            string `json:"metadata,omitempty"`
}

type SslProfile struct {
	Name               string `json:"name,omitempty"`
	CertFile           string `json:"certFile,omitempty"`
	PrivateKeyFile     string `json:"privateKeyFile,omitempty"`
	CaCertFile         string `json:"caCertFile,omitempty"`
	Ordinal            uint64 `json:"ordinal,omitempty"`
	OldestValidOrdinal uint64 `json:"oldestValidOrdinal,omitempty"`
}

func (p SslProfile) toRecord() Record {
	result := make(map[string]any)
	if p.Name != "" {
		result["name"] = p.Name
	}
	if p.CertFile != "" {
		result["certFile"] = p.CertFile
	}
	if p.PrivateKeyFile != "" {
		result["privateKeyFile"] = p.PrivateKeyFile
	}
	if p.CaCertFile != "" {
		result["caCertFile"] = p.CaCertFile
	}
	if p.Ordinal > 0 {
		result["ordinal"] = p.Ordinal
	}
	if p.OldestValidOrdinal > 0 {
		result["oldestValidOrdinal"] = p.OldestValidOrdinal
	}
	return result
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

func (listener Listener) toRecord() Record {
	record := map[string]any{}
	record["name"] = listener.Name
	record["role"] = string(listener.Role)
	record["host"] = listener.Host
	record["port"] = strconv.Itoa(int(listener.Port))
	if listener.Cost > 0 {
		record["cost"] = listener.Cost
	}
	if listener.LinkCapacity > 0 {
		record["linkCapacity"] = listener.LinkCapacity
	}
	if len(listener.SslProfile) > 0 {
		record["sslProfile"] = listener.SslProfile
	}
	if listener.AuthenticatePeer {
		record["authenticatePeer"] = listener.AuthenticatePeer
	}
	if len(listener.SaslMechanisms) > 0 {
		record["saslMechanisms"] = listener.SaslMechanisms
	}
	if listener.MaxFrameSize > 0 {
		record["maxFrameSize"] = listener.MaxFrameSize
	}
	if listener.MaxSessionFrames > 0 {
		record["maxSessionFrames"] = listener.MaxSessionFrames
	}
	if listener.RouteContainer {
		record["routeContainer"] = listener.RouteContainer
	}
	if listener.Http {
		record["http"] = listener.Http
	}
	if len(listener.HttpRootDir) > 0 {
		record["httpRootDir"] = listener.HttpRootDir
	}
	if listener.Websockets {
		record["websockets"] = listener.Websockets
	}
	if listener.Healthz {
		record["healthz"] = listener.Healthz
	}
	if listener.Metrics {
		record["metrics"] = listener.Metrics
	}

	return record
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

func (connector Connector) toRecord() Record {
	record := map[string]any{}
	record["name"] = connector.Name
	record["role"] = string(connector.Role)
	record["host"] = connector.Host
	record["port"] = connector.Port
	if connector.Cost > 0 {
		record["cost"] = connector.Cost
	}
	if len(connector.SslProfile) > 0 {
		record["sslProfile"] = connector.SslProfile
	}
	if connector.MaxFrameSize > 0 {
		record["maxFrameSize"] = connector.MaxFrameSize
	}
	if connector.MaxSessionFrames > 0 {
		record["maxSessionFrames"] = connector.MaxSessionFrames
	}
	return record
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
	Name           string `json:"name,omitempty"`
	Host           string `json:"host,omitempty"`
	Port           string `json:"port,omitempty"`
	Address        string `json:"address,omitempty"`
	SiteId         string `json:"siteId,omitempty"`
	SslProfile     string `json:"sslProfile,omitempty"`
	Observer       string `json:"observer,omitempty"`
	VerifyHostname *bool  `json:"verifyHostname,omitempty"`
	ProcessID      string `json:"processId,omitempty"`
}

func (e TcpEndpoint) toRecord() Record {
	result := make(map[string]any)
	if e.Name != "" {
		result["name"] = e.Name
	}
	if e.Host != "" {
		result["host"] = e.Host
	}
	if e.Port != "" {
		result["port"] = e.Port
	}
	if e.Address != "" {
		result["address"] = e.Address
	}
	if e.SiteId != "" {
		result["siteId"] = e.SiteId
	}
	if e.SslProfile != "" {
		result["sslProfile"] = e.SslProfile
	}
	if e.Observer != "" {
		result["observer"] = e.Observer
	}
	if e.VerifyHostname != nil {
		result["verifyHostname"] = e.VerifyHostname
	}
	if e.ProcessID != "" {
		result["processId"] = e.ProcessID
	}
	return result
}

type SiteConfig struct {
	Name      string `json:"name,omitempty"`
	Location  string `json:"location,omitempty"`
	Provider  string `json:"provider,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Version   string `json:"version,omitempty"`
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

func RouterConfigEquals(actual, desired string) bool {
	actualConfig, err := UnmarshalRouterConfig(actual)
	if err != nil {
		return false
	}
	desiredConfig, err := UnmarshalRouterConfig(desired)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(actualConfig, desiredConfig)
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
			TcpListeners:  map[string]TcpEndpoint{},
			TcpConnectors: map[string]TcpEndpoint{},
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
		case "site":
			siteConfig := &SiteConfig{}
			err = convert(element[1], siteConfig)
			if err != nil {
				return result, fmt.Errorf("Invalid %s element got %#v", entityType, element[1])
			}
			result.SiteConfig = siteConfig
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
	for _, e := range config.LogConfig {
		tuple := []interface{}{
			"log",
			e,
		}
		elements = append(elements, tuple)
	}
	if config.SiteConfig != nil {
		tuple := []interface{}{
			"site",
			*config.SiteConfig,
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

type ListenerPredicate func(Listener) bool

func IsNotProtectedListener(l Listener) bool {
	protectedNames := [3]string{"@9090", "amqp", "amqps"}
	for _, name := range protectedNames {
		if l.Name == name {
			return false
		}
	}
	return true
}

func FilterListeners(in map[string]Listener, predicate ListenerPredicate) map[string]Listener {
	results := map[string]Listener{}
	for key, listener := range in {
		if predicate(listener) {
			results[key] = listener
		}
	}
	return results
}

func (config *RouterConfig) GetMatchingListeners(predicate ListenerPredicate) map[string]Listener {
	return FilterListeners(config.Listeners, predicate)
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

type BridgeConfigDifference struct {
	TcpListeners       TcpEndpointDifference
	TcpConnectors      TcpEndpointDifference
	AddedSslProfiles   []string
	DeletedSSlProfiles []string
	logger             *slog.Logger
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

func (a TcpEndpoint) equivalentVerifyHostname(b TcpEndpoint) bool {
	if a.VerifyHostname == nil {
		return b.VerifyHostname == nil || *b.VerifyHostname == true
	}
	if b.VerifyHostname == nil {
		return a.VerifyHostname == nil || *a.VerifyHostname == true
	}
	return *a.VerifyHostname == *b.VerifyHostname
}

func (a TcpEndpoint) Equivalent(b TcpEndpoint) bool {
	obsA := a.Observer
	if obsA == "" {
		obsA = "auto"
	}
	obsB := b.Observer
	if obsB == "" {
		obsB = "auto"
	}
	if !equivalentHost(a.Host, b.Host) || a.Port != b.Port || a.Address != b.Address ||
		a.SiteId != b.SiteId || a.ProcessID != b.ProcessID || !a.equivalentVerifyHostname(b) ||
		obsA != obsB {
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

func (a *BridgeConfig) Difference(b *BridgeConfig) *BridgeConfigDifference {
	result := BridgeConfigDifference{
		logger:        slog.New(slog.Default().Handler()).With("component", "qdr.bridgeConfigDifference"),
		TcpConnectors: a.TcpConnectors.Difference(b.TcpConnectors),
		TcpListeners:  a.TcpListeners.Difference(b.TcpListeners),
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

	for _, tcpConnector := range before.TcpConnectors {
		originalSslConfig[tcpConnector.SslProfile] = tcpConnector.SslProfile
	}
	for _, tcpListener := range before.TcpListeners {
		originalSslConfig[tcpListener.SslProfile] = tcpListener.SslProfile
	}

	for _, tcpConnector := range desired.TcpConnectors {
		newSslConfig[tcpConnector.SslProfile] = tcpConnector.SslProfile
	}
	for _, tcpListener := range desired.TcpListeners {
		newSslConfig[tcpListener.SslProfile] = tcpListener.SslProfile
	}

	//Auto-generated Skupper certs will be deleted if they are not used in the desired configuration
	for key, name := range originalSslConfig {
		_, ok := newSslConfig[key]

		if !ok && isGeneratedBySkupper(name) {
			deletedProfiles = append(deletedProfiles, name)
		}
	}

	//New profiles associated with http or tcp connectors/listeners will be created in the router
	for key, name := range newSslConfig {
		_, ok := originalSslConfig[key]

		if !ok && name != types.ServiceClientSecret {
			addedProfiles = append(addedProfiles, name)
		}
	}

	return addedProfiles, deletedProfiles
}

func isGeneratedBySkupper(name string) bool {
	return strings.HasPrefix(name, types.SkupperServiceCertPrefix) && name != types.ServiceClientSecret
}

func (a *TcpEndpointDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func (a *BridgeConfigDifference) Empty() bool {
	return a.TcpConnectors.Empty() && a.TcpListeners.Empty()
}

func (a *BridgeConfigDifference) Print() {
	a.logger.Info("TcpConnectors", slog.Any("added", a.TcpConnectors.Added), slog.Any("deleted", a.TcpConnectors.Deleted))
	a.logger.Info("TcpListeners", slog.Any("added", a.TcpListeners.Added), slog.Any("deleted", a.TcpListeners.Deleted))
	a.logger.Info("SslProfiles", slog.Any("added", a.AddedSslProfiles), slog.Any("deleted", a.DeletedSSlProfiles))
}

func ConnectorsDifference(actual map[string]Connector, desired *RouterConfig, ignorePrefix *string) *ConnectorDifference {
	result := ConnectorDifference{}
	result.AddedSslProfiles = make(map[string]SslProfile)
	for key, v1 := range desired.Connectors {
		actualValue, ok := actual[key]
		if !ok {
			result.Added = append(result.Added, v1)
			result.AddedSslProfiles[v1.SslProfile] = desired.SslProfiles[v1.SslProfile]
		}

		//in case the link connector exists but has changed some of its values, it needs to be recreated again
		if ok && v1.IsLinkConnector() && !v1.Equivalent(actualValue) {
			result.Deleted = append(result.Deleted, v1)
			result.Added = append(result.Added, v1)
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

func (desired Connector) IsLinkConnector() bool {
	return desired.Role == "inter-router" || desired.Role == "edge"
}

func (desired Connector) Equivalent(actual Connector) bool {
	return desired.Name == actual.Name &&
		desired.Host == actual.Host &&
		desired.Port == actual.Port &&
		desired.Cost == actual.Cost &&
		desired.SslProfile == actual.SslProfile
}

type ListenerDifference struct {
	Deleted []Listener
	Added   []Listener
}

func (desired Listener) Equivalent(actual Listener) bool {
	return desired.Name == actual.Name &&
		desired.Role == actual.Role &&
		desired.Host == actual.Host &&
		desired.Port == actual.Port &&
		desired.RouteContainer == actual.RouteContainer &&
		desired.Http == actual.Http &&
		desired.SslProfile == actual.SslProfile &&
		desired.SaslMechanisms == actual.SaslMechanisms &&
		desired.AuthenticatePeer == actual.AuthenticatePeer &&
		(desired.Cost == 0 || desired.Cost == actual.Cost) &&
		(desired.MaxFrameSize == 0 || desired.MaxFrameSize == actual.MaxFrameSize) &&
		(desired.MaxSessionFrames == 0 || desired.MaxSessionFrames == actual.MaxSessionFrames) &&
		(desired.LinkCapacity == 0 || desired.LinkCapacity == actual.LinkCapacity) &&
		(desired.HttpRootDir == "" || desired.HttpRootDir == actual.HttpRootDir)
	//Skip check for Websockets, Healthz and Metrics as they are
	//always coming back as true at present and are not used where
	//this method is required at present.
}

func ListenersDifference(actual map[string]Listener, desired map[string]Listener) *ListenerDifference {
	result := ListenerDifference{}
	for key, desiredValue := range desired {
		if actualValue, ok := actual[key]; ok {
			if !desiredValue.Equivalent(actualValue) {
				slog.Info("Listener definition does not match", slog.Any("actual", actualValue), slog.Any("desired", desiredValue))
				// handle change as delete then add, so it also works over management protocol
				result.Deleted = append(result.Deleted, desiredValue)
				result.Added = append(result.Added, desiredValue)
			}
		} else {
			result.Added = append(result.Added, desiredValue)
		}
	}
	for key, value := range actual {
		if _, ok := desired[key]; !ok {
			result.Deleted = append(result.Deleted, value)
		}
	}
	return &result
}

func (a *ListenerDifference) Empty() bool {
	return len(a.Deleted) == 0 && len(a.Added) == 0
}

func GetRouterConfigForHeadlessProxy(definition types.ServiceInterface, siteId string, version string, namespace string, profilePath string) (string, error) {
	config := InitialConfig("${HOSTNAME}-"+siteId, siteId, version, true, 3)
	// add edge-connector
	config.AddSslProfile(ConfigureSslProfile(types.InterRouterProfile, profilePath, true))
	config.AddConnector(Connector{
		Name:       "uplink",
		SslProfile: types.InterRouterProfile,
		Host:       types.TransportServiceName + "." + namespace + ".svc.cluster.local",
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
		if definition.IsOfLocalOrigin() {
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
			default:
			}
		}
	}
	return MarshalRouterConfig(config)
}

func disableMutualTLS(l *Listener) {
	l.SaslMechanisms = ""
	l.AuthenticatePeer = false
}

func InteriorListener(options types.RouterOptions) Listener {
	l := Listener{
		Name:             "interior-listener",
		Role:             RoleInterRouter,
		Port:             types.InterRouterListenerPort,
		SslProfile:       types.InterRouterProfile, // The skupper-internal profile needs to be filtered by the config-sync sidecar, in order to avoid deleting automesh connectors
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
		MaxFrameSize:     options.MaxFrameSize,
		MaxSessionFrames: options.MaxSessionFrames,
	}
	if options.DisableMutualTLS {
		disableMutualTLS(&l)
	}
	return l
}

func EdgeListener(options types.RouterOptions) Listener {
	l := Listener{
		Name:             "edge-listener",
		Role:             RoleEdge,
		Port:             types.EdgeListenerPort,
		SslProfile:       types.InterRouterProfile,
		SaslMechanisms:   "EXTERNAL",
		AuthenticatePeer: true,
		MaxFrameSize:     options.MaxFrameSize,
		MaxSessionFrames: options.MaxSessionFrames,
	}
	if options.DisableMutualTLS {
		disableMutualTLS(&l)
	}
	return l
}

func GetInterRouterOrEdgeConnection(host string, connections []Connection) *Connection {
	for _, c := range connections {
		if (c.Role == "inter-router" || c.Role == "edge") && c.Host == host {
			return &c
		}
	}
	return nil
}

func GetLinkStatus(s *corev1.Secret, edge bool, connections []Connection) types.LinkStatus {
	link := types.LinkStatus{
		Name: s.ObjectMeta.Name,
	}
	if s.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeClaimRequest {
		link.Url = s.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey]
		if desc, ok := s.ObjectMeta.Annotations[types.StatusAnnotationKey]; ok {
			link.Description = "Failed to redeem claim: " + desc
		}
		link.Configured = false
	} else {
		if edge {
			link.Url = fmt.Sprintf("%s:%s", s.ObjectMeta.Annotations["edge-host"], s.ObjectMeta.Annotations["edge-port"])
		} else {
			link.Url = fmt.Sprintf("%s:%s", s.ObjectMeta.Annotations["inter-router-host"], s.ObjectMeta.Annotations["inter-router-port"])
		}
		link.Configured = true
		if connection := GetInterRouterOrEdgeConnection(link.Url, connections); connection != nil && connection.Active {
			link.Connected = true
			link.Cost, _ = strconv.Atoi(s.ObjectMeta.Annotations[types.TokenCost])
			link.Created = s.ObjectMeta.CreationTimestamp.String()
		}
		if s.ObjectMeta.Labels[types.SkupperDisabledQualifier] == "true" {
			link.Description = "Destination host is not allowed"
		}
	}
	return link
}

type ConfigUpdate interface {
	Apply(config *RouterConfig) bool
}
