package kube

import (
	"context"
	"fmt"
	"strconv"
	"time"

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
	listenerCreateLong    = "Clients at this site use the listener host and port to establish connections to the remote service."
	listenerCreateExample = "skupper listener create database 5432"
)

type ListenerCreate struct {
	routingKey   string
	host         string
	tlsSecret    string
	listenerType string
	timeout      time.Duration
	output       string
}

type CmdListenerCreate struct {
	client     skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd   cobra.Command
	flags      ListenerCreate
	namespace  string
	name       string
	port       int
	output     string
	activeSite *v1alpha1.Site
	KubeClient kubernetes.Interface
}

func NewCmdListenerCreate() *CmdListenerCreate {

	skupperCmd := CmdListenerCreate{flags: ListenerCreate{}}

	cmd := cobra.Command{
		Use:     "create <name> <port>",
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
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdListenerCreate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.routingKey, "routing-key", "r", "", "The identifier used to route traffic from listeners to connectors")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.host, "host", "", "The hostname or IP address of the local listener")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.tlsSecret, "tls-secret", "t", "", "The name of a Kubernetes secret containing TLS credentials")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.listenerType, "type", "tcp", "The listener type. Choices: [tcp].")
	cmd.CobraCmd.Flags().DurationVar(&cmd.flags.timeout, "timeout", 60*time.Second, "Raise an error if the operation does not complete in the given period of time.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdListenerCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(utils.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

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

	// Validate there is already a site defined in the namespace before a listener can be created
	siteList, _ := cmd.client.Sites(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList == nil || len(siteList.Items) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("A site must exist in namespace %s before a listener can be created", cmd.namespace))
	} else {
		for _, s := range siteList.Items {
			if s.Status.Status.StatusMessage == "OK" && s.Status.Active {
				cmd.activeSite = &s
			}
		}
		if cmd.activeSite == nil {
			validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
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
		ok, err := resourceStringValidator.Evaluate(cmd.flags.routingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}

	//TBD what characters are not allowed for host flag

	if cmd.flags.tlsSecret != "" {
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

	//TBD what is valid timeout
	if cmd.flags.timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.flags.output
		}
	}
	return validationErrors
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
			Host:           cmd.flags.host,
			Port:           cmd.port,
			RoutingKey:     cmd.flags.routingKey,
			TlsCredentials: cmd.flags.tlsSecret,
			Type:           cmd.flags.listenerType,
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

func (cmd *CmdListenerCreate) WaitUntilReady() error {
	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	waitTime := int(cmd.flags.timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for create to complete...", waitTime, func() error {

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
