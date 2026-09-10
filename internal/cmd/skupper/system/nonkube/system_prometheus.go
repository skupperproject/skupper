package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	networkobserver "github.com/skupperproject/skupper/internal/nonkube/network-observer"
	"github.com/spf13/cobra"
)

type CmdSystemPrometheus struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandPrometheusFlags
	Install   func() error
	Uninstall func() error
}

func NewCmdSystemPrometheus() *CmdSystemPrometheus {
	return &CmdSystemPrometheus{}
}

func (cmd *CmdSystemPrometheus) NewClient(cobraCommand *cobra.Command, args []string) {
	installer, err := networkobserver.NewPrometheusInstaller()
	if err != nil {
		return
	}
	cmd.Install = func() error {
		if err := installer.ValidatePrerequisitesForInstall(); err != nil {
			return fmt.Errorf("prerequisite validation failed: %w", err)
		}
		return installer.Install()
	}
	cmd.Uninstall = func() error {
		if err := installer.ValidatePrerequisitesForUninstall(); err != nil {
			return err
		}
		return installer.Uninstall()
	}
}

func (cmd *CmdSystemPrometheus) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemPrometheus) InputToOptions() {}

func (cmd *CmdSystemPrometheus) Run() error {
	if cmd.Flags.Uninstall {
		if cmd.Uninstall == nil {
			return fmt.Errorf("failed to create prometheus installer")
		}
		if err := cmd.Uninstall(); err != nil {
			return fmt.Errorf("uninstallation failed: %w", err)
		}
		fmt.Println("Prometheus uninstalled successfully!")
		return nil
	}

	if cmd.Install == nil {
		return fmt.Errorf("failed to create prometheus installer")
	}
	if err := cmd.Install(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Println("Prometheus installed successfully!")
	return nil
}

func (cmd *CmdSystemPrometheus) WaitUntil() error {
	return nil
}
