package kube

import (
	"context"
	"fmt"
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
	connectorUpdateLong = `Clients at this site use the connector host and port to establish connections to the remote service.
	The user can change port, host name, TLS secret, selector, connector type and routing key`
	connectorUpdateExample = "skupper connector update database --host mysql --port 3306"

	connectorTypes = []string{"tcp"}
)

type ConnectorUpdates struct {
	routingKey      string
	host            string
	tlsSecret       string
	connectorType   string
	port            int
	workload        string
	selector        string
	includeNotReady bool
	timeout         time.Duration
	output          string
}

type CmdConnectorUpdate struct {
	client          skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd        cobra.Command
	flags           ConnectorUpdates
	namespace       string
	name            string
	resourceVersion string
	newSettings     ConnectorUpdates
	KubeClient      kubernetes.Interface
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {

	skupperCmd := CmdConnectorUpdate{flags: ConnectorUpdates{}}

	cmd := cobra.Command{
		Use:     "update <name>",
		Short:   "update a connector",
		Long:    connectorUpdateLong,
		Example: connectorUpdateExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return skupperCmd.WaitUntil()
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorUpdate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.routingKey, "routing-key", "r", "", "The identifier used to route traffic from connectors to connectors")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.host, "host", "", "The hostname or IP address of the local connector")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.tlsSecret, "tls-secret", "t", "", "The name of a Kubernetes secret containing TLS credentials")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.connectorType, "type", "tcp", "The connector type. Choices: [tcp|http].")
	cmd.CobraCmd.Flags().IntVar(&cmd.flags.port, "port", 0, "The port of the local connector")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.selector, "selector", "s", "", "A Kubernetes label selector for specifying target server pods.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.workload, "workload", "w", "", "A Kubernetes resource name that identifies a workload")
	cmd.CobraCmd.Flags().BoolVarP(&cmd.flags.includeNotReady, "include-not-ready", "i", false, "If set, include server pods that are not in the ready state.")
	cmd.CobraCmd.Flags().DurationVar(&cmd.flags.timeout, "timeout", 60*time.Second, "Raise an error if the operation does not complete in the given period of time.")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "Print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdConnectorUpdate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(connectorTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)
	workloadStringValidator := validator.NewWorkloadStringValidator()

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("connector name must be configured"))
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
	}

	// Validate that there is already a connector with this name in the namespace
	if cmd.name != "" {
		connector, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if connector == nil || errors.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("connector %s must exist in namespace %s to be updated", cmd.name, cmd.namespace))
		} else {
			// save existing values
			cmd.resourceVersion = connector.ResourceVersion
			cmd.newSettings.host = connector.Spec.Host
			cmd.newSettings.port = connector.Spec.Port
			cmd.newSettings.tlsSecret = connector.Spec.TlsCredentials
			cmd.newSettings.connectorType = connector.Spec.Type
			//cmd.newSettings.workload = connector.Spec.Workload
			cmd.newSettings.selector = connector.Spec.Selector
			cmd.newSettings.includeNotReady = connector.Spec.IncludeNotReady
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
	//TBD what characters are not allowed for host flag
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
	if cmd.flags.connectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.flags.connectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		} else {
			cmd.newSettings.connectorType = cmd.flags.connectorType
		}
	}
	if cmd.flags.port != 0 {
		ok, err := numberValidator.Evaluate(cmd.flags.port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.flags.port
		}
	}
	//TBD what are valid values here
	if cmd.flags.selector != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.flags.selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
		cmd.newSettings.selector = cmd.flags.selector
	}
	if cmd.flags.workload != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.flags.workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		}
		cmd.newSettings.selector = cmd.flags.workload
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
			cmd.newSettings.output = cmd.flags.output
		}
	}

	return validationErrors
}

func (cmd *CmdConnectorUpdate) Run() error {

	resource := v1alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            cmd.name,
			Namespace:       cmd.namespace,
			ResourceVersion: cmd.resourceVersion,
		},
		Spec: v1alpha1.ConnectorSpec{
			Host:           cmd.newSettings.host,
			Port:           cmd.newSettings.port,
			RoutingKey:     cmd.newSettings.routingKey,
			TlsCredentials: cmd.newSettings.tlsSecret,
			Type:           cmd.newSettings.connectorType,
			//Workload:       cmd.newSettings.workload,
			Selector:        cmd.newSettings.selector,
			IncludeNotReady: cmd.newSettings.includeNotReady,
		},
	}

	if cmd.newSettings.output != "" {
		encodedOutput, err := utils.Encode(cmd.newSettings.output, resource)
		fmt.Println(encodedOutput)
		return err
	} else {
		_, err := cmd.client.Connectors(cmd.namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
		return err
	}
}

func (cmd *CmdConnectorUpdate) WaitUntil() error {

	// the site resource was not created
	if cmd.newSettings.output != "" {
		return nil
	}

	waitTime := int(cmd.flags.timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for update to complete...", waitTime, func() error {

		resource, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.IsConfigured() {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Connector %q not ready yet, check the status for more information\n", cmd.name)
	}

	fmt.Printf("Connector %q is ready\n", cmd.name)
	return nil
}

func (cmd *CmdConnectorUpdate) InputToOptions() {}
