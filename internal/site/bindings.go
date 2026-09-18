package site

import (
	"net"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ListenerConfiguration func(siteId string, listener *skupperv2alpha1.Listener, config *qdr.BridgeConfig)
type ConnectorConfiguration func(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig)
type MultiKeyListenerConfiguration func(siteId string, mkl *skupperv2alpha1.MultiKeyListener, config *qdr.BridgeConfig)

type HostConnectorInfo struct {
	Name string
	Host string
	Port int
}

type ConnectorHealthChange struct {
	Name    string
	Host    string
	Port    int
	Healthy bool
}

type HealthChangeCallback func(changed []ConnectorHealthChange)

type BindingEventHandler interface {
	ListenerUpdated(listener *skupperv2alpha1.Listener)
	ListenerDeleted(listener *skupperv2alpha1.Listener)
	ConnectorUpdated(connector *skupperv2alpha1.Connector) bool
	ConnectorDeleted(connector *skupperv2alpha1.Connector)
}

type ConnectorFunction func(*skupperv2alpha1.Connector) *skupperv2alpha1.Connector
type ListenerFunction func(*skupperv2alpha1.Listener) *skupperv2alpha1.Listener
type MultiKeyListenerFunction func(*skupperv2alpha1.MultiKeyListener) *skupperv2alpha1.MultiKeyListener

type Bindings struct {
	SiteId             string
	ProfilePath        string
	connectors         map[string]*skupperv2alpha1.Connector
	listeners          map[string]*skupperv2alpha1.Listener
	multiKeyListeners  map[string]*skupperv2alpha1.MultiKeyListener
	handler            BindingEventHandler
	isTlsSecretPresent func(secretName string) bool
	configure          struct {
		listener         ListenerConfiguration
		connector        ConnectorConfiguration
		multiKeyListener MultiKeyListenerConfiguration
	}

	hostConnectors  map[string]HostConnectorInfo
	connectorHealth map[string]bool
	healthMu        sync.RWMutex
	stopCh          chan struct{}
	dialTimeout     func(network, address string, timeout time.Duration) (net.Conn, error)
	onHealthChange  HealthChangeCallback
}

func NewBindings(profilePath string) *Bindings {
	bindings := &Bindings{
		ProfilePath:       profilePath,
		connectors:        map[string]*skupperv2alpha1.Connector{},
		listeners:         map[string]*skupperv2alpha1.Listener{},
		multiKeyListeners: map[string]*skupperv2alpha1.MultiKeyListener{},
		hostConnectors:    map[string]HostConnectorInfo{},
		connectorHealth:   map[string]bool{},
		dialTimeout:       net.DialTimeout,
	}
	bindings.configure.listener = UpdateBridgeConfigForListener
	bindings.configure.connector = bindings.UpdateBridgeConfigForConnector
	bindings.configure.multiKeyListener = UpdateBridgeConfigForMultiKeyListener
	return bindings
}

func (b *Bindings) SetSiteId(siteId string) {
	b.SiteId = siteId
}

func (b *Bindings) SetIsTlsSecretPresent(f func(string) bool) {
	b.isTlsSecretPresent = f
}

// IsTlsSecretPresent reports whether TLS material for the given secret name is available
// (e.g. in the ProfilesWatcher cache). Empty name always returns true.
func (b *Bindings) IsTlsSecretPresent(secretName string) bool {
	if secretName == "" {
		return true
	}
	if b.isTlsSecretPresent == nil {
		return true
	}
	return b.isTlsSecretPresent(secretName)
}

func (b *Bindings) SetListenerConfiguration(configuration ListenerConfiguration) {
	b.configure.listener = configuration
}

func (b *Bindings) SetConnectorConfiguration(configuration ConnectorConfiguration) {
	b.configure.connector = configuration
}

func (b *Bindings) SetBindingEventHandler(handler BindingEventHandler) {
	b.handler = handler
	for _, c := range b.connectors {
		b.handler.ConnectorUpdated(c)
	}
	for _, l := range b.listeners {
		b.handler.ListenerUpdated(l)
	}
}

func (b *Bindings) Map(cf ConnectorFunction, lf ListenerFunction) {
	if cf != nil {
		for key, connector := range b.connectors {
			if updated := cf(connector); updated != nil {
				b.connectors[key] = updated
			}
		}
	}
	if lf != nil {
		for key, listener := range b.listeners {
			if updated := lf(listener); updated != nil {
				b.listeners[key] = updated
			}
		}
	}
}

func (b *Bindings) MapOverMultiKeyListeners(mkf MultiKeyListenerFunction) {
	if mkf != nil {
		for key, mkl := range b.multiKeyListeners {
			if updated := mkf(mkl); updated != nil {
				b.multiKeyListeners[key] = updated
			}
		}
	}
}

func (b *Bindings) GetConnector(name string) *skupperv2alpha1.Connector {
	if existing, ok := b.connectors[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) ConnectorNames() []string {
	names := make([]string, 0, len(b.connectors))
	for name := range b.connectors {
		names = append(names, name)
	}
	return names
}

func (b *Bindings) GetListener(name string) *skupperv2alpha1.Listener {
	if existing, ok := b.listeners[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) ListenerNames() []string {
	names := make([]string, 0, len(b.listeners))
	for name := range b.listeners {
		names = append(names, name)
	}
	return names
}

func (b *Bindings) MultiKeyListenerNames() []string {
	names := make([]string, 0, len(b.multiKeyListeners))
	for name := range b.multiKeyListeners {
		names = append(names, name)
	}
	return names
}

func (b *Bindings) UpdateConnector(name string, connector *skupperv2alpha1.Connector) qdr.ConfigUpdate {
	if connector == nil {
		return b.deleteConnector(name)
	}
	return b.updateConnector(connector)
}

func (b *Bindings) updateConnector(connector *skupperv2alpha1.Connector) qdr.ConfigUpdate {
	name := connector.ObjectMeta.Name
	existing, ok := b.connectors[name]
	b.connectors[name] = connector // always update pointer, even if spec has not changed

	if connector.Spec.Host != "" && connector.Spec.Port != 0 {
		b.healthMu.Lock()
		if b.hostConnectors == nil {
			b.hostConnectors = map[string]HostConnectorInfo{}
		}
		if b.connectorHealth == nil {
			b.connectorHealth = map[string]bool{}
		}
		b.hostConnectors[name] = HostConnectorInfo{
			Name: name,
			Host: connector.Spec.Host,
			Port: connector.Spec.Port,
		}
		if _, exists := b.connectorHealth[name]; !exists {
			b.connectorHealth[name] = true
		}
		b.healthMu.Unlock()
	} else {
		b.healthMu.Lock()
		if b.hostConnectors != nil {
			delete(b.hostConnectors, name)
		}
		if b.connectorHealth != nil {
			delete(b.connectorHealth, name)
		}
		b.healthMu.Unlock()
	}

	if ok && reflect.DeepEqual(existing.Spec, connector.Spec) {
		return nil
	}
	if b.handler == nil || b.handler.ConnectorUpdated(connector) {
		return b
	}
	return nil
}

func (b *Bindings) deleteConnector(name string) qdr.ConfigUpdate {
	if existing, ok := b.connectors[name]; ok {
		delete(b.connectors, name)
		b.healthMu.Lock()
		if b.hostConnectors != nil {
			delete(b.hostConnectors, name)
		}
		if b.connectorHealth != nil {
			delete(b.connectorHealth, name)
		}
		b.healthMu.Unlock()
		if b.handler != nil {
			b.handler.ConnectorDeleted(existing)
		}
		return b
	}
	return nil
}

func (b *Bindings) UpdateListener(name string, listener *skupperv2alpha1.Listener) qdr.ConfigUpdate {
	if listener == nil {
		return b.deleteListener(name)
	}
	return b.updateListener(listener)
}

func (b *Bindings) updateListener(latest *skupperv2alpha1.Listener) qdr.ConfigUpdate {
	name := latest.ObjectMeta.Name
	existing, ok := b.listeners[name]
	b.listeners[name] = latest

	if !ok || !reflect.DeepEqual(existing.Spec, latest.Spec) {
		if b.handler != nil {
			b.handler.ListenerUpdated(latest)
		}
		return b
	}
	return nil
}

func (b *Bindings) deleteListener(name string) qdr.ConfigUpdate {
	if existing, ok := b.listeners[name]; ok {
		delete(b.listeners, name)
		if b.handler != nil {
			b.handler.ListenerDeleted(existing)
		}
		return b
	}
	return nil
}

func (b *Bindings) GetMultiKeyListener(name string) *skupperv2alpha1.MultiKeyListener {
	if existing, ok := b.multiKeyListeners[name]; ok {
		return existing
	}
	return nil
}

func (b *Bindings) UpdateMultiKeyListener(name string, mkl *skupperv2alpha1.MultiKeyListener) qdr.ConfigUpdate {
	if mkl == nil {
		return b.deleteMultiKeyListener(name)
	}
	return b.updateMultiKeyListener(mkl)
}

func (b *Bindings) updateMultiKeyListener(mkl *skupperv2alpha1.MultiKeyListener) qdr.ConfigUpdate {
	name := mkl.ObjectMeta.Name
	existing, ok := b.multiKeyListeners[name]
	b.multiKeyListeners[name] = mkl
	if ok && reflect.DeepEqual(existing.Spec, mkl.Spec) {
		return nil
	}

	if mkl.Spec.Strategy.Weighted != nil && mkl.Status.Strategy != nil && mkl.Status.Strategy.Weighted != nil {
		// update weight values in status if they changed in the spec
		for k, w := range mkl.Status.Strategy.Weighted.RoutingKeysReachable {
			if ws, ok := mkl.Spec.Strategy.Weighted.RoutingKeys[k]; ok {
				if w != ws {
					mkl.Status.Strategy.Weighted.RoutingKeysReachable[k] = ws
				}
			} else {
				delete(mkl.Status.Strategy.Weighted.RoutingKeysReachable, k)
			}
		}
	}
	return b
}

func (b *Bindings) deleteMultiKeyListener(name string) qdr.ConfigUpdate {
	if _, ok := b.multiKeyListeners[name]; !ok {
		return nil
	}
	delete(b.multiKeyListeners, name)
	return b
}

func (b *Bindings) SetMultiKeyListenerConfiguration(configuration MultiKeyListenerConfiguration) {
	b.configure.multiKeyListener = configuration
}

func (b *Bindings) ToBridgeConfig() qdr.BridgeConfig {
	config := qdr.BridgeConfig{
		TcpListeners:      qdr.TcpEndpointMap{},
		TcpConnectors:     qdr.TcpEndpointMap{},
		ListenerAddresses: qdr.ListenerAddressMap{},
	}
	for _, c := range b.connectors {
		if c.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(c.Spec.TlsCredentials) {
			continue
		}
		b.configure.connector(b.SiteId, c, &config)
	}
	for _, l := range b.listeners {
		if l.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(l.Spec.TlsCredentials) {
			continue
		}
		b.configure.listener(b.SiteId, l, &config)
	}
	for _, mkl := range b.multiKeyListeners {
		if mkl.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(mkl.Spec.TlsCredentials) {
			continue
		}
		b.configure.multiKeyListener(b.SiteId, mkl, &config)
	}

	return config
}

func (b *Bindings) AddSslProfiles(config *qdr.RouterConfig) bool {
	profiles := map[string]qdr.SslProfile{}
	for _, c := range b.connectors {
		if c.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(c.Spec.TlsCredentials) {
			continue
		}
		if c.Spec.TlsCredentials != "" {
			if !c.Spec.UseClientCert {
				//if only ca is used, need to qualify the profile to ensure that it does not collide with
				// use of the same secret where client auth *is* required
				name := GetSslProfileName(c.Spec.TlsCredentials, c.Spec.UseClientCert)
				if _, ok := profiles[name]; !ok {
					profiles[name] = qdr.ConfigureSslProfile(name, b.ProfilePath, false)
				}
			} else {
				if _, ok := profiles[c.Spec.TlsCredentials]; !ok {
					profiles[c.Spec.TlsCredentials] = qdr.ConfigureSslProfile(c.Spec.TlsCredentials, b.ProfilePath, true)
				}
			}
		}
	}
	for _, l := range b.listeners {
		if l.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(l.Spec.TlsCredentials) {
			continue
		}
		if _, ok := profiles[l.Spec.TlsCredentials]; l.Spec.TlsCredentials != "" && !ok {
			profiles[l.Spec.TlsCredentials] = qdr.ConfigureSslProfile(l.Spec.TlsCredentials, b.ProfilePath, true)
		}
	}
	for _, mkl := range b.multiKeyListeners {
		if mkl.Spec.TlsCredentials != "" && !b.IsTlsSecretPresent(mkl.Spec.TlsCredentials) {
			continue
		}
		if _, ok := profiles[mkl.Spec.TlsCredentials]; mkl.Spec.TlsCredentials != "" && !ok {
			profiles[mkl.Spec.TlsCredentials] = qdr.ConfigureSslProfile(mkl.Spec.TlsCredentials, b.ProfilePath, true)
		}
	}
	changed := false
	for _, profile := range profiles {
		if config.AddSslProfile(profile) {
			changed = true
		}
	}
	return changed
}

func (b *Bindings) Apply(config *qdr.RouterConfig) bool {
	b.AddSslProfiles(config)
	config.UpdateBridgeConfig(b.ToBridgeConfig())
	config.RemoveUnreferencedSslProfiles()
	return true //TODO: can optimise by indicating if no change was required
}

func (b *Bindings) UpdateBridgeConfigForConnector(siteId string, connector *skupperv2alpha1.Connector, config *qdr.BridgeConfig) {
	if connector.Spec.Host != "" {
		if b.IsConnectorHealthy(connector.Name) {
			UpdateBridgeConfigForConnector(siteId, connector, config)
		}
	}
}

func (b *Bindings) SetHealthChangeCallback(cb HealthChangeCallback) {
	b.onHealthChange = cb
}

func (b *Bindings) IsConnectorHealthy(name string) bool {
	if b == nil {
		return true
	}
	b.healthMu.RLock()
	defer b.healthMu.RUnlock()
	if b.connectorHealth == nil {
		return true
	}
	if healthy, ok := b.connectorHealth[name]; ok {
		return healthy
	}
	return true
}

func (b *Bindings) StartHealthCheckLoop(interval time.Duration) {
	b.healthMu.Lock()
	if b.stopCh != nil {
		b.healthMu.Unlock()
		return
	}
	b.stopCh = make(chan struct{})
	stopCh := b.stopCh
	b.healthMu.Unlock()

	if interval <= 0 {
		interval = 5 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			b.PerformHealthChecks()
			select {
			case <-ticker.C:
			case <-stopCh:
				return
			}
		}
	}()
}

func (b *Bindings) Stop() {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	if b.stopCh != nil {
		select {
		case <-b.stopCh:
		default:
			close(b.stopCh)
		}
		b.stopCh = nil
	}
}

func (b *Bindings) PerformHealthChecks() []ConnectorHealthChange {
	b.healthMu.RLock()
	var connectorsToCheck []HostConnectorInfo
	for _, info := range b.hostConnectors {
		connectorsToCheck = append(connectorsToCheck, info)
	}
	b.healthMu.RUnlock()

	if len(connectorsToCheck) == 0 {
		return nil
	}

	resultsChan := make(chan ConnectorHealthChange, len(connectorsToCheck))
	var wg sync.WaitGroup
	for _, info := range connectorsToCheck {
		wg.Add(1)
		go func(inf HostConnectorInfo) {
			defer wg.Done()
			healthy := b.checkTarget(inf.Host, inf.Port)
			resultsChan <- ConnectorHealthChange{
				Name:    inf.Name,
				Host:    inf.Host,
				Port:    inf.Port,
				Healthy: healthy,
			}
		}(info)
	}
	wg.Wait()
	close(resultsChan)

	var changedConnectors []ConnectorHealthChange

	b.healthMu.Lock()
	for res := range resultsChan {
		if _, exists := b.hostConnectors[res.Name]; !exists {
			continue
		}
		prevHealthy, ok := b.connectorHealth[res.Name]
		if !ok || prevHealthy != res.Healthy {
			b.connectorHealth[res.Name] = res.Healthy
			changedConnectors = append(changedConnectors, res)
		}
	}
	b.healthMu.Unlock()

	if len(changedConnectors) > 0 && b.onHealthChange != nil {
		b.onHealthChange(changedConnectors)
	}

	return changedConnectors
}

func (b *Bindings) checkTarget(host string, port int) bool {
	address := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := b.dialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
