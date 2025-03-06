package bootstrap

import (
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
	"path"
)

func Uninstall(platform string) error {

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
