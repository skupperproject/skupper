package nonkube

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemUninstall struct {
	CobraCmd         *cobra.Command
	Namespace        string
	SystemUninstall  func() error
	CheckActiveSites func() (bool, error)
	Flags            *common.CommandSystemUninstallFlags
	forceUninstall   bool
}

func NewCmdSystemUninstall() *CmdSystemUninstall {

	skupperCmd := CmdSystemUninstall{}

	return &skupperCmd
}

func (cmd *CmdSystemUninstall) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemUninstall = bootstrap.Uninstall
	cmd.CheckActiveSites = bootstrap.CheckActiveSites
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemUninstall) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if cmd.Flags != nil && !cmd.Flags.Force {
		activeSites, err := cmd.CheckActiveSites()
		if err != nil {
			return err
		}
		if activeSites {
			validationErrors = append(validationErrors, fmt.Errorf("Uninstallation halted: Active sites detected."))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemUninstall) InputToOptions() {

	cmd.forceUninstall = cmd.Flags.Force
}

func (cmd *CmdSystemUninstall) Run() error {

	err := cmd.SystemUninstall()

	if err != nil {
		return fmt.Errorf("failed to uninstall : %s", err)
	}

	fmt.Println("Podman infrastructure for Skupper is now uninstalled")

	return nil
}

func (cmd *CmdSystemUninstall) WaitUntil() error { return nil }
