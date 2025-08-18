package nonkube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
)

type CmdSystemGenerateBundle struct {
	CobraCmd        *cobra.Command
	Namespace       string
	PreCheck        func(config *bootstrap.Config) error
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	Flags           *common.CommandSystemGenerateBundleFlags
	ConfigBootstrap bootstrap.Config
	BundleName      string
}

func NewCmdSystemGenerateBundle() *CmdSystemGenerateBundle {

	skupperCmd := CmdSystemGenerateBundle{}

	return &skupperCmd
}

func (cmd *CmdSystemGenerateBundle) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.PreCheck = bootstrap.PreBootstrap
	cmd.Bootstrap = bootstrap.Bootstrap
	cmd.PostExec = bootstrap.PostBootstrap
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemGenerateBundle) ValidateInput(args []string) error {
	var validationErrors []error
	namespaceStringValidator := validator.NamespaceStringValidator()

	if args == nil || len(args) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("You need to specify a name for the bundle file to generate."))
	}

	if args != nil && len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("This command does not accept more than one argument."))
	}

	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	if len(args) == 1 {
		cmd.BundleName = args[0]
	}

	if cmd.Flags != nil && cmd.Flags.Input != "" {

		inputPath, err := filepath.Abs(cmd.Flags.Input)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("Unable to determine absolute path of %s: %v", inputPath, err))
		}

		if info, err := os.Stat(inputPath); err == nil {
			if !info.IsDir() {
				validationErrors = append(validationErrors, fmt.Errorf("The input path must be a directory"))
			}
		} else if os.IsNotExist(err) {
			validationErrors = append(validationErrors, fmt.Errorf("The input path does not exist"))
		}

	}
	if cmd.Flags != nil && cmd.Flags.Type != "" {
		typeValidator := validator.NewOptionValidator(common.BundleTypes)

		ok, err := typeValidator.Evaluate(cmd.Flags.Type)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("Invalid bundle type: %s", err))
		}

	}

	return errors.Join(validationErrors...)

}

func (cmd *CmdSystemGenerateBundle) InputToOptions() {

	var inputPath string

	namespace := "default"
	if cmd.Namespace != "" {
		namespace = cmd.Namespace
	}

	if cmd.Flags.Input != "" {
		inputPath, _ = filepath.Abs(cmd.Flags.Input)
	}

	selectedType := cmd.Flags.Type
	if cmd.Flags.Type == "shell-script" {
		selectedType = "bundle"
	}

	isBundle := internalbundle.GetBundleStrategy(selectedType) != ""

	selectedPlatform := config.GetPlatform()

	configBootStrap := bootstrap.Config{
		InputPath:      inputPath,
		Namespace:      namespace,
		BundleName:     cmd.BundleName,
		BundleStrategy: internalbundle.GetBundleStrategy(selectedType),
		IsBundle:       isBundle,
		Platform:       selectedPlatform,
	}

	cmd.ConfigBootstrap = configBootStrap

}

func (cmd *CmdSystemGenerateBundle) Run() error {
	err := cmd.PreCheck(&cmd.ConfigBootstrap)
	if err != nil {
		return err
	}

	siteState, err := cmd.Bootstrap(&cmd.ConfigBootstrap)
	if err != nil {
		return fmt.Errorf("Failed to generate bundle: %s", err)
	}

	cmd.PostExec(&cmd.ConfigBootstrap, siteState)

	return nil
}

func (cmd *CmdSystemGenerateBundle) WaitUntil() error { return nil }
