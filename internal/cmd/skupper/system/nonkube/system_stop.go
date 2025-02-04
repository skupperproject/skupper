package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemStop struct {
	CobraCmd   *cobra.Command
	Namespace  string
	SystemStop func(service string) error
}

func NewCmdSystemStop() *CmdSystemStop {

	skupperCmd := CmdSystemStop{}

	return &skupperCmd
}

func (cmd *CmdSystemStop) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemStop = bootstrap.Stop
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
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

	err := cmd.SystemStop(cmd.Namespace)

	if err != nil {
		return fmt.Errorf("failed to stop router: %s", err)
	}

	fmt.Printf("%s-skupper-router is now stopped \n", cmd.Namespace)

	return nil
}

func (cmd *CmdSystemStop) WaitUntil() error { return nil }
