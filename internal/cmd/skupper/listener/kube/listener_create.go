package kube

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdListenerCreate struct {
	client         skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd       *cobra.Command
	Flags          *common.CommandListenerCreateFlags
	namespace      string
	name           string
	port           int
	host           string
	tlsCredentials string
	listenerType   string
	routingKey     string
	timeout        time.Duration
	output         string
	KubeClient     kubernetes.Interface
	status         string
}

func NewCmdListenerCreate() *CmdListenerCreate {

	return &CmdListenerCreate{}

}

func (cmd *CmdListenerCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdListenerCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	statusValidator := validator.NewOptionValidator(common.WaitStatusTypes)

	// Validate arguments name and port
	if len(args) < 2 {
		validationErrors = append(validationErrors, fmt.Errorf("listener name and port must be configured"))
	} else if len(args) > 2 {
		validationErrors = append(validationErrors, fmt.Errorf("only two arguments are allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must not be empty"))
	} else if args[1] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("listener port must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener name is not valid: %s", err))
		} else {
			cmd.name = args[0]
		}

		cmd.port, err = strconv.Atoi(args[1])
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("listener port is not valid: %s", err))
		}
		ok, err = numberValidator.Evaluate(cmd.port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener port is not valid: %s", err))
		}
	}

	// Validate if there is already a listener with this name in the namespace
	if cmd.name != "" {
		listener, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if listener != nil && !errors.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("there is already a listener %s created for namespace %s", cmd.name, cmd.namespace))
		}
	}

	// Validate flags
	if cmd.Flags != nil && cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.TlsCredentials != "" {
		// check that the secret exists
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.Flags.TlsCredentials, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		}
	}

	if cmd.Flags != nil && cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Wait != "" {
		ok, err := statusValidator.Evaluate(cmd.Flags.Wait)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("status is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdListenerCreate) InputToOptions() {
	// default host and routingkey to name of listener
	if cmd.Flags.Host == "" {
		cmd.host = cmd.name
	} else {
		cmd.host = cmd.Flags.Host
	}
	if cmd.Flags.RoutingKey == "" {
		cmd.routingKey = cmd.name
	} else {
		cmd.routingKey = cmd.Flags.RoutingKey
	}
	cmd.timeout = cmd.Flags.Timeout
	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.listenerType = cmd.Flags.ListenerType
	cmd.output = cmd.Flags.Output
	cmd.status = cmd.Flags.Wait
}

func (cmd *CmdListenerCreate) Run() error {

	resource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.ListenerSpec{
			Host:           cmd.host,
			Port:           cmd.port,
			RoutingKey:     cmd.routingKey,
			TlsCredentials: cmd.tlsCredentials,
			Type:           cmd.listenerType,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)
		return err
	} else {
		_, err := cmd.client.Listeners(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
		return err
	}
}

func (cmd *CmdListenerCreate) WaitUntil() error {
	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	if cmd.status == "none" {
		return nil
	}

	waitTime := int(cmd.timeout.Seconds())
	var listenerCondition *metav1.Condition

	err := utils.NewSpinnerWithTimeout("Waiting for create to complete...", waitTime, func() error {

		resource, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isConditionFound := false
		isConditionTrue := false

		switch cmd.status {
		case "ready":
			listenerCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_READY)
		default:
			listenerCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_CONFIGURED)
		}

		if listenerCondition != nil {
			isConditionFound = true
			isConditionTrue = listenerCondition.Status == metav1.ConditionTrue
		}

		if resource != nil && isConditionFound && isConditionTrue {
			return nil
		}

		if resource != nil && isConditionFound && !isConditionTrue {
			return fmt.Errorf("error in the condition")
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil && listenerCondition == nil {
		return fmt.Errorf("Listener %q is not %s yet, check the status for more information\n", cmd.name, cmd.status)
	} else if err != nil && listenerCondition.Status == metav1.ConditionFalse {
		return fmt.Errorf("Listener %q is %s with errors, check the status for more information\n", cmd.name, cmd.status)
	}

	fmt.Printf("Listener %q is %s.\n", cmd.name, cmd.status)
	return nil
}
