package bootstrap

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
	"path"
)

func Uninstall() error {

	cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	err = cli.NetworkRemove("skupper")
	if err != nil {
		return err
	}

	systemdGlobal, err := common.NewSystemdGlobal(string(types.PlatformPodman))
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

	entries, err := os.ReadDir(path.Join(api.GetDataHome(), "namespaces/"))
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
