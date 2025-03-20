package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemStop struct {
	CobraCmd  *cobra.Command
	TearDown  func(namespace string) error
	Namespace string
	Platform  string
}

func NewCmdSystemStop() *CmdSystemStop {

	skupperCmd := CmdSystemStop{}

	return &skupperCmd
}

func (cmd *CmdSystemStop) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.TearDown = bootstrap.Teardown
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
	cmd.Platform = string(config.GetPlatform())
}

func (cmd *CmdSystemStop) ValidateInput(args []string) error {
	if len(args) > 0 {
		return errors.New("this command does not accept arguments")
	}

	return nil
}

func (cmd *CmdSystemStop) InputToOptions() {
	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
}

func (cmd *CmdSystemStop) Run() error {

	err := cmd.TearDown(cmd.Namespace)

	if err != nil {
		return fmt.Errorf("System teardown has failed: %s", err)
	}

	return nil
}

func (cmd *CmdSystemStop) WaitUntil() error { return nil }
