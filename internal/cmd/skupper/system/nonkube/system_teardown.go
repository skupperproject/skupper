package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemTeardown struct {
	CobraCmd  *cobra.Command
	TearDown  func(namespace string) error
	Namespace string
	Platform  string
}

func NewCmdSystemTeardown() *CmdSystemTeardown {

	skupperCmd := CmdSystemTeardown{}

	return &skupperCmd
}

func (cmd *CmdSystemTeardown) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.TearDown = bootstrap.Teardown
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
	cmd.Platform = string(config.GetPlatform())
}

func (cmd *CmdSystemTeardown) ValidateInput(args []string) error {
	if len(args) > 0 {
		return errors.New("this command does not accept arguments")
	}

	return nil
}

func (cmd *CmdSystemTeardown) InputToOptions() {
	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
}

func (cmd *CmdSystemTeardown) Run() error {

	err := cmd.TearDown(cmd.Namespace)

	if err != nil {
		return fmt.Errorf("System teardown has failed: %s", err)
	}

	return nil
}

func (cmd *CmdSystemTeardown) WaitUntil() error { return nil }
