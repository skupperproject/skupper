package kube

import (
	"context"
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdConnectorDelete struct {
	client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandConnectorDeleteFlags
	namespace string
	name      string
	wait      bool
}

func NewCmdConnectorDelete() *CmdConnectorDelete {

	return &CmdConnectorDelete{}
}

func (cmd *CmdConnectorDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdConnectorDelete) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must be specified"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector name is not valid: %s", err))
		} else {
			cmd.name = args[0]
		}
		// Validate that there is already a connector with this name in the namespace
		if cmd.name != "" {
			connector, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
			if err != nil || connector == nil {
				validationErrors = append(validationErrors, fmt.Errorf("connector %s does not exist in namespace %s", cmd.name, cmd.namespace))
			}
		}

		if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
			ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
			}
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdConnectorDelete) Run() error {
	err := cmd.client.Connectors(cmd.namespace).Delete(context.TODO(), cmd.name, metav1.DeleteOptions{})
	return err
}

func (cmd *CmdConnectorDelete) WaitUntil() error {

	if cmd.wait {
		waitTime := int(cmd.Flags.Timeout.Seconds())
		err := utils.NewSpinnerWithTimeout("Waiting for deletion to complete...", waitTime, func() error {

			resource, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
			if err == nil && resource != nil {
				return fmt.Errorf("error deleting the resource")
			} else {
				return nil
			}
		})

		if err != nil {
			return fmt.Errorf("Connector %q not deleted yet, check the status for more information %s\n", cmd.name, err)
		}

		fmt.Printf("Connector %q deleted\n", cmd.name)

	}
	return nil
}

func (cmd *CmdConnectorDelete) InputToOptions() {
	cmd.wait = cmd.Flags.Wait
}
