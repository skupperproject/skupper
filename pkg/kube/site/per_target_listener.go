package site

import (
	"reflect"
	"strconv"
	"strings"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type PerTargetListener struct {
	definition *skupperv2alpha1.Listener
	targets    map[string]int // name -> port
}

func newPerTargetListener(l *skupperv2alpha1.Listener) *PerTargetListener {
	return &PerTargetListener{
		definition: l,
		targets:    map[string]int{},
	}
}

func (p *PerTargetListener) updateListener(l *skupperv2alpha1.Listener) bool {
	changed := p.definition == nil || !reflect.DeepEqual(l.Spec, p.definition.Spec)
	p.definition = l
	return changed
}

func (p *PerTargetListener) extractTargets(network []skupperv2alpha1.SiteRecord, mapping *qdr.PortMapping, exposedPorts ExposedPorts, context BindingContext) (bool, error) {
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
	return changed, nil
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
				return err
			}
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

func (p *PerTargetListener) updateBridgeConfig(siteId string, config *qdr.BridgeConfig) {
	for target, port := range p.targets {
		if p.definition.Spec.Type == "tcp" || p.definition.Spec.Type == "" {
			config.AddTcpListener(qdr.TcpEndpoint{
				Name:       p.definition.Name + "-" + target,
				SiteId:     siteId,
				Host:       "0.0.0.0",
				Port:       strconv.Itoa(port),
				Address:    p.address(target),
				SslProfile: p.definition.Spec.TlsCredentials,
			})
		}
	}
}

func extractTargets(prefix string, network []skupperv2alpha1.SiteRecord) []string {
	var results []string
	for _, site := range network {
		for _, service := range site.Services {
			if strings.HasPrefix(service.RoutingKey, prefix) {
				results = append(results, strings.TrimPrefix(service.RoutingKey, prefix))
			}
		}
	}
	return results
}

func equivalentSlices(a []string, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, s := range a {
		for _, r := range b {
			if s == r {
				continue
			}
		}
		return false
	}
	return true
}
