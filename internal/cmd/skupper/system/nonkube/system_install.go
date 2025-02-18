package nonkube

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"

	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemInstall struct {
	CobraCmd      *cobra.Command
	Namespace     string
	SystemInstall func() error
}

func NewCmdSystemInstall() *CmdSystemInstall {

	skupperCmd := CmdSystemInstall{}

	return &skupperCmd
}

func (cmd *CmdSystemInstall) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemInstall = bootstrap.Install
}

func (cmd *CmdSystemInstall) ValidateInput(args []string) error {

	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if config.GetPlatform() != types.PlatformPodman {
		validationErrors = append(validationErrors, fmt.Errorf("the selected platorm is not podman. There is nothing to install"))
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemInstall) InputToOptions() {}

func (cmd *CmdSystemInstall) Run() error {
	err := cmd.SystemInstall()

	if err != nil {
		return fmt.Errorf("failed to configure the environment : %s", err)
	}

	fmt.Println("Podman is now configured for Skupper")

	return nil
}

func (cmd *CmdSystemInstall) WaitUntil() error { return nil }
