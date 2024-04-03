package site

import (
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type LinkAccessMap map[string]*skupperv1alpha1.LinkAccess

type LinkAccessChanges struct {
	changed map[string]qdr.Listener
	deleted []string
}

func (m LinkAccessMap) Apply(config *qdr.RouterConfig) bool {
	lac := changes(extractActual(config), m.asRouterListeners())
	//TODO: update SslProfiles also
	return lac.Apply(config)
}

func (m LinkAccessMap) asRouterListeners() map[string]qdr.Listener {
	desired := map[string]qdr.Listener{}
	for _, la := range m {
		asRouterListeners(la, desired)
	}
	return desired
}

func changes(actual map[string]qdr.Listener, desired map[string]qdr.Listener) LinkAccessChanges {
	changes := LinkAccessChanges {
		changed: map[string]qdr.Listener{},
	}
	for key, desiredValue := range desired {
		if actualValue, ok := actual[key]; !ok || actualValue != desiredValue {
			changes.changed[key] = desiredValue
		}
	}
	for key, _ := range actual {
		if _, ok := desired[key]; !ok {
			changes.deleted = append(changes.deleted, key)
		}
	}
	return changes
}

func (lac *LinkAccessChanges) Apply(config *qdr.RouterConfig) bool {
	if len(lac.changed) == 0 && len(lac.deleted) == 0 {
		return false
	}
	for key, value := range lac.changed {
		config.Listeners[key] = value
	}
	for _, key := range lac.deleted {
		delete(config.Listeners, key)
	}
	return true
}

func extractActual(config *qdr.RouterConfig) map[string]qdr.Listener {
	actual := map[string]qdr.Listener{}
	for key, listener := range config.Listeners {
		if listener.Role == qdr.RoleInterRouter || listener.Role == qdr.RoleEdge {
			actual[key] = listener
		}
	}
	return actual
}

func asRouterListeners(la *skupperv1alpha1.LinkAccess, listeners map[string]qdr.Listener) {
	for _, role := range la.Spec.Roles {
		name := la.Name + "-" + role.Role
		listeners[name] = qdr.Listener{
			Name:             name,
			Role:             qdr.GetRole(role.Role),
			Port:             getPort(role),
			SslProfile:       la.Spec.TlsCredentials,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
		}
	}
}

func getPort(role skupperv1alpha1.LinkAccessRole) int32 {
	if role.Port != 0 {
		return int32(role.Port)
	} else if role.Role == "edge" {
		return 45671
	} else {
		return 55671
	}
}
