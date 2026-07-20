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
	user      string
	password  string
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

	if cmd.Flags != nil && cmd.Flags.Uninstall {
		if cmd.Flags.Password != "" {
			validationErrors = append(validationErrors, fmt.Errorf("--%s cannot be used with --%s", common.FlagNameNetworkObserverPassword, common.FlagNameNetworkObserverUninstall))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemNetworkObserver) InputToOptions() {

	if cmd.Flags.Username != "" {
		cmd.user = cmd.Flags.Username
	}

	if cmd.Flags.Password != "" {
		cmd.password = cmd.Flags.Password
	}

}

func (cmd *CmdSystemNetworkObserver) Run() error {
	installer, err := networkobserver.NewInstaller(cmd.namespace, cmd.user, cmd.password)
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
	fmt.Printf("Username: %s\n", result.Username)
	fmt.Printf("Password: %s\n", result.Password)
	fmt.Println("\nNote: Save these credentials securely.")

	return nil
}

func (cmd *CmdSystemNetworkObserver) WaitUntil() error {
	return nil
}
