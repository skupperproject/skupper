package site

import (
	"fmt"
	"log/slog"
	"strings"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type BindingStatus struct {
	connectors map[string][]string
	listeners  map[string][]string
	client     internalclient.Clients
	errors     []string
	logger     *slog.Logger
}

func newBindingStatus(client internalclient.Clients, network []skupperv2alpha1.SiteRecord) *BindingStatus {
	s := &BindingStatus{
		client:     client,
		connectors: map[string][]string{},
		listeners:  map[string][]string{},
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.binding_status"),
		),
	}
	s.populate(network)
	return s
}

func (s *BindingStatus) populate(network []skupperv2alpha1.SiteRecord) {
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

func (s *BindingStatus) updateMatchingListenerCount(connector *skupperv2alpha1.Connector) *skupperv2alpha1.Connector {
	if connector.SetMatchingListenerCount(len(s.listeners[connector.Spec.RoutingKey])) {
		updated, err := updateConnectorStatus(s.client, connector)
		if err != nil {
			s.logger.Error("Failed to update status for connector",
				slog.String("namespace", connector.Namespace),
				slog.String("name", connector.Name))
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) updateMatchingConnectorCount(listener *skupperv2alpha1.Listener) *skupperv2alpha1.Listener {
	if listener.SetMatchingConnectorCount(len(s.connectors[listener.Spec.RoutingKey])) {
		updated, err := updateListenerStatus(s.client, listener)
		if err != nil {
			s.logger.Error("Failed to update status for listener",
				slog.String("namespace", listener.Namespace),
				slog.String("name", listener.Name))
			s.errors = append(s.errors, err.Error())
			return nil
		}
		return updated
	}
	return nil
}

func (s *BindingStatus) updateMatchingListenerCountForAttachedConnector(connector *AttachedConnector) {
	if connector.anchor != nil {
		connector.setMatchingListenerCount(len(s.listeners[connector.anchor.Spec.RoutingKey]))
	}
}

func (s *BindingStatus) error() error {
	if len(s.errors) > 0 {
		return fmt.Errorf(strings.Join(s.errors, ", "))
	}
	return nil
}
