package site

import (
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
	proxyConfig    ProxyConfig
	definition     *skupperv2alpha1.Link
}

func NewLink(name string, sslProfilePath string, proxyConfig ProxyConfig) *Link {
	return &Link{
		name:           name,
		sslProfilePath: sslProfilePath,
		proxyConfig:    proxyConfig,
	}
}

func (l *Link) Apply(current *qdr.RouterConfig) bool {
	if l.definition == nil {
		return false
	}
	role := qdr.RoleInterRouter
	if current.IsEdge() {
		role = qdr.RoleEdge
	}
	endpoint, ok := l.definition.Spec.GetEndpointForRole(string(role))
	if !ok {
		return false
	}
	sslProfileName := sslProfileName(l.definition)
	proxyProfileName := proxyProfileName(l.definition)
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
	current.AddProxyProfile(qdr.ConfigureProxyProfile(proxyProfileName, l.proxyConfig.Host, l.proxyConfig.Port, l.proxyConfig.User, l.proxyConfig.ProfilePath))
	return true //TODO: optimise by indicating if no change was actually needed
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

func (o *RemoveConnector) Apply(current *qdr.RouterConfig) bool {
	if changed, connector := current.RemoveConnector(o.name); changed {
		unreferenced := current.UnreferencedSslProfiles()
		if _, ok := unreferenced[connector.SslProfile]; ok {
			current.RemoveSslProfile(connector.SslProfile)
		}
		return true
	}
	return false
}

func NewRemoveConnector(name string) qdr.ConfigUpdate {
	return &RemoveConnector{
		name: name,
	}
}
