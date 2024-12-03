package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/config"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

type CmdSystemSetup struct {
	PreCheck        func(config *bootstrap.Config) error
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	CobraCmd        *cobra.Command
	Flags           *common.CommandSystemSetupFlags
	Namespace       string
	Platform        string
	ConfigBootstrap bootstrap.Config
}

func NewCmdSystemSetup() *CmdSystemSetup {

	skupperCmd := CmdSystemSetup{}

	return &skupperCmd
}

func (cmd *CmdSystemSetup) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.PreCheck = bootstrap.PreBootstrap
	cmd.Bootstrap = bootstrap.Bootstrap
	cmd.PostExec = bootstrap.PostBootstrap
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
	cmd.Platform = cobraCommand.Flag("platform").Value.String()
}

func (cmd *CmdSystemSetup) ValidateInput(args []string) []error {
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
	}

	if cmd.Flags != nil && !cmd.Flags.Force && cmd.Namespace != "" && cmd.Flags.Strategy == "" {
		_, err := os.Stat(api.GetInternalOutputPath(cmd.Namespace, api.RuntimeSiteStatePath))
		if err == nil {
			validationErrors = append(validationErrors, fmt.Errorf("Namespace already exists: %s", cmd.Namespace))
		}
	}

	return validationErrors
}

func (cmd *CmdSystemSetup) InputToOptions() {

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
	selectedPlatform := config.GetPlatform()

	if cmd.Platform != "" {
		selectedPlatform = types.Platform(cmd.Platform)
	}

	if !isBundle {
		switch selectedPlatform {
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
		Platform:       selectedPlatform,
		Binary:         binary,
	}

	cmd.ConfigBootstrap = configBootStrap
}

func (cmd *CmdSystemSetup) Run() error {

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

func (cmd *CmdSystemSetup) WaitUntil() error { return nil }
