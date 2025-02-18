package bootstrap

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/container"
	"os"
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

	cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	_, err = cli.NetworkCreate(&container.Network{Name: "skupper"})
	if err != nil {
		return err
	}

	return nil
}
