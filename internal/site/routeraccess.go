package site

import (
	"strconv"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type RouterAccessMap map[string]*skupperv2alpha1.RouterAccess

func (m RouterAccessMap) desiredListeners() map[string]qdr.Listener {
	desired := map[string]qdr.Listener{}
	for _, ra := range m {
		for _, role := range ra.Spec.Roles {
			name := ra.Name + "-" + role.Name
			desired[name] = qdr.Listener{
				Name:             name,
				Role:             qdr.GetRole(role.Name),
				Host:             ra.Spec.BindHost,
				Port:             role.GetPort(),
				SslProfile:       ra.Spec.TlsCredentials,
				SaslMechanisms:   "EXTERNAL",
				AuthenticatePeer: true,
			}
		}
	}
	return desired
}

func (m RouterAccessMap) desiredConnectors(targetGroups []string) []qdr.Connector {
	if len(targetGroups) == 0 {
		return nil
	}
	var connectors []qdr.Connector
	if role, ra := m.findInterRouterRole(); role != nil {
		for _, group := range targetGroups {
			name := group
			connector := qdr.Connector{
				Name:       name,
				Host:       group,
				Role:       qdr.RoleInterRouter,
				Port:       strconv.Itoa(role.Port),
				SslProfile: ra.Spec.TlsCredentials,
				Cost:       1,
			}
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func (m RouterAccessMap) findInterRouterRole() (*skupperv2alpha1.RouterAccessRole, *skupperv2alpha1.RouterAccess) {
	for _, value := range m {
		if role := value.FindRole("inter-router"); role != nil {
			return role, value
		}
	}
	return nil, nil
}

func (m RouterAccessMap) DesiredConfig(targetGroups []string, profilePath string) *RouterAccessConfig {
	return &RouterAccessConfig{
		listeners:   m.desiredListeners(),
		connectors:  m.desiredConnectors(targetGroups),
		profilePath: profilePath,
	}
}

type RouterAccessConfig struct {
	listeners   map[string]qdr.Listener
	connectors  []qdr.Connector
	profilePath string
}

func (g *RouterAccessConfig) Apply(config *qdr.RouterConfig) bool {
	changed := false
	lc := qdr.ListenersDifference(config.GetMatchingListeners(qdr.IsNotProtectedListener), g.listeners)
	// delete before add with listeners, as changes are handled as delete and add
	for _, value := range lc.Deleted {
		if removed, _ := config.RemoveListener(value.Name); removed {
			delete(config.Listeners, value.Name)
			changed = true
		}
	}
	for _, value := range lc.Added {
		if config.AddListener(value) && config.AddSslProfile(qdr.ConfigureSslProfile(value.SslProfile, g.profilePath, true)) {
			changed = true
		}
	}
	for _, connector := range g.connectors {
		if config.AddConnector(connector) {
			changed = true
		}
	}
	// SslProfiles may be shared, so only delete those that are now unreferenced
	for name, _ := range config.UnreferencedSslProfiles() {
		config.RemoveSslProfile(name)
		changed = true
	}
	return changed
}
