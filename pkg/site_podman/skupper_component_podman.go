package site_podman

import (
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
)

type SkupperComponentHandlerPodman struct {
	cli *podman.PodmanRestClient
}

func NewSkupperComponentHandlerPodman(cli *podman.PodmanRestClient) *SkupperComponentHandlerPodman {
	return &SkupperComponentHandlerPodman{
		cli: cli,
	}
}

func (s *SkupperComponentHandlerPodman) Get(name string) (domain.SkupperComponent, error) {
	c, err := s.cli.ContainerInspect(name)
	if err != nil {
		return nil, err
	}
	notOwnedErr := fmt.Errorf("container is not owned by Skupper")
	if c.Labels == nil {
		return nil, notOwnedErr
	}
	if app, ok := c.Labels["application"]; !ok || app != types.AppName {
		return nil, notOwnedErr
	}
	// parsing site ingresses
	siteIngresses := []domain.SiteIngress{}
	for _, port := range c.Ports {
		hostPort, _ := strconv.Atoi(port.Host)
		targetPort, _ := strconv.Atoi(port.Target)
		siteIngresses = append(siteIngresses, SiteIngressPodmanHost{
			&domain.SiteIngressCommon{
				Host: port.HostIP,
				Port: hostPort,
				Target: &domain.PortCommon{
					Port: targetPort,
				},
			},
		})
	}

	// currently only router component is supported
	component := &domain.Router{
		Env:           c.Env,
		Labels:        c.Labels,
		SiteIngresses: siteIngresses,
	}

	return component, nil
}

func (s *SkupperComponentHandlerPodman) List() ([]domain.SkupperComponent, error) {
	components := []domain.SkupperComponent{}
	list, err := s.cli.ContainerList()
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		component, err := s.Get(c.Name)
		if err != nil {
			continue
		}
		components = append(components, component)
	}
	return components, nil
}
