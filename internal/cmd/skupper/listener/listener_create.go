package listener

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	listenerCreateLong    = "Clients at this site use the listener host and port to establish connections to the remote service."
	listenerCreateExample = "skupper listener create database 5432"

	listenerTypes = []string{"tcp"}
)

type ListenerFlags struct {
	routingKey   string
	host         string
	tlsSecret    string
	listenerType string
}

type CmdListenerCreate struct {
	client     skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd   cobra.Command
	flags      ListenerFlags
	namespace  string
	name       string
	port       int
	KubeClient kubernetes.Interface
}

func NewCmdListenerCreate() *CmdListenerCreate {

	skupperCmd := CmdListenerCreate{flags: ListenerFlags{}}

	cmd := cobra.Command{
		Use:     "create <name>",
		Short:   "create a listener",
		Long:    listenerCreateLong,
		Example: listenerCreateExample,
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

func (cmd *CmdListenerCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	kubeconfig := os.Getenv("KUBECONFIG")
	cli, err := client.NewClient("", "", kubeconfig)
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.KubeClient
}

func (cmd *CmdListenerCreate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.routingKey, "routing-key", "r", "", "The identifier used to route traffic from listeners to connectors")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.host, "host", "", "The hostname or IP address of the local listener")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.tlsSecret, "tls-secret", "", "The name of a Kubernetes secret containing TLS credentials")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.listenerType, "type", "t", "tcp", "The listener type. Choices: [tcp].")
}

func (cmd *CmdListenerCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(listenerTypes)

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
		cmd.name = args[0]
		ok, err := resourceStringValidator.Evaluate(cmd.name)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener name is not valid: %s", err))
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
		if listener != nil && !errors.IsNotFound(err) && listener.Status.StatusMessage == "Ok" {
			validationErrors = append(validationErrors, fmt.Errorf("there is already a listener %s created for namespace %s", cmd.name, cmd.namespace))
		}
	}

	// Validate flags

	if cmd.flags.routingKey != "" {
		//TBD what characters are not allowed
		ok, err := resourceStringValidator.Evaluate(cmd.flags.routingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}
	if cmd.flags.host != "" {
		//TBD what characters are not allowed
		ok, err := resourceStringValidator.Evaluate(cmd.flags.host)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("host name is not valid: %s", err))
		}
	}
	if cmd.flags.tlsSecret != "" {
		//TBD what characters are allowed
		// check that the secret exists
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.flags.tlsSecret, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		}
	}
	if cmd.flags.listenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.flags.listenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		}
	}
	return validationErrors
}

func (cmd *CmdListenerCreate) Run() error {

	listener := v1alpha1.Listener{
		ObjectMeta: metav1.ObjectMeta{Name: cmd.name},
		Spec: v1alpha1.ListenerSpec{
			Host:           cmd.flags.host,
			Port:           cmd.port,
			RoutingKey:     cmd.flags.routingKey,
			TlsCredentials: cmd.flags.tlsSecret,
			Type:           cmd.flags.listenerType,
		},
	}

	_, err := cmd.client.Listeners(cmd.namespace).Create(context.TODO(), &listener, metav1.CreateOptions{})
	return err
}

func (cmd *CmdListenerCreate) WaitUntilReady() error {
	err := utils.NewSpinner("Waiting for status...", 5, func() error {

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
		return fmt.Errorf("Listner %q not ready yet, check the logs for more information\n", cmd.name)
	}

	fmt.Printf("Listener %q is ready\n", cmd.name)
	return nil
}
