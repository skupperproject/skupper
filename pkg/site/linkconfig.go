package site

import (
	"log"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func getTokenCost(token *corev1.Secret) int32 {
	if token.ObjectMeta.Annotations == nil {
		return 0
	}
	if costString, ok := token.ObjectMeta.Annotations[types.TokenCost]; ok {
		cost, err := strconv.Atoi(costString)
		if err != nil {
			log.Printf("Ignoring invalid cost annotation %q in %s/%s", costString, token.ObjectMeta.Namespace, token.ObjectMeta.Name)
			return 0
		}
		return int32(cost)
	}
	return 0
}

func getHostPorts(secret *corev1.Secret) map[qdr.Role]HostPort {
	hostPorts := map[qdr.Role]HostPort{}
	if secret.ObjectMeta.Annotations == nil {
		return hostPorts
	}
	hostPorts[qdr.RoleEdge] = HostPort{
		host: secret.ObjectMeta.Annotations["edge-host"],
		port: secret.ObjectMeta.Annotations["edge-port"],
	}
	hostPorts[qdr.RoleInterRouter] = HostPort{
		host: secret.ObjectMeta.Annotations["inter-router-host"],
		port: secret.ObjectMeta.Annotations["inter-router-port"],
	}
	return hostPorts
}


type HostPort struct {
	host string
	port string
}

type LinkConfig struct {
	name          string
	cost          int32
	hostPorts     map[qdr.Role]HostPort
	hasClientCert bool
}

func NewLinkConfig(name string) *LinkConfig {
	return &LinkConfig{
		name: name,
	}
}

func (l *LinkConfig) Apply(current *qdr.RouterConfig) bool {
	profile := qdr.SslProfile {
		Name: sslProfileName(l.name),
	}
	role := qdr.RoleInterRouter
	if current.IsEdge() {
		role = qdr.RoleEdge
	}
	hostPort := l.hostPorts[role]
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

type LinkConfigMap map[string]*LinkConfig

func (m LinkConfigMap) Apply(current *qdr.RouterConfig) bool {
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

func (config *LinkConfig) Update(secret *corev1.Secret) bool {
	changed := false
	if cost := getTokenCost(secret); cost != config.cost {
		config.cost = cost
		changed = true
	}
	if _, hasClientCert := secret.Data["tls.crt"]; hasClientCert != config.hasClientCert {
		config.hasClientCert = hasClientCert
		changed = true
	}
	if hostPorts := getHostPorts(secret); !reflect.DeepEqual(hostPorts, config.hostPorts) {
		config.hostPorts = hostPorts
		changed = true
	}
	return changed
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
