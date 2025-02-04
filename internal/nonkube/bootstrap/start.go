package bootstrap

import (
	"fmt"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
)

func Start(namespace string) error {
	_, err := os.Stat(api.GetHostNamespaceHome(namespace))
	if err != nil {
		return fmt.Errorf("there is no definition for namespace %s: %s", namespace, err)
	}

	cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	containerName := namespace + "-skupper-router"
	if _, err := cli.ContainerInspect(containerName); err == nil {
		err = cli.ContainerStart(containerName)
		if err != nil {
			return err
		}
	}

	return nil
}
