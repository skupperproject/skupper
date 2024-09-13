package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"time"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type ListenerUpdates struct {
	routingKey   string
	host         string
	tlsSecret    string
	listenerType string
	port         int
	timeout      time.Duration
	output       string
}
type CmdListenerUpdate struct {
	client          skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd        *cobra.Command
	Flags           *common.CommandListenerUpdateFlags
	namespace       string
	name            string
	resourceVersion string
	newSettings     ListenerUpdates
	KubeClient      kubernetes.Interface
}

func NewCmdListenerUpdate() *CmdListenerUpdate {

	return &CmdListenerUpdate{}
}

func (cmd *CmdListenerUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdListenerUpdate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must be configured"))
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
	}

	// Validate that there is already a listener with this name in the namespace
	if cmd.name != "" {
		listener, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if listener == nil || errors.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("listener %s must exist in namespace %s to be updated", cmd.name, cmd.namespace))
		} else {
			// save existing values
			cmd.resourceVersion = listener.ResourceVersion
			cmd.newSettings.host = listener.Spec.Host
			cmd.newSettings.port = listener.Spec.Port
			cmd.newSettings.tlsSecret = listener.Spec.TlsCredentials
			cmd.newSettings.listenerType = listener.Spec.Type
		}
	}

	// Validate flags
	if cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		} else {
			cmd.newSettings.routingKey = cmd.Flags.RoutingKey
		}
	}
	// TBD what validation should be done
	if cmd.Flags.Host != "" {
		cmd.newSettings.host = cmd.Flags.Host
	}
	if cmd.Flags.TlsSecret != "" {
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.Flags.TlsSecret, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		} else {
			cmd.newSettings.tlsSecret = cmd.Flags.TlsSecret
		}
	}
	if cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		} else {
			cmd.newSettings.listenerType = cmd.Flags.ListenerType
		}
	}
	if cmd.Flags.Port != 0 {
		ok, err := numberValidator.Evaluate(cmd.Flags.Port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.Flags.Port
		}
	}
	//TBD what is valid timeout --> use timeoutvalidator
	if cmd.Flags.Timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}
	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.newSettings.output = cmd.Flags.Output
		}
	}

	return validationErrors
}

func (cmd *CmdListenerUpdate) Run() error {

	resource := v1alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            cmd.name,
			Namespace:       cmd.namespace,
			ResourceVersion: cmd.resourceVersion},
		Spec: v1alpha1.ListenerSpec{
			Host:           cmd.newSettings.host,
			Port:           cmd.newSettings.port,
			RoutingKey:     cmd.newSettings.routingKey,
			TlsCredentials: cmd.newSettings.tlsSecret,
			Type:           cmd.newSettings.listenerType,
		},
	}

	if cmd.newSettings.output != "" {
		encodedOutput, err := utils.Encode(cmd.newSettings.output, resource)
		fmt.Println(encodedOutput)
		return err
	} else {
		_, err := cmd.client.Listeners(cmd.namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
		return err
	}
}

func (cmd *CmdListenerUpdate) WaitUntil() error {

	// the site resource was not created
	if cmd.newSettings.output != "" {
		return nil
	}

	waitTime := int(cmd.Flags.Timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for update to complete...", waitTime, func() error {

		resource, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.IsConfigured() {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Listener %q not ready yet, check the status for more information\n", cmd.name)
	}

	fmt.Printf("Listener %q is ready\n", cmd.name)
	return nil
}

func (cmd *CmdListenerUpdate) InputToOptions() {}
