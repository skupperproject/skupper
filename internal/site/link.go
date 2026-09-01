package site

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ProxyConfig struct {
	Host        string
	Port        string
	User        string
	ProfilePath string
}

type Link struct {
	name           string
	sslProfilePath string
	proxyConfig    *ProxyConfig
	definition     *skupperv2alpha1.Link
}

func NewLink(name string, sslProfilePath string, proxyConfig *ProxyConfig) *Link {
	return &Link{
		name:           name,
		sslProfilePath: sslProfilePath,
		proxyConfig:    proxyConfig,
	}
}

func (l *Link) UpdateProxyConfig(proxyConfig *ProxyConfig) bool {
	if l.definition == nil {
		return false
	}
	l.proxyConfig = proxyConfig
	return true
}

func (l *Link) Apply(current *qdr.RouterConfig) bool {
	if l.definition == nil {
		return false
	}
	var role qdr.Role
	if !l.definition.IsInterVAN() {
		role = qdr.RoleInterRouter
		if current.IsEdge() {
			role = qdr.RoleEdge
		}
	} else {
		role = qdr.RoleInterNetwork
	}
	endpoint, ok := l.definition.Spec.GetEndpointForRole(string(role))
	if !ok {
		return false
	}
	sslProfileName := sslProfileName(l.definition)
	proxyProfileName := proxyProfileName(l.definition)
	prevProxyProfileName := current.Connectors[l.name].ProxyProfile
	cost := int32(l.definition.Spec.Cost)
	if cost < 1 {
		cost = 1
	}
	connector := qdr.Connector{
		Name:         l.name,
		Cost:         cost,
		SslProfile:   sslProfileName,
		ProxyProfile: proxyProfileName,
		Role:         role,
		Host:         endpoint.Host,
		Port:         endpoint.Port,
	}
	current.AddConnector(connector)
	current.AddSslProfile(qdr.ConfigureSslProfile(sslProfileName, l.sslProfilePath, true))
	if proxyProfileName != "" {
		current.AddProxyProfile(qdr.ConfigureProxyProfile(proxyProfileName, l.proxyConfig.Host, l.proxyConfig.Port, l.proxyConfig.User, l.proxyConfig.ProfilePath))
		if prevProxyProfileName != "" && prevProxyProfileName != proxyProfileName {
			current.RemoveProxyProfile(prevProxyProfileName)
		}
	} else if prevProxyProfileName != "" {
		current.RemoveProxyProfile(prevProxyProfileName)
	}
	if l.definition.IsInterVAN() {
		desired := l.desiredAutoLinks(current.Network.NetworkId)
		diff := qdr.AutoLinksDifference(qdr.FilterAutoLinks(current.AutoLinks, connector.FilterAutoLinks), desired)
		for _, autoLinkDel := range diff.Deleted {
			current.RemoveAutoLink(autoLinkDel.Name)
		}
		for _, autoLinkAdd := range diff.Added {
			current.AddAutoLink(autoLinkAdd)
		}
	}
	return true //TODO: optimise by indicating if no change was actually needed
}

func (l *Link) desiredAutoLinks(networkId string) map[string]qdr.AutoLink {
	var res = map[string]qdr.AutoLink{}
	if networkId != "" {
		al := AutoLinkForConnector(l.name, networkId)
		res[al.Name] = al
	}
	for _, routingKey := range l.definition.Spec.RoutingKeys {
		autoLinkName := fmt.Sprintf("link/%s/%s", l.name, routingKey)
		res[autoLinkName] = qdr.AutoLink{
			Name:       autoLinkName,
			Address:    routingKey,
			Direction:  qdr.DirectionIn,
			Connection: l.name,
		}
	}
	return res
}

func AutoLinkForConnector(connectorName, networkId string) qdr.AutoLink {
	autoLinkName := fmt.Sprintf("link/%s", connectorName)
	return qdr.AutoLink{
		Name:            autoLinkName,
		ExternalAddress: "_xtopo/" + networkId,
		Direction:       qdr.DirectionIn,
		Connection:      connectorName,
	}
}

func sslProfileName(link *skupperv2alpha1.Link) string {
	return link.Spec.TlsCredentials + "-profile"
}

func proxyProfileName(link *skupperv2alpha1.Link) string {
	return link.Spec.GetProxyConfiguration()
}

type LinkMap map[string]*Link

func (m LinkMap) Apply(current *qdr.RouterConfig) bool {
	for _, config := range m {
		config.Apply(current)
	}
	for _, connector := range current.Connectors {
		if !strings.HasPrefix(connector.Name, "auto-mesh") {
			if _, ok := m[connector.Name]; !ok {
				current.RemoveConnector(connector.Name)
				current.RemoveSslProfile(connector.SslProfile)
				current.RemoveProxyProfile(connector.ProxyProfile)
				for name := range qdr.FilterAutoLinks(current.AutoLinks, connector.FilterAutoLinks) {
					current.RemoveAutoLink(name)
				}
			}
		}
	}
	return true //TODO: can optimise by indicating if no change was required
}

func (link *Link) Update(definition *skupperv2alpha1.Link) bool {
	changed := !reflect.DeepEqual(link.definition, definition)
	link.definition = definition
	return changed
}

func (link *Link) Definition() *skupperv2alpha1.Link {
	return link.definition
}

type RemoveConnector struct {
	name string
}

func (o *RemoveConnector) filterAutoLink(autoLink qdr.AutoLink) bool {
	return autoLink.Name == fmt.Sprintf("link/%s", o.name) ||
		strings.HasPrefix(autoLink.Name, fmt.Sprintf("link/%s/", o.name))
}

func (o *RemoveConnector) Apply(current *qdr.RouterConfig) bool {
	var changed bool
	var connector qdr.Connector
	if changed, connector = current.RemoveConnector(o.name); changed {
		unreferenced := current.UnreferencedSslProfiles()
		if _, ok := unreferenced[connector.SslProfile]; ok {
			current.RemoveSslProfile(connector.SslProfile)
		}
		unreferencedProxyProfiles := current.UnreferencedProxyProfiles()
		if _, ok := unreferencedProxyProfiles[connector.ProxyProfile]; ok {
			current.RemoveProxyProfile(connector.ProxyProfile)
		}
	}
	for name := range qdr.FilterAutoLinks(current.AutoLinks, o.filterAutoLink) {
		if current.RemoveAutoLink(name) {
			changed = true
		}
	}
	return changed
}

func NewRemoveConnector(name string) qdr.ConfigUpdate {
	return &RemoveConnector{
		name: name,
	}
}
