package kube

import (
	"context"
	"fmt"
	"strconv"

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
	connectorCreateLong    = "Clients at this site use the connector host and port to establish connections to the remote service."
	connectorCreateExample = "skupper connector create database 5432"
)

type ConnectorCreate struct {
	routingKey      string
	host            string
	selector        string
	tlsSecret       string
	connectorType   string
	includeNotReady bool
	workload        string
	output          string
}

type CmdConnectorCreate struct {
	client     skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd   cobra.Command
	flags      ConnectorCreate
	namespace  string
	name       string
	port       int
	output     string
	KubeClient kubernetes.Interface
}

func NewCmdConnectorCreate() *CmdConnectorCreate {

	skupperCmd := CmdConnectorCreate{flags: ConnectorCreate{}}

	cmd := cobra.Command{
		Use:     "create <name>",
		Short:   "create a connector",
		Long:    connectorCreateLong,
		Example: connectorCreateExample,
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

func (cmd *CmdConnectorCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorCreate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.routingKey, "routing-key", "r", "", "The identifier used to route traffic from Connectors to connectors")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.host, "host", "", "The hostname or IP address of the local Connector")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.tlsSecret, "tls-secret", "", "The name of a Kubernetes secret containing TLS credentials")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.connectorType, "type", "t", "tcp", "The Connector type. Choices: [tcp].")
	cmd.CobraCmd.Flags().BoolVarP(&cmd.flags.includeNotReady, "include-not-ready", "i", false, "If true, include server pods that are not in the ready state.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.selector, "selector", "s", "", "A Kubernetes label selector for specifying target server pods.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.workload, "workload", "w", "", "A Kubernetes label selector for specifying target server pods.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdConnectorCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(connectorTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)
	workloadStringValidator := validator.NewWorkloadStringValidator()

	// Validate arguments name and port
	if len(args) < 2 {
		validationErrors = append(validationErrors, fmt.Errorf("connector name and port must be configured"))
	} else if len(args) > 2 {
		validationErrors = append(validationErrors, fmt.Errorf("only two arguments are allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must not be empty"))
	} else if args[1] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("connector port must not be empty"))
	} else {
		cmd.name = args[0]
		ok, err := resourceStringValidator.Evaluate(cmd.name)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector name is not valid: %s", err))
		}

		cmd.port, err = strconv.Atoi(args[1])
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("connector port is not valid: %s", err))
		}
		ok, err = numberValidator.Evaluate(cmd.port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector port is not valid: %s", err))
		}
	}

	// Validate if there is already a Connector with this name in the namespace
	if cmd.name != "" {
		Connector, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if Connector != nil && !errors.IsNotFound(err) && Connector.Status.StatusMessage == "Ok" {
			validationErrors = append(validationErrors, fmt.Errorf("there is already a connector %s created for namespace %s", cmd.name, cmd.namespace))
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
		// check that the secret exists
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.flags.tlsSecret, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		}
	}
	if cmd.flags.connectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.flags.connectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		}
	}
	//TBD what are valid values here
	if cmd.flags.selector != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.flags.selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
	}
	//TBD no workload in connector CRD
	if cmd.flags.workload != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.flags.workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		}
	}
	// workload, selector or host must be specified
	if cmd.flags.workload == "" && cmd.flags.selector == "" && cmd.flags.host == "" {
		validationErrors = append(validationErrors, fmt.Errorf("One of the following options must be set: workload, selector, host"))
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

func (cmd *CmdConnectorCreate) Run() error {

	resource := v1alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v1alpha1.ConnectorSpec{
			Host:            cmd.flags.host,
			Port:            cmd.port,
			RoutingKey:      cmd.flags.routingKey,
			TlsCredentials:  cmd.flags.tlsSecret,
			Type:            cmd.flags.connectorType,
			IncludeNotReady: cmd.flags.includeNotReady,
			//Workload:       cmd.flags.workload,
			Selector: cmd.flags.selector,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)
		return err
	} else {
		_, err := cmd.client.Connectors(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
		return err
	}
}

func (cmd *CmdConnectorCreate) WaitUntilReady() error {
	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for create to complete...", 5, func() error {

		resource, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.Status.StatusMessage == "Ok" {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Connector %q not ready yet, check the logs for more information\n", cmd.name)
	}

	fmt.Printf("Connector %q is ready\n", cmd.name)
	return nil
}
