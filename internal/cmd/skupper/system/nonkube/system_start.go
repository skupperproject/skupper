package nonkube

import (
	"errors"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
)

type CmdSystemStart struct {
	PreCheck        func(config *bootstrap.Config) error
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	CobraCmd        *cobra.Command
	Namespace       string
	ConfigBootstrap bootstrap.Config
}

func NewCmdSystemStart() *CmdSystemStart {

	skupperCmd := CmdSystemStart{}

	return &skupperCmd
}

func (cmd *CmdSystemStart) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.PreCheck = bootstrap.PreBootstrap
	cmd.Bootstrap = bootstrap.Bootstrap
	cmd.PostExec = bootstrap.PostBootstrap
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemStart) ValidateInput(args []string) error {
	var validationErrors []error
	namespaceStringValidator := validator.NamespaceStringValidator()

	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)

	if systemReloadType == types.SystemReloadTypeAuto {
		validationErrors = append(validationErrors, fmt.Errorf("this command is disabled because automatic reloading is configured"))
	}

	if args != nil && len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	selectedNamespace := "default"
	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
		selectedNamespace = cmd.Namespace
	}

	_, err := os.Stat(api.GetInternalOutputPath(selectedNamespace, api.RuntimeSiteStatePath))
	if err == nil {
		validationErrors = append(validationErrors, fmt.Errorf("namespace already exists: %s", selectedNamespace))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemStart) InputToOptions() {

	namespace := "default"
	if cmd.Namespace != "" {
		namespace = cmd.Namespace
	}

	var binary string
	selectedPlatform := config.GetPlatform()

	switch common.Platform(selectedPlatform) {
	case common.PlatformLinux:
		binary = "skrouterd"
	case common.PlatformDocker:
		binary = "docker"
	default:
		binary = "podman"
	}

	configBootStrap := bootstrap.Config{
		Namespace: namespace,
		IsBundle:  false,
		Platform:  selectedPlatform,
		Binary:    binary,
	}

	cmd.ConfigBootstrap = configBootStrap
}

func (cmd *CmdSystemStart) Run() error {

	err := cmd.PreCheck(&cmd.ConfigBootstrap)
	if err != nil {
		return err
	}

	siteState, err := cmd.Bootstrap(&cmd.ConfigBootstrap)
	if err != nil {
		return fmt.Errorf("Failed to bootstrap: %s", err)
	}

	cmd.PostExec(&cmd.ConfigBootstrap, siteState)

	return nil
}

func (cmd *CmdSystemStart) WaitUntil() error { return nil }
