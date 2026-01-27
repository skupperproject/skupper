package bootstrap

import (
	"fmt"
	"os"

	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type LocalData struct {
	servicePath string
	service     string
}

func Teardown(namespace string) error {

	platformLoader := &common.NamespacePlatformLoader{}
	platform, err := platformLoader.Load(namespace)
	if err != nil {
		return err
	}
	return RemoveAll(namespace, platform)

}

func removeDefinition(namespace string) error {

	_, err := os.Stat(api.GetHostNamespaceHome(namespace))
	if err != nil {
		return err
	}

	return os.RemoveAll(api.GetHostNamespaceHome(namespace))
}

func removeRouter(namespace string, platform string) error {

	endpoint := os.Getenv("CONTAINER_ENDPOINT")

	// the container endpoint is mapped to the podman socket inside the container
	if api.IsRunningInContainer() {
		endpoint = "unix:///var/run/podman.sock"
		if platform == "docker" {
			endpoint = "unix:///var/run/docker.sock"
		}
	} else {
		if endpoint == "" {
			endpoint = fmt.Sprintf("unix://%s/podman/podman.sock", api.GetRuntimeDir())
			if platform == "docker" {
				endpoint = "unix:///run/docker.sock"
			}
		}
	}

	cli, err := internalclient.NewCompatClient(endpoint, "")
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

func RemoveAll(namespace string, platform string) error {

	if err := removeRouter(namespace, platform); err != nil {
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
