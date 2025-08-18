package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdConnectorDelete struct {
	connectorHandler *fs.ConnectorHandler
	CobraCmd         *cobra.Command
	Flags            *common.CommandConnectorDeleteFlags
	namespace        string
	connectorName    string
}

func NewCmdConnectorDelete() *CmdConnectorDelete {
	return &CmdConnectorDelete{}
}

func (cmd *CmdConnectorDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
}

func (cmd *CmdConnectorDelete) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector name is not valid: %s", err))
		} else {
			cmd.connectorName = args[0]
		}
	}

	if cmd.connectorName != "" {
		// Validate that there is already a connector with this name
		connector, err := cmd.connectorHandler.Get(cmd.connectorName, opts)
		if connector == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("connector %s does not exist", cmd.connectorName))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdConnectorDelete) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
}

func (cmd *CmdConnectorDelete) Run() error {
	err := cmd.connectorHandler.Delete(cmd.connectorName)
	if err != nil {
		return err
	}
	return nil
}

func (cmd *CmdConnectorDelete) WaitUntil() error { return nil }
