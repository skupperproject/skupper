package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/utils/validator"

	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemInstall struct {
	CobraCmd      *cobra.Command
	Namespace     string
	SystemInstall func(string, string) error
	SystemUpgrade func(string) error
	Flags         *common.CommandSystemInstallFlags
	reloadType    string
	upgrade       bool
}

func NewCmdSystemInstall() *CmdSystemInstall {

	skupperCmd := CmdSystemInstall{}

	return &skupperCmd
}

func (cmd *CmdSystemInstall) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemInstall = bootstrap.Install
	cmd.SystemUpgrade = bootstrap.Upgrade
}

func (cmd *CmdSystemInstall) ValidateInput(args []string) error {

	var validationErrors []error
	reloadTypeValidator := validator.NewOptionValidator(common.ReloadTypes)

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if config.GetPlatform() != types.PlatformPodman && config.GetPlatform() != types.PlatformDocker {
		validationErrors = append(validationErrors, fmt.Errorf("the selected platform is not supported by this command. There is nothing to install"))
	}

	if cmd.Flags != nil && cmd.Flags.Upgrade {
		reloadTypeSpecified := false
		if cmd.CobraCmd != nil && cmd.CobraCmd.Flags().Changed(common.FlagNameReloadType) {
			reloadTypeSpecified = true
		} else if cmd.CobraCmd == nil && cmd.Flags.ReloadType != "" {
			reloadTypeSpecified = true
		}
		if reloadTypeSpecified {
			validationErrors = append(validationErrors, fmt.Errorf("--%s cannot be specified with --%s", common.FlagNameReloadType, common.FlagNameUpgrade))
		}
	} else if cmd.Flags != nil && cmd.Flags.ReloadType != "" {
		ok, err := reloadTypeValidator.Evaluate(cmd.Flags.ReloadType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("reload type is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemInstall) InputToOptions() {
	if cmd.Flags != nil && cmd.Flags.ReloadType != "" {
		cmd.reloadType = cmd.Flags.ReloadType
	}
	if cmd.Flags != nil {
		cmd.upgrade = cmd.Flags.Upgrade
	}
}

func (cmd *CmdSystemInstall) Run() error {
	var err error
	if cmd.upgrade {
		err = cmd.SystemUpgrade(string(config.GetPlatform()))
	} else {
		err = cmd.SystemInstall(string(config.GetPlatform()), cmd.reloadType)
	}
	if err != nil {
		return fmt.Errorf("failed to configure the environment: %s", err)
	}
	return nil
}

func (cmd *CmdSystemInstall) WaitUntil() error { return nil }
