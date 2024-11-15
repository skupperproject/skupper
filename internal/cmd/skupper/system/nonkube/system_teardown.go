package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemTeardown struct {
	CobraCmd  *cobra.Command
	TearDown  func(namespace string) error
	Namespace string
}

func NewCmdSystemTeardown() *CmdSystemTeardown {

	skupperCmd := CmdSystemTeardown{}

	return &skupperCmd
}

func (cmd *CmdSystemTeardown) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.TearDown = bootstrap.Teardown
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdSystemTeardown) ValidateInput(args []string) []error {
	var validationErrors []error

	if args != nil && len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	return validationErrors
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
