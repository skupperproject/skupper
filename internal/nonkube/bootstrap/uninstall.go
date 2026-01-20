package bootstrap

import (
	"fmt"
	"os"
	"os/user"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap/controller"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

func Uninstall(platform string) error {

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %v", err)
	}

	containerName := fmt.Sprintf("%s-skupper-controller", currentUser.Username)

	isContainerAlreadyRunningInPodman := IsContainerRunning(containerName, types.PlatformPodman)

	if isContainerAlreadyRunningInPodman && platform == "docker" {
		fmt.Printf("Warning: The system controller container %q is already running in Podman but the selected platform is Docker.\n", containerName)
		return nil
	}

	isContainerAlreadyRunningInDocker := IsContainerRunning(containerName, types.PlatformDocker)

	if isContainerAlreadyRunningInDocker && platform == "podman" {
		fmt.Printf("Warning: The system controller container %q is already running in Docker but the selected platform is Podman.\n", containerName)
		return nil
	}

	endpoint := ""

	if platform == "docker" {
		endpoint = "unix:///run/docker.sock"
	}

	cli, err := internalclient.NewCompatClient(endpoint, "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}

	container, err := cli.ContainerInspect(containerName)
	if err != nil || container == nil {
		return nil
	}

	err = cli.ContainerStop(containerName)
	if err != nil {
		return fmt.Errorf("failed to stop system-controller container: %v", err)
	}

	err = cli.ContainerRemove(containerName)
	if err != nil {
		return fmt.Errorf("failed to remove system-controller container: %v", err)
	} else {
		fmt.Println("System-controller has been removed")
	}

	systemdService, err := controller.NewSystemdServiceInfo(*container, platform)
	if err != nil {
		return nil

	}

	systemdService.Remove()

	systemdGlobal, err := common.NewSystemdGlobal(platform)
	if err != nil {
		return err
	}

	err = systemdGlobal.Disable()
	if err != nil {
		return err
	}

	fmt.Printf("Platform %s infrastructure for Skupper is now uninstalled\n", platform)

	return nil
}

func CheckActiveSites() (bool, error) {
	activeSites := false

	entries, err := os.ReadDir(path.Join(api.GetHostDataHome(), "namespaces/"))
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			runtimeDir := "namespaces/" + entry.Name() + "/runtime/"
			_, err := os.ReadDir(path.Join(api.GetHostDataHome(), runtimeDir))
			if err == nil {
				activeSites = true
				fmt.Printf("site %s active\n", entry.Name())
			}

		}
	}

	if activeSites == true {
		return true, nil
	} else {
		return false, nil
	}
}
