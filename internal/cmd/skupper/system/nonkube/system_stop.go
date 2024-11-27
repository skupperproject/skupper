package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/nonkube/bootstrap"
	"github.com/spf13/cobra"
)

type CmdSystemStop struct {
	CobraCmd    *cobra.Command
	Namespace   string
	Platform    string
	ServiceStop func(service string, platform string) error
}

func NewCmdSystemStop() *CmdSystemStop {

	skupperCmd := CmdSystemStop{}

	return &skupperCmd
}

func (cmd *CmdSystemStop) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.ServiceStop = bootstrap.Stop
	cmd.Namespace = cobraCommand.Flag("namespace").Value.String()
	cmd.Platform = cobraCommand.Flag("platform").Value.String()
}

func (cmd *CmdSystemStop) ValidateInput(args []string) []error {
	var validationErrors []error

	if args != nil && len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	return validationErrors
}

func (cmd *CmdSystemStop) InputToOptions() {

	if cmd.Namespace == "" {
		cmd.Namespace = "default"
	}

}

func (cmd *CmdSystemStop) Run() error {

	err := cmd.ServiceStop(cmd.Namespace, cmd.Platform)

	if err != nil {
		return fmt.Errorf("Failed to stop nonkube service: %s", err)
	}

	fmt.Printf("skupper-%s.service is now stopped \n", cmd.Namespace)

	return nil
}

func (cmd *CmdSystemStop) WaitUntil() error { return nil }