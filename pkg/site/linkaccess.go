package site

import (
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type LinkAccessMap map[string]*skupperv1alpha1.LinkAccess

type LinkAccessChanges struct {
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

func (m LinkAccessMap) Apply(config *qdr.RouterConfig) bool {
	return m.ApplyWithSslProfilePath(config, "/etc/skupper-router-certs")
}

func (m LinkAccessMap) ApplyWithSslProfilePath(config *qdr.RouterConfig, sslProfilePath string) bool {
	lac := changes(config.GetMatchingListeners(qdr.IsNotNormalListener), m.asRouterListeners())
	return lac.ApplyWithSslProfilePath(config, sslProfilePath)
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
		listeners: ListenerChanges {
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

func (lac *LinkAccessChanges) Apply(config *qdr.RouterConfig) bool {
	return lac.ApplyWithSslProfilePath(config, "/etc/skupper-router-certs")
}

func (lac *LinkAccessChanges) ApplyWithSslProfilePath(config *qdr.RouterConfig, sslProfilePath string) bool {
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
		config.AddSslProfileWithPath(sslProfilePath, qdr.SslProfile{Name: name})
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

func asRouterListeners(la *skupperv1alpha1.LinkAccess, listeners map[string]qdr.Listener) {
	for _, role := range la.Spec.Roles {
		name := la.Name + "-" + role.Role
		listeners[name] = qdr.Listener{
			Name:             name,
			Role:             qdr.GetRole(role.Role),
			Host:             la.Spec.BindHost,
			Port:             getPort(role),
			SslProfile:       la.Spec.TlsCredentials,
			SaslMechanisms:   "EXTERNAL",
			AuthenticatePeer: true,
			// TODO MaxFrameSize and MaxSessionFrames
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
