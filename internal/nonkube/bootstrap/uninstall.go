package bootstrap

import (
	"fmt"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
	"os/user"
	"path"
)

func Uninstall(platform string) error {

	cli, err := internalclient.NewCompatClient("", "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("Failed to get current user: %v", err)
	}

	containerName := fmt.Sprintf("%s-skupper-system-controller", currentUser.Username)

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
	}

	systemdGlobal, err := common.NewSystemdGlobal(platform)
	if err != nil {
		return err
	}

	err = systemdGlobal.Disable()
	if err != nil {
		return err
	}

	return nil
}

func CheckActiveSites() (bool, error) {

	entries, err := os.ReadDir(path.Join(api.GetHostDataHome(), "namespaces/"))
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return true, nil
		}
	}

	return false, nil
}
