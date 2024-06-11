package site

import (
	"strconv"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type RouterAccessMap map[string]*skupperv1alpha1.RouterAccess

type RouterAccessChanges struct {
	listeners ListenerChanges
	profiles  SslProfileChanges
}

type ListenerChanges struct {
	changed map[string]qdr.Listener
	deleted []string
}

type SslProfileChanges struct {
	added   []string
	deleted []string
}

type RouterAccessConfig struct {
	listeners  map[string]qdr.Listener
	connectors []qdr.Connector
}

func (g *RouterAccessConfig) Apply(config *qdr.RouterConfig) bool {
	lac := changes(config.GetMatchingListeners(qdr.IsNotNormalListener), g.listeners)
	// TODO: determine if there are any stale local connectors
	changed := false
	for _, connector := range g.connectors {
		if config.AddConnector(connector) {
			changed = true
		}
	}
	return lac.Apply(config) || changed
}

func (m RouterAccessMap) asRouterListeners() map[string]qdr.Listener {
	desired := map[string]qdr.Listener{}
	for _, ra := range m {
		asRouterListeners(ra, desired)
	}
	return desired
}

func (m RouterAccessMap) findInterRouterRole() (*skupperv1alpha1.RouterAccessRole, *skupperv1alpha1.RouterAccess) {
	for _, value := range m {
		if role := value.FindRole("inter-router"); role != nil {
			return role, value
		}
	}
	return nil, nil
}

func (c *RouterAccessConfig) getDesired(definitions RouterAccessMap, targetGroups []string) {
	for _, ra := range definitions {
		asRouterListeners(ra, c.listeners)
	}
	if len(targetGroups) > 0 {
		if role, ra := definitions.findInterRouterRole(); role != nil {
			for _, group := range targetGroups {
				name := group
				connector := qdr.Connector{
					Name:       name,
					Host:       group,
					Role:       qdr.RoleInterRouter,
					Port:       strconv.Itoa(role.Port),
					SslProfile: ra.Spec.TlsCredentials,
				}
				c.connectors = append(c.connectors, connector)
			}
		}
	}
}

func (m RouterAccessMap) getChanges(targetGroups []string) *RouterAccessConfig {
	desired := &RouterAccessConfig{
		listeners: map[string]qdr.Listener{},
	}
	desired.getDesired(m, targetGroups)
	return desired
}

func changes(actual map[string]qdr.Listener, desired map[string]qdr.Listener) RouterAccessChanges {
	changes := RouterAccessChanges{
		listeners: ListenerChanges{
			changed: map[string]qdr.Listener{},
		},
	}
	for key, desiredValue := range desired {
		if actualValue, ok := actual[key]; !ok || actualValue != desiredValue {
			changes.listeners.changed[key] = desiredValue
			changes.profiles.added = append(changes.profiles.added, desiredValue.SslProfile)
		}
	}
	for key, stale := range actual {
		if _, ok := desired[key]; !ok {
			changes.listeners.deleted = append(changes.listeners.deleted, key)
			changes.profiles.deleted = append(changes.profiles.deleted, stale.SslProfile)
		}
	}
	return changes
}

func (lac *RouterAccessChanges) Apply(config *qdr.RouterConfig) bool {
	if len(lac.listeners.changed) == 0 && len(lac.listeners.deleted) == 0 {
		return false
	}
	for key, value := range lac.listeners.changed {
		config.Listeners[key] = value
	}
	for _, key := range lac.listeners.deleted {
		delete(config.Listeners, key)
	}
	for _, name := range lac.profiles.added {
		config.AddSslProfileWithPath("/etc/skupper-router-certs", qdr.SslProfile{Name: name})
	}
	// SslProfiles may be shared, so only delete those that are now unreferenced
	unreferenced := config.UnreferencedSslProfiles()
	for _, name := range lac.profiles.deleted {
		if _, ok := unreferenced[name]; ok {
			config.RemoveSslProfile(name)
		}
	}
	return true
}

func asRouterListeners(la *skupperv1alpha1.RouterAccess, listeners map[string]qdr.Listener) {
	for _, role := range la.Spec.Roles {
		name := la.Name + "-" + role.Name
		listeners[name] = qdr.Listener{
			Name:             name,
			Role:             qdr.GetRole(role.Name),
			Port:             int32(getPort(role)),
			SslProfile:       la.Spec.TlsCredentials,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		}
	}
}

func getPort(role skupperv1alpha1.RouterAccessRole) int {
	if role.Port != 0 {
		return role.Port
	} else if role.Name == "edge" {
		return 45671
	} else {
		return 55671
	}
}
