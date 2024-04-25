package site

import (
	"reflect"
	"strings"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type Link struct {
	name        string
	profilePath string
	definition  *skupperv1alpha1.Link
}

func NewLink(name string, profilePath string) *Link {
	return &Link{
		name:        name,
		profilePath: profilePath,
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
	profileName := sslProfileName(l.definition)
	connector := qdr.Connector{
		Name:       l.name,
		Cost:       int32(l.definition.Spec.Cost),
		SslProfile: profileName,
		Role:       role,
		Host:       endpoint.Host,
		Port:       endpoint.Port,
	}
	current.AddConnector(connector)
	current.AddSslProfile(qdr.ConfigureSslProfile(profileName, l.profilePath, !l.definition.Spec.NoClientAuth))
	return true //TODO: optimise by indicating if no change was actually needed
}

func sslProfileName(link *skupperv1alpha1.Link) string {
	return link.Spec.TlsCredentials + "-profile"
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

func (link *Link) Update(definition *skupperv1alpha1.Link) bool {
	if reflect.DeepEqual(link.definition, definition) {
		return false
	}
	link.definition = definition
	return true
}

func (config *Link) UpdateStatus(lc *skupperv1alpha1.Link) {
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
