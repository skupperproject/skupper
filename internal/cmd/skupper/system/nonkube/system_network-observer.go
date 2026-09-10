package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	networkobserver "github.com/skupperproject/skupper/internal/nonkube/network-observer"
	"github.com/spf13/cobra"
)

type CmdSystemNetworkObserver struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandNetworkObserverFlags
	namespace string
}

func NewCmdSystemNetworkObserver() *CmdSystemNetworkObserver {
	return &CmdSystemNetworkObserver{}
}

func (cmd *CmdSystemNetworkObserver) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
}

func (cmd *CmdSystemNetworkObserver) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemNetworkObserver) InputToOptions() {}

func (cmd *CmdSystemNetworkObserver) Run() error {
	installer, err := networkobserver.NewInstaller(cmd.namespace)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	if cmd.Flags.Uninstall {

		if err := installer.ValidatePrerequisitesForUninstall(); err != nil {
			return err
		}

		if err := installer.Uninstall(); err != nil {
			return fmt.Errorf("uninstallation failed: %w", err)
		}

		return nil
	}

	if err := installer.ValidatePrerequisitesForInstall(); err != nil {
		return fmt.Errorf("prerequisite validation failed: %w", err)
	}

	result, err := installer.Install()
	if err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	fmt.Println("Network observer installed successfully!")
	fmt.Printf("\nAccess URL: %s\n", result.URL)

	return nil
}

func (cmd *CmdSystemNetworkObserver) WaitUntil() error {
	return nil
}
