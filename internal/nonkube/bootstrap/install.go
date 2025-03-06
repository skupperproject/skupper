package bootstrap

import (
	"github.com/skupperproject/skupper/internal/nonkube/common"
)

func Install(platform string) error {

	systemdGlobal, err := common.NewSystemdGlobal(platform)
	if err != nil {
		return err
	}

	err = systemdGlobal.Enable()
	if err != nil {
		return err
	}

	return nil
}
