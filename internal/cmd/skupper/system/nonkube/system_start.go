package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

type CmdSystemStart struct {
	PreCheck        func(config *bootstrap.Config) error
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	CobraCmd        *cobra.Command
	Flags           *common.CommandSystemStartFlags
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

func (cmd *CmdSystemStart) ValidateInput(args []string) []error {
	var validationErrors []error

	if args != nil && len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if cmd.Flags != nil && cmd.Flags.Path != "" {

		inputPath, err := filepath.Abs(cmd.Flags.Path)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("Unable to determine absolute path of %s: %v", inputPath, err))

		}

		if api.IsRunningInContainer() {
			if inputPath != "/input" {
				validationErrors = append(validationErrors, fmt.Errorf("The input path must be set to /input when using a container to bootstrap"))
			}
		}
	}
	if cmd.Flags != nil && cmd.Flags.Strategy != "" {
		if !internalbundle.IsValidBundle(cmd.Flags.Strategy) {
			validationErrors = append(validationErrors, fmt.Errorf("Invalid bundle strategy: %s", cmd.Flags.Strategy))
		}

		if !cmd.Flags.Force && cmd.Namespace != "" {
			_, err := os.Stat(api.GetInternalOutputPath(cmd.Namespace, api.RuntimeSiteStatePath))
			if err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("Namespace already exists: %s", cmd.Namespace))
			}
		}

	}

	return validationErrors
}

func (cmd *CmdSystemStart) InputToOptions() {

	var inputPath string
	if cmd.Flags.Path != "" {
		inputPath, _ = filepath.Abs(cmd.Flags.Path)
	}

	namespace := "default"
	if cmd.Namespace != "" {
		namespace = cmd.Namespace
	}

	isBundle := internalbundle.GetBundleStrategy(cmd.Flags.Strategy) != ""

	var binary string

	if !isBundle {
		switch config.GetPlatform() {
		case types.PlatformSystemd:
			binary = "skrouterd"
		case types.PlatformDocker:
			binary = "docker"
		default:
			binary = "podman"
		}
	}

	configBootStrap := bootstrap.Config{
		InputPath:      inputPath,
		Namespace:      namespace,
		BundleStrategy: internalbundle.GetBundleStrategy(cmd.Flags.Strategy),
		IsBundle:       internalbundle.GetBundleStrategy(cmd.Flags.Strategy) != "",
		Platform:       config.GetPlatform(),
		Binary:         binary,
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
