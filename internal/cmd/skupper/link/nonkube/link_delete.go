package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdLinkDelete struct {
	linkHandler *fs.LinkHandler
	CobraCmd    *cobra.Command
	Flags       *common.CommandLinkDeleteFlags
	namespace   string
	linkName    string
}

func NewCmdLinkDelete() *CmdLinkDelete {
	return &CmdLinkDelete{}
}

func (cmd *CmdLinkDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.linkHandler = fs.NewLinkHandler(cmd.namespace)
}

func (cmd *CmdLinkDelete) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	// Validate arguments name if specified
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("link name must not be empty"))
		} else {
			ok, err := resourceStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("link name is not valid: %s", err))
			} else {
				cmd.linkName = args[0]
			}
		}
	} else {
		validationErrors = append(validationErrors, fmt.Errorf("link name must be specified"))
	}

	if cmd.linkName != "" {
		// Validate that there is a link with this name
		_, err := cmd.linkHandler.Get(cmd.linkName, fs.GetOptions{LogWarning: false, RuntimeFirst: false})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("There is no link resource in the namespace with the name %q", cmd.linkName))
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkDelete) Run() error {
	err := cmd.linkHandler.Delete(cmd.linkName)
	if err != nil {
		return err
	}
	return nil
}

func (cmd *CmdLinkDelete) InputToOptions()  {}
func (cmd *CmdLinkDelete) WaitUntil() error { return nil }
