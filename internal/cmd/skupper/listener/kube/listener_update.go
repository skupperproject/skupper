package kube

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	listenerUpdateLong = `Clients at this site use the listener host and port to establish connections to the remote service.
	The user can change port, host name, TLS secret, listener type and routing key`
	listenerUpdateExample = "skupper listener update database --host mysql --port 3306"
)

type ListenerUpdates struct {
	routingKey   string
	host         string
	tlsSecret    string
	listenerType string
	port         int
	output       string
}

type CmdListenerUpdate struct {
	client          skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd        cobra.Command
	flags           ListenerUpdates
	namespace       string
	name            string
	resourceVersion string
	newSettings     ListenerUpdates
	KubeClient      kubernetes.Interface
}

func NewCmdListenerUpdate() *CmdListenerUpdate {

	skupperCmd := CmdListenerUpdate{flags: ListenerUpdates{}}

	cmd := cobra.Command{
		Use:     "update <name>",
		Short:   "update a listener",
		Long:    listenerUpdateLong,
		Example: listenerUpdateExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return skupperCmd.WaitUntilReady()
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdListenerUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdListenerUpdate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.routingKey, "routing-key", "r", "", "The identifier used to route traffic from listeners to connectors")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.host, "host", "", "The hostname or IP address of the local listener")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.tlsSecret, "tls-secret", "", "The name of a Kubernetes secret containing TLS credentials")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.listenerType, "type", "t", "tcp", "The listener type. Choices: [tcp|http].")
	cmd.CobraCmd.Flags().IntVar(&cmd.flags.port, "port", 0, "The port of the local listener")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdListenerUpdate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(utils.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

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
	if cmd.flags.routingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.flags.routingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		} else {
			cmd.newSettings.routingKey = cmd.flags.routingKey
		}
	}
	// TBD what validation should be done
	if cmd.flags.host != "" {
		cmd.newSettings.host = cmd.flags.host
	}
	if cmd.flags.tlsSecret != "" {
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.flags.tlsSecret, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		} else {
			cmd.newSettings.tlsSecret = cmd.flags.tlsSecret
		}
	}
	if cmd.flags.listenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.flags.listenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		} else {
			cmd.newSettings.listenerType = cmd.flags.listenerType
		}
	}
	if cmd.flags.port != 0 {
		ok, err := numberValidator.Evaluate(cmd.flags.port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.flags.port
		}
	}
	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.newSettings.output = cmd.flags.output
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

func (cmd *CmdListenerUpdate) WaitUntilReady() error {

	// the site resource was not created
	if cmd.newSettings.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for update to complete...", 5, func() error {

		resource, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.Status.StatusMessage == "Ok" {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Listener %q not ready yet, check the logs for more information\n", cmd.name)
	}

	fmt.Printf("Listener %q is ready\n", cmd.name)
	return nil
}