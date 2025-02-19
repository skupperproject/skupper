package bootstrap

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/common"
)

func Install() error {

	systemdGlobal, err := common.NewSystemdGlobal(string(types.PlatformPodman))
	if err != nil {
		return err
	}

	err = systemdGlobal.Enable()
	if err != nil {
		return err
	}

	return nil
}
