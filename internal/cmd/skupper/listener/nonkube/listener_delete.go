package nonkube

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
)

type CmdListenerDelete struct {
	listenerHandler *fs.ListenerHandler
	CobraCmd        *cobra.Command
	Flags           *common.CommandListenerDeleteFlags
	namespace       string
	listenerName    string
}

func NewCmdListenerDelete() *CmdListenerDelete {
	return &CmdListenerDelete{}
}

func (cmd *CmdListenerDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)

}

func (cmd *CmdListenerDelete) ValidateInput(args []string) []error {
	var validationErrors []error

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	resourceStringValidator := validator.NewResourceStringValidator()

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must be specified"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener name is not valid: %s", err))
		} else {
			cmd.listenerName = args[0]
		}
	}
	return validationErrors
}

func (cmd *CmdListenerDelete) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
}

func (cmd *CmdListenerDelete) Run() error {
	err := cmd.listenerHandler.Delete(cmd.listenerName)
	if err != nil {
		return err
	}
	return nil
}

func (cmd *CmdListenerDelete) WaitUntil() error { return nil }
