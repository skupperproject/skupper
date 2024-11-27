package bootstrap

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"os"
	"os/exec"
	"path/filepath"
)

type LocalData struct {
	servicePath string
	service     string
}

func Teardown(namespace string, platform string) error {

	localData, err := getLocalData(namespace)
	if err != nil {
		return err
	}

	if err := removeRouter(namespace); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(localData.servicePath, localData.service)); err == nil {
		if err := removeService(platform, namespace); err != nil {
			return err
		}
	}

	if err := removeDefinition(namespace); err != nil {
		return err
	}

	fmt.Printf("Namespace \"%s\" has been removed\n", namespace)
	return nil

}

func Stop(namespace string, platform string) error {

	if err := removeRouter(namespace); err != nil {
		return err
	}

	if err := removeService(platform, namespace); err != nil {
		return err
	}

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
	skupperPlatform := config.GetPlatform()

	switch skupperPlatform {
	case "podman", "docker":
		err := exec.Command(string(skupperPlatform), "rm", "-f", namespace+"-skupper-router").Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func removeService(platform string, namespace string) error {

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

	err = systemdService.Remove()
	if err != nil {
		return err
	}

	return nil
}

func getLocalData(namespace string) (*LocalData, error) {

	var namespacesPath string

	if xdgDataHome := api.GetDataHome(); xdgDataHome != "" {
		namespacesPath = filepath.Join(xdgDataHome, "namespaces")
	} else {
		namespacesPath = filepath.Join(os.Getenv("HOME"), ".local/share/skupper/namespaces")
	}

	if _, err := os.Stat(filepath.Join(namespacesPath, namespace)); os.IsNotExist(err) {
		return nil, fmt.Errorf("Namespace \"%s\" does not exist\n", namespace)
	}

	var servicePath string
	if xdgConfigHome := api.GetConfigHome(); xdgConfigHome != "" {
		servicePath = filepath.Join(xdgConfigHome, "systemd/user")
	} else {
		servicePath = filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
	}

	service := "skupper-" + namespace + ".service"

	return &LocalData{
		servicePath: servicePath,
		service:     service,
	}, nil
}
