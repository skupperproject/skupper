package podman

import (
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
)

type SkupperComponentHandler struct {
	cli *podman.PodmanRestClient
}

func NewSkupperComponentHandlerPodman(cli *podman.PodmanRestClient) *SkupperComponentHandler {
	return &SkupperComponentHandler{
		cli: cli,
	}
}

func (s *SkupperComponentHandler) Get(name string) (domain.SkupperComponent, error) {
	c, err := s.cli.ContainerInspect(name)
	if err != nil {
		return nil, err
	}
	if err = OwnedBySkupper("container", c.Labels); err != nil {
		return nil, err
	}
	// parsing site ingresses
	siteIngresses := []domain.SiteIngress{}
	for _, port := range c.Ports {
		hostPort, _ := strconv.Atoi(port.Host)
		targetPort, _ := strconv.Atoi(port.Target)
		siteIngresses = append(siteIngresses, SiteIngressHost{
			&domain.SiteIngressCommon{
				Host: port.HostIP,
				Port: hostPort,
				Target: &domain.PortCommon{
					Port: targetPort,
				},
			},
		})
	}

	componentName := c.Labels[types.ComponentAnnotation]
	var component domain.SkupperComponent
	switch componentName {
	case types.TransportDeploymentName:
		component = &domain.Router{
			Image:         c.Image,
			Env:           c.Env,
			Labels:        c.Labels,
			SiteIngresses: siteIngresses,
		}
	case types.FlowCollectorContainerName:
		component = &domain.FlowCollector{
			Image:         c.Image,
			Env:           c.Env,
			Labels:        c.Labels,
			SiteIngresses: siteIngresses,
		}
	case types.ControllerDeploymentName:
		component = &domain.ServiceController{
			Image:         c.Image,
			Env:           c.Env,
			Labels:        c.Labels,
			SiteIngresses: siteIngresses,
		}
	default:
		return nil, fmt.Errorf("invalid component: %s", componentName)
	}

	return component, nil
}

func (s *SkupperComponentHandler) List() ([]domain.SkupperComponent, error) {
	components := []domain.SkupperComponent{}
	list, err := s.cli.ContainerList()
	if err != nil {
		return nil, err
	}
	for _, c := range list {
		// ignoring containers not owned by Skupper
		if err = OwnedBySkupper("container", c.Labels); err != nil {
			continue
		}
		component, err := s.Get(c.Name)
		if err != nil {
			continue
		}
		components = append(components, component)
	}
	return components, nil
}
