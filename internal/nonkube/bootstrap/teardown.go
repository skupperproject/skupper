package bootstrap

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
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
	configuredPlatform, err := platformLoader.Load(namespace)
	if err != nil {
		return err
	}

	currentPlatform := config.GetPlatform()
	if currentPlatform.IsKubernetes() {
		currentPlatform = types.PlatformPodman
	}
	if string(currentPlatform) != configuredPlatform {
		return fmt.Errorf("existing namespace uses %q platform and it cannot change to %q", configuredPlatform, string(currentPlatform))
	}

	if err := removeRouter(namespace, configuredPlatform); err != nil {
		return err
	}

	if err := removeService(namespace, configuredPlatform); err != nil {
		return err
	}

	if err := removeDefinition(namespace); err != nil {
		return err
	}

	fmt.Printf("Namespace \"%s\" has been removed\n", namespace)
	return nil

}

func removeDefinition(namespace string) error {

	path := api.GetHostNamespaceHome(namespace)

	if api.IsRunningInContainer() {
		path = api.GetDefaultOutputPath(namespace)
	}

	_, err := os.Stat(path)
	if err != nil {
		return err
	}

	return os.RemoveAll(path)
}

func removeRouter(namespace string, platform string) error {

	endpoint := os.Getenv("CONTAINER_ENDPOINT")

	if api.IsRunningInContainer() || endpoint == "" {
		endpoint = internalclient.GetDefaultContainerEndpoint()
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
