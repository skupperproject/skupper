package site

import (
	"fmt"
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
				Port:             ra.GetPortForRole(role.Name),
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
				Port:       strconv.Itoa(int(ra.GetPortForRole(role.Name))),
				SslProfile: ra.Spec.TlsCredentials,
				Cost:       1,
			}
			connectors = append(connectors, connector)
		}
	}
	return connectors
}

func (m RouterAccessMap) desiredAutoLinks() map[string]qdr.AutoLink {
	var autoLinks = map[string]qdr.AutoLink{}
	for raName, ra := range m {
		if ra.FindRole(qdr.RoleInterNetwork) == nil {
			continue
		}
		for _, routingKey := range ra.Spec.RoutingKeys {
			autoLinkName := fmt.Sprintf("routerAccess/%s/%s", raName, routingKey)
			autoLinks[autoLinkName] = qdr.AutoLink{
				Name:       autoLinkName,
				Address:    routingKey,
				Direction:  qdr.DirectionIn,
				Connection: fmt.Sprintf("%s-inter-network", raName),
			}
		}
	}
	return autoLinks
}

func (m RouterAccessMap) findInterRouterRole() (*skupperv2alpha1.RouterAccessRole, *skupperv2alpha1.RouterAccess) {
	for _, value := range m {
		if role := value.FindRole("inter-router"); role != nil {
			return role, value
		}
	}
	return nil, nil
}

func (m RouterAccessMap) HasPortConflict(ra *skupperv2alpha1.RouterAccess) (bool, string, int) {
	var usedPorts = map[int32]string{}
	for _, cur := range m {
		for _, curRole := range cur.Spec.Roles {
			if port := cur.GetPortForRole(curRole.Name); port != 0 {
				usedPorts[port] = cur.Name
			}
		}
	}
	for _, role := range ra.Spec.Roles {
		if role.Port == 0 {
			continue
		}
		if name, ok := usedPorts[int32(role.Port)]; ok && name != ra.Name {
			return true, fmt.Sprintf("router access: %s", name), role.Port
		}
	}
	return false, "", 0
}

func (m RouterAccessMap) DesiredConfig(targetGroups []string, profilePath string) *RouterAccessConfig {
	return m.DesiredConfigWithAvailableCredentials(targetGroups, profilePath, nil)
}

// DesiredConfigWithAvailableCredentials is like DesiredConfig but skips RouterAccess entries whose
// spec.tlsCredentials is set when isTlsSecretPresent is non-nil and returns false for that name.
func (m RouterAccessMap) DesiredConfigWithAvailableCredentials(targetGroups []string, profilePath string, isTlsSecretPresent func(string) bool) *RouterAccessConfig {
	source := m
	if isTlsSecretPresent != nil {
		source = make(RouterAccessMap, len(m))
		for k, ra := range m {
			if ra.Spec.TlsCredentials != "" && !isTlsSecretPresent(ra.Spec.TlsCredentials) {
				continue
			}
			source[k] = ra
		}
	}
	return &RouterAccessConfig{
		listeners:   source.desiredListeners(),
		connectors:  source.desiredConnectors(targetGroups),
		autoLinks:   source.desiredAutoLinks(),
		profilePath: profilePath,
	}
}

type RouterAccessConfig struct {
	listeners   map[string]qdr.Listener
	connectors  []qdr.Connector
	autoLinks   map[string]qdr.AutoLink
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
		if config.AddListener(value) {
			config.AddSslProfile(qdr.ConfigureSslProfile(value.SslProfile, g.profilePath, true))
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
	// Check if networkId related autoLinks are needed for inter-network listeners
	if config.Network.IsSet() {
		for listenerName := range qdr.FilterListeners(config.Listeners, qdr.IsInterVANListener) {
			al := AutoLinkForListener(listenerName, config.Network.NetworkId)
			g.autoLinks[al.Name] = al
		}
	}
	// Update listener related autoLinks
	autoLinksDiff := qdr.AutoLinksDifference(qdr.FilterAutoLinks(config.AutoLinks, qdr.FilterAutoLinkListeners), g.autoLinks)
	for _, ld := range autoLinksDiff.Deleted {
		if config.RemoveAutoLink(ld.Name) {
			changed = true
		}
	}
	for _, la := range autoLinksDiff.Added {
		if config.AddAutoLink(la) {
			changed = true
		}
	}
	return changed
}

func AutoLinkForListener(listenerName, networkId string) qdr.AutoLink {
	autoLinkName := fmt.Sprintf("routerAccess/%s", listenerName)
	return qdr.AutoLink{
		Name:            autoLinkName,
		ExternalAddress: fmt.Sprintf("_xtopo/%s", networkId),
		Direction:       qdr.DirectionIn,
		Connection:      listenerName,
	}
}
