package site

import (
	"fmt"
	"log"
	"strings"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
)

type BindingStatus struct {
	connectors map[string][]string
	listeners  map[string][]string
	client     internalclient.Clients
	errors     []string
}

func newBindingStatus(client internalclient.Clients, network []skupperv1alpha1.SiteRecord) *BindingStatus {
	s := &BindingStatus{
		client:     client,
		connectors: map[string][]string{},
		listeners:  map[string][]string{},
	}
	s.populate(network)
	return s
}

func (s *BindingStatus) populate(network []skupperv1alpha1.SiteRecord) {
	for _, site := range network {
		for _, svc := range site.Services {
			connectors := s.connectors[svc.RoutingKey]
			for _, connector := range svc.Connectors {
				connectors = append(connectors, connector)
			}
			s.connectors[svc.RoutingKey] = connectors

			listeners := s.listeners[svc.RoutingKey]
			for _, listener := range svc.Listeners {
				listeners = append(listeners, listener)
			}
			s.listeners[svc.RoutingKey] = listeners
		}
	}
}

func (s *BindingStatus) updateMatchingListenerCount(connector *skupperv1alpha1.Connector) *skupperv1alpha1.Connector {
	if connector.SetMatchingListenerCount(len(s.listeners[connector.Spec.RoutingKey])) {
		updated, err := updateConnectorStatus(s.client, connector)
		if err != nil {
			log.Printf("Failed to update status for connector %s/%s", connector.Namespace, connector.Name)
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) updateMatchingConnectorCount(listener *skupperv1alpha1.Listener) *skupperv1alpha1.Listener {
	if listener.SetMatchingConnectorCount(len(s.connectors[listener.Spec.RoutingKey])) {
		updated, err := updateListenerStatus(s.client, listener)
		if err != nil {
			log.Printf("Failed to update status for listener %s/%s", listener.Namespace, listener.Name)
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) error() error {
	if len(s.errors) > 0 {
		return fmt.Errorf(strings.Join(s.errors, ", "))
	}
	return nil
}
