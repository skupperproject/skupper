package kube

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdListenerCreate struct {
	client       skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd     *cobra.Command
	Flags        *common.CommandListenerCreateFlags
	namespace    string
	name         string
	port         int
	host         string
	tlsSecret    string
	listenerType string
	routingKey   string
	timeout      time.Duration
	output       string
	KubeClient   kubernetes.Interface
}

func NewCmdListenerCreate() *CmdListenerCreate {

	return &CmdListenerCreate{}

}

func (cmd *CmdListenerCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdListenerCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

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
	if cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}

	//TBD what characters are not allowed for host flag

	if cmd.Flags.TlsSecret != "" {
		// check that the secret exists
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.Flags.TlsSecret, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		}
	}

	if cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		}
	}

	//TBD what is valid timeout
	if cmd.Flags.Timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
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
	cmd.tlsSecret = cmd.Flags.TlsSecret
	cmd.listenerType = cmd.Flags.ListenerType
	cmd.output = cmd.Flags.Output
}

func (cmd *CmdListenerCreate) Run() error {

	resource := v1alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v1alpha1.ListenerSpec{
			Host:           cmd.host,
			Port:           cmd.port,
			RoutingKey:     cmd.routingKey,
			TlsCredentials: cmd.tlsSecret,
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

	waitTime := int(cmd.timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for create to complete...", waitTime, func() error {

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
