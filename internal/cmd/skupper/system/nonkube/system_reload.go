package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemReload struct {
	CobraCmd        *cobra.Command
	Namespace       string
	PreCheck        func(config *bootstrap.Config) error
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	ConfigBootstrap bootstrap.Config
}

func NewCmdSystemReload() *CmdSystemReload {

	skupperCmd := CmdSystemReload{}

	return &skupperCmd
}

func (cmd *CmdSystemReload) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.Bootstrap = bootstrap.Bootstrap
	cmd.PreCheck = bootstrap.PreBootstrap
	cmd.PostExec = bootstrap.PostBootstrap
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemReload) ValidateInput(args []string) error {
	if len(args) > 0 {
		return errors.New("this command does not accept arguments")
	}

	return nil
}

func (cmd *CmdSystemReload) InputToOptions() {
	cmd.ConfigBootstrap.Namespace = "default"
	if cmd.Namespace != "" {
		cmd.ConfigBootstrap.Namespace = cmd.Namespace
	}
	var binary string

	selectedPlatform := config.GetPlatform()

	switch selectedPlatform {
	case types.PlatformLinux:
		binary = "skrouterd"
	case types.PlatformDocker:
		binary = "docker"
	default:
		binary = "podman"
	}

	cmd.ConfigBootstrap.Platform = selectedPlatform
	cmd.ConfigBootstrap.Binary = binary
}

func (cmd *CmdSystemReload) Run() error {
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

func (cmd *CmdSystemReload) WaitUntil() error { return nil }
