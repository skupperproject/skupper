package podman

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type RouterEntityManagerPodman struct {
	cli *podman.PodmanRestClient
}

func NewRouterEntityManagerPodman(cli *podman.PodmanRestClient) *RouterEntityManagerPodman {
	return &RouterEntityManagerPodman{
		cli: cli,
	}
}

func (r *RouterEntityManagerPodman) exec(cmd []string) (string, error) {
	return r.cli.ContainerExec(types.TransportDeploymentName, cmd)
}

func (r *RouterEntityManagerPodman) CreateSslProfile(sslProfile qdr.SslProfile) error {
	cmd := qdr.SkmanageCreateCommand("sslProfile", sslProfile.Name, sslProfile)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error creating sslProfile %s - %w", sslProfile.Name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) DeleteSslProfile(name string) error {
	cmd := qdr.SkmanageDeleteCommand("sslProfile", name)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error deleting sslProfile %s - %w", name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) CreateConnector(connector qdr.Connector) error {
	cmd := qdr.SkmanageCreateCommand("connector", connector.Name, connector)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error creating connector %s - %w", connector.Name, err)
	}
	return nil
}

func (r *RouterEntityManagerPodman) DeleteConnector(name string) error {
	cmd := qdr.SkmanageDeleteCommand("connector", name)
	if _, err := r.exec(cmd); err != nil {
		return fmt.Errorf("error deleting sslProfile %s - %w", name, err)
	}
	return nil
}
