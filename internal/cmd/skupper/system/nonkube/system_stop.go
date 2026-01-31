package nonkube

import (
	"errors"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
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
	var validationErrors []error
	namespaceStringValidator := validator.NamespaceStringValidator()

	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)

	if systemReloadType == types.SystemReloadTypeAuto {
		validationErrors = append(validationErrors, fmt.Errorf("this command is disabled because automatic reloading is configured"))
	}

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
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
