package site

import (
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type PerTargetListener struct {
	definition *skupperv2alpha1.Listener
	targets    map[string]int // name -> port
	logger     *slog.Logger
}

func newPerTargetListener(l *skupperv2alpha1.Listener, logger *slog.Logger) *PerTargetListener {
	return &PerTargetListener{
		definition: l,
		targets:    map[string]int{},
		logger:     logger,
	}
}

func (p *PerTargetListener) updateListener(l *skupperv2alpha1.Listener) bool {
	changed := p.definition == nil || !reflect.DeepEqual(l.Spec, p.definition.Spec)
	p.definition = l
	return changed
}

func (p *PerTargetListener) extractTargets(network []skupperv2alpha1.SiteRecord, mapping *qdr.PortMapping, exposedPorts ExposedPorts, context BindingContext) (bool, error) {
	p.logger.Debug("Extracting targets for listener",
		slog.String("namespace", p.definition.Namespace),
		slog.String("listener", p.definition.Name))
	targets := extractTargets(p.address(""), network)
	changed := false
	stale := map[string]bool{}
	for key, _ := range p.targets {
		stale[key] = true
	}
	for _, target := range targets {
		delete(stale, target)
		if _, ok := p.targets[target]; !ok {
			changed = true
			allocatedRouterPort, err := mapping.GetPortForKey(p.address(target))
			if err != nil {
				return false, err
			}
			p.targets[target] = allocatedRouterPort
		}
	}
	for target, _ := range stale {
		mapping.ReleasePortForKey(p.address(target))
		if err := p.unexpose(target, mapping, exposedPorts, context); err != nil {
			return false, err
		}
		delete(p.targets, target)
	}
	if !changed {
		return false, nil
	}
	if err := p.expose(mapping, exposedPorts, context); err != nil {
		return true, err
	}
	return true, nil
}

func (p *PerTargetListener) address(target string) string {
	return p.definition.Spec.RoutingKey + "." + target
}

func (p *PerTargetListener) expose(mapping *qdr.PortMapping, exposedPorts ExposedPorts, context BindingContext) error {
	for target, allocatedRouterPort := range p.targets {
		port := Port{
			Name:       p.definition.Name,
			Port:       p.definition.Spec.Port,
			TargetPort: allocatedRouterPort,
			Protocol:   p.definition.Protocol(),
		}
		if ports := exposedPorts.Expose(target, port); ports != nil {
			if err := context.Expose(ports); err != nil {
				p.logger.Error("Error exposing per target listener",
					slog.String("namespace", p.definition.Namespace),
					slog.String("listener", p.definition.Name),
					slog.String("target", target),
					slog.Any("error", err),
				)

				return err
			}
			p.logger.Info("Exposed per target listener",
				slog.String("namespace", p.definition.Namespace),
				slog.String("listener", p.definition.Name),
				slog.String("target", target),
			)
		}
	}
	return nil
}

func (p *PerTargetListener) unexposeAll(mapping *qdr.PortMapping, exposedPorts ExposedPorts, context BindingContext) error {
	for target, _ := range p.targets {
		if err := p.unexpose(target, mapping, exposedPorts, context); err != nil {
			return err
		}
	}
	return nil
}

func (p *PerTargetListener) unexpose(target string, mapping *qdr.PortMapping, exposedPorts ExposedPorts, context BindingContext) error {
	if exposed := exposedPorts.Unexpose(target, p.definition.Name); exposed != nil {
		mapping.ReleasePortForKey(p.address(target))
		if exposed.empty() {
			if err := context.Unexpose(target); err != nil {
				return err
			}
		} else {
			if err := context.Expose(exposed); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *PerTargetListener) updateBridgeConfig(siteId string, config *qdr.BridgeConfig) bool {
	var updated bool
	for target, port := range p.targets {
		if p.definition.Spec.Type == "tcp" || p.definition.Spec.Type == "" {
			if config.AddTcpListener(qdr.TcpEndpoint{
				Name:       p.definition.Name + "@" + target,
				SiteId:     siteId,
				Port:       strconv.Itoa(port),
				Address:    p.address(target),
				SslProfile: p.definition.Spec.TlsCredentials,
			}) {
				updated = true
			}
		}
	}
	return updated
}

func extractTargets(prefix string, network []skupperv2alpha1.SiteRecord) []string {
	var results []string
	for _, site := range network {
		for _, service := range site.Services {
			if strings.HasPrefix(service.RoutingKey, prefix) && len(service.Connectors) > 0 {
				results = append(results, strings.TrimPrefix(service.RoutingKey, prefix))
			}
		}
	}
	return results
}
