package bootstrap

import (
	"fmt"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
	"path"
	"strings"
)

func Uninstall(platform string) error {

	cli, err := internalclient.NewCompatClient("", "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	list, err := cli.ContainerList()
	if err != nil {
		return err
	}
	var systemControllerContainer string
	for _, container := range list {
		if strings.HasSuffix(container.Name, "system-controller") {
			systemControllerContainer = container.Name
			break
		}
	}

	err = cli.ContainerStop(systemControllerContainer)
	if err != nil {
		return fmt.Errorf("failed to stop system-controller container: %v", err)
	}

	err = cli.ContainerRemove(systemControllerContainer)
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
