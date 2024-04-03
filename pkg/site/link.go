package site

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func getHostPorts(lc *skupperv1alpha1.Link) map[qdr.Role]HostPort {
	hostPorts := map[qdr.Role]HostPort{}
	hostPorts[qdr.RoleEdge] = HostPort{
		host: lc.Spec.Edge.Host,
		port: strconv.Itoa(lc.Spec.Edge.Port),
	}
	hostPorts[qdr.RoleInterRouter] = HostPort{
		host: lc.Spec.InterRouter.Host,
		port: strconv.Itoa(lc.Spec.InterRouter.Port),
	}
	return hostPorts
}


type HostPort struct {
	host string
	port string
}

func (o *HostPort) url() string {
	return fmt.Sprintf("%s:%s", o.host, o.port)
}

func (o *HostPort) defined() bool {
	return o.host != "" && o.port != ""
}

type Link struct {
	name          string
	cost          int32
	hostPorts     map[qdr.Role]HostPort
	hasClientCert bool
	url           string
}

func NewLink(name string) *Link {
	return &Link{
		name: name,
	}
}

func (l *Link) Apply(current *qdr.RouterConfig) bool {
	profile := qdr.SslProfile {
		Name: sslProfileName(l.name),
	}
	role := qdr.RoleInterRouter
	if current.IsEdge() {
		role = qdr.RoleEdge
	}
	hostPort := l.hostPorts[role]
	if hostPort.defined() {
		l.url = hostPort.url()
	}
	connector := qdr.Connector {
		Name:       l.name,
		Cost:       l.cost,
		SslProfile: profile.Name,
		Role:       role,
		Host:       hostPort.host,
		Port:       hostPort.port,
	}
	//TODO: ????
	//connector.SetMaxFrameSize(siteConfig.Spec.Router.MaxFrameSize)
	//connector.SetMaxSessionFrames(siteConfig.Spec.Router.MaxSessionFrames)
	current.AddConnector(connector)
	if l.hasClientCert {
		current.AddSslProfile(profile)
	} else {
		current.AddSimpleSslProfile(profile)
	}
	return true //TODO: optimise by indicating if no change was actually needed
}

func sslProfileName(connectorName string) string {
	return connectorName + "-profile"
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
				current.RemoveSslProfile(sslProfileName(connector.Name))
			}
		}
	}
	return true //TODO: can optimise by indicating if no change was required
}

func (config *Link) Update(lc *skupperv1alpha1.Link) bool {
	changed := false
	if int32(lc.Spec.Cost) != config.cost {
		config.cost = int32(lc.Spec.Cost)
		changed = true
	}
	if hasClientCert := !lc.Spec.NoClientAuth; hasClientCert != config.hasClientCert {
		config.hasClientCert = hasClientCert
		changed = true
	}
	if hostPorts := getHostPorts(lc); !reflect.DeepEqual(hostPorts, config.hostPorts) {
		config.hostPorts = hostPorts
		changed = true
	}
	return changed
}

func (config *Link) UpdateStatus(lc *skupperv1alpha1.Link) {
	lc.Status.Url = config.url
}

type RemoveConnector struct {
	name string
}

func (o *RemoveConnector) Apply(current *qdr.RouterConfig) bool {
	updated := false
	if changed, _ := current.RemoveConnector(o.name); changed {
		updated = true
	}
	if current.RemoveSslProfile(sslProfileName(o.name)) {
		updated = true
	}
	return updated
}

func NewRemoveConnector(name string) qdr.ConfigUpdate {
	return &RemoveConnector{
		name: name,
	}
}
