package bootstrap

import (
	"fmt"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"os"
)

type LocalData struct {
	servicePath string
	service     string
}

func Teardown(namespace string, platform string) error {

	if err := removeRouter(namespace); err != nil {
		return err
	}

	if err := removeService(namespace, platform); err != nil {
		return err
	}

	if err := removeDefinition(namespace); err != nil {
		return err
	}

	fmt.Printf("Namespace \"%s\" has been removed\n", namespace)
	return nil

}

func removeDefinition(namespace string) error {

	_, err := os.Stat(api.GetHostNamespaceHome(namespace))
	if err != nil {
		return err
	}

	return os.RemoveAll(api.GetHostNamespaceHome(namespace))
}

func removeRouter(namespace string) error {

	cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	containerName := namespace + "-skupper-router"
	if _, err := cli.ContainerInspect(containerName); err == nil {
		err = cli.ContainerRemove(containerName)
		if err != nil {
			return err
		}
	}

	return nil
}

func removeService(namespace string, platform string) error {

	pathProvider := fs.PathProvider{Namespace: namespace}

	siteStateLoader := &common.FileSystemSiteStateLoader{
		Path: pathProvider.GetRuntimeNamespace(),
	}

	siteState, err := siteStateLoader.Load()
	if err != nil {
		return err
	}

	systemdService, err := common.NewSystemdServiceInfo(siteState, platform)
	if err != nil {
		return err
	}

	if systemdService.GetServiceFile() != "" {
		err = systemdService.Remove()
		if err != nil {
			return err
		}
	}

	return nil
}
