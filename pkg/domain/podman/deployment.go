package podman

import (
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/domain"
)

type SkupperDeploymentPodman struct {
	*domain.SkupperDeploymentCommon
	Name         string
	Aliases      []string
	VolumeMounts map[string]string
	Networks     []string
}

func (s *SkupperDeploymentPodman) GetName() string {
	return s.Name
}

type SkupperDeploymentHandlerPodman struct {
	cli *podman.PodmanRestClient
}

func NewSkupperDeploymentHandlerPodman(cli *podman.PodmanRestClient) *SkupperDeploymentHandlerPodman {
	return &SkupperDeploymentHandlerPodman{
		cli: cli,
	}
}

// Deploy deploys each component as a container
func (s *SkupperDeploymentHandlerPodman) Deploy(deployment domain.SkupperDeployment) error {
	var err error
	var cleanupContainers []string

	defer func() {
		if err != nil {
			for _, containerName := range cleanupContainers {
				_ = s.cli.ContainerStop(containerName)
				_ = s.cli.ContainerRemove(containerName)
			}
		}
	}()

	if len(deployment.GetComponents()) > 1 {
		return fmt.Errorf("podman implementation currently allows only one component per deployment")
	}

	podmanDeployment := deployment.(*SkupperDeploymentPodman)
	for _, component := range deployment.GetComponents() {

		// Pulling image first
		err = s.cli.ImagePull(component.GetImage())
		if err != nil {
			return err
		}

		// Setting network aliases
		networkMap := map[string]container.ContainerNetworkInfo{}
		for _, network := range podmanDeployment.Networks {
			networkMap[network] = container.ContainerNetworkInfo{
				Aliases: podmanDeployment.Aliases,
			}
		}

		// Defining the mounted volumes
		mounts := []container.Volume{}
		for volumeName, destDir := range podmanDeployment.VolumeMounts {
			var volume *container.Volume
			volume, err = s.cli.VolumeInspect(volumeName)
			if err != nil {
				err = fmt.Errorf("error reading volume %s - %v", volumeName, err)
				return err
			}
			volume.Destination = destDir
			volume.Mode = "z" // shared between containers
			mounts = append(mounts, *volume)
		}

		// Ports
		ports := []container.Port{}
		for _, siteIngress := range component.GetSiteIngresses() {
			ports = append(ports, container.Port{
				Host:     strconv.Itoa(siteIngress.GetPort()),
				HostIP:   siteIngress.GetHost(),
				Target:   strconv.Itoa(siteIngress.GetTarget().GetPort()),
				Protocol: "tcp",
			})
		}

		// Defining the container
		labels := component.GetLabels()
		labels[types.ComponentAnnotation] = deployment.GetName()
		c := &container.Container{
			Name:          component.Name(),
			Image:         component.GetImage(),
			Env:           component.GetEnv(),
			Labels:        labels,
			Networks:      networkMap,
			Mounts:        mounts,
			Ports:         ports,
			RestartPolicy: "always",
		}

		err = s.cli.ContainerCreate(c)
		if err != nil {
			return fmt.Errorf("error creating skupper component: %s - %v", c.Name, err)
		}
		cleanupContainers = append(cleanupContainers, c.Name)

		err = s.cli.ContainerStart(c.Name)
		if err != nil {
			return fmt.Errorf("error starting skupper component: %s - %v", c.Name, err)
		}
	}

	return nil
}

func (s *SkupperDeploymentHandlerPodman) Undeploy(name string) error {
	containers, err := s.cli.ContainerList()
	if err != nil {
		return fmt.Errorf("error listing containers - %w", err)
	}

	stopContainers := []string{}
	for _, c := range containers {
		if component, ok := c.Labels[types.ComponentAnnotation]; ok && component == name {
			stopContainers = append(stopContainers, c.Name)
		}
	}

	if len(stopContainers) == 0 {
		return nil
	}

	for _, c := range stopContainers {
		_ = s.cli.ContainerStop(c)
		_ = s.cli.ContainerRemove(c)
	}
	return nil
}

func (s *SkupperDeploymentHandlerPodman) List() ([]domain.SkupperDeployment, error) {
	depMap := map[string]domain.SkupperDeployment{}

	list, err := s.cli.ContainerList()
	if err != nil {
		return nil, fmt.Errorf("error retrieving container list - %w", err)
	}

	var depList []domain.SkupperDeployment

	componentHandler := NewSkupperComponentHandlerPodman(s.cli)
	components, err := componentHandler.List()
	if err != nil {
		return nil, fmt.Errorf("error retrieving existing skupper components - %w", err)
	}

	for _, c := range list {
		ci, err := s.cli.ContainerInspect(c.Name)
		if err != nil {
			return nil, fmt.Errorf("error retrieving container information for %s - %w", c.Name, err)
		}
		if ci.Labels == nil {
			continue
		}
		deployName, ok := ci.Labels[types.ComponentAnnotation]
		if !ok {
			continue
		}
		var aliases []string
		for _, aliases = range ci.NetworkAliases() {
			break
		}
		mounts := map[string]string{}
		for _, mount := range ci.Mounts {
			mounts[mount.Name] = mount.Destination
		}
		deployment := &SkupperDeploymentPodman{
			SkupperDeploymentCommon: &domain.SkupperDeploymentCommon{},
			Name:                    deployName,
			Aliases:                 aliases,
			VolumeMounts:            mounts,
			Networks:                ci.NetworkNames(),
		}
		depMap[deployName] = deployment

		depComponents := []domain.SkupperComponent{}
		for _, component := range components {
			if compOwner, ok := component.GetLabels()[types.ComponentAnnotation]; ok && compOwner == deployName {
				depComponents = append(depComponents, component)
			}
		}
		deployment.Components = depComponents
		depList = append(depList, deployment)
	}

	return depList, nil
}
