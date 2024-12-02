package kube

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdListenerDelete struct {
	client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandListenerDeleteFlags
	namespace string
	name      string
	wait      bool
}

func NewCmdListenerDelete() *CmdListenerDelete {

	return &CmdListenerDelete{}
}

func (cmd *CmdListenerDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdListenerDelete) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

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
			cmd.name = args[0]
		}

		if cmd.name != "" {
			// Validate that there is already a listener with this name in the namespace
			listener, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
			if err != nil || listener == nil {
				validationErrors = append(validationErrors, fmt.Errorf("listener %s does not exist in namespace %s", cmd.name, cmd.namespace))
			}
		}

		if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
			ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
			}
		}
	}

	return validationErrors
}

func (cmd *CmdListenerDelete) Run() error {
	err := cmd.client.Listeners(cmd.namespace).Delete(context.TODO(), cmd.name, metav1.DeleteOptions{})
	return err
}

func (cmd *CmdListenerDelete) WaitUntil() error {

	if cmd.wait {
		waitTime := int(cmd.Flags.Timeout.Seconds())
		err := utils.NewSpinnerWithTimeout("Waiting for deletion to complete...", waitTime, func() error {

			resource, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
			if err == nil && resource != nil {
				return fmt.Errorf("error deleting the resource")
			} else {
				return nil
			}
		})

		if err != nil {
			return fmt.Errorf("Listener %q not deleted yet, check the status for more information %s\n", cmd.name, err)
		}

		fmt.Printf("Listener %q deleted\n", cmd.name)
	}
	return nil
}

func (cmd *CmdListenerDelete) InputToOptions() {
	cmd.wait = cmd.Flags.Wait
}
