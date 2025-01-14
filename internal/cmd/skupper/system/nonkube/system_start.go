package nonkube

import (
	"fmt"

	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemStart struct {
	CobraCmd    *cobra.Command
	Namespace   string
	SystemStart func(service string) error
}

func NewCmdCmdSystemStart() *CmdSystemStart {

	skupperCmd := CmdSystemStart{}

	return &skupperCmd
}

func (cmd *CmdSystemStart) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemStart = bootstrap.Start
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemStart) ValidateInput(args []string) []error {
	var validationErrors []error

	if args != nil && len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	return validationErrors
}

func (cmd *CmdSystemStart) InputToOptions() {

	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}
}

func (cmd *CmdSystemStart) Run() error {
	err := cmd.SystemStart(cmd.Namespace)

	if err != nil {
		return fmt.Errorf("failed to start router: %s", err)
	}

	fmt.Printf("%s-skupper-router is now started\n", cmd.Namespace)

	return nil
}

func (cmd *CmdSystemStart) WaitUntil() error { return nil }
