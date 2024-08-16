package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	utils2 "github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
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
	CobraCmd        *cobra.Command
	Flags           *common.CommandConnectorUpdateFlags
	namespace       string
	name            string
	resourceVersion string
	newSettings     ConnectorUpdates
	KubeClient      kubernetes.Interface
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {

	return &CmdConnectorUpdate{}

}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils2.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorUpdate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
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
	if cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		} else {
			cmd.newSettings.routingKey = cmd.Flags.RoutingKey
		}
	}
	//TBD what characters are not allowed for host flag
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
	if cmd.Flags.ConnectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.Flags.ConnectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		} else {
			cmd.newSettings.connectorType = cmd.Flags.ConnectorType
		}
	}
	if cmd.Flags.Port != 0 {
		ok, err := numberValidator.Evaluate(cmd.Flags.Port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.Flags.Port
		}
	}
	//TBD what are valid values here
	if cmd.Flags.Selector != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.Flags.Selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
		cmd.newSettings.selector = cmd.Flags.Selector
	}
	if cmd.Flags.Workload != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.Flags.Workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		}
		cmd.newSettings.selector = cmd.Flags.Workload
	}
	//TBD what is valid timeout ---> use timevalidator
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
			//Workload:       cmd.newSettings.workload, //TODO: if this flag is not used we should reconsider its removal
			Selector:        cmd.newSettings.selector,
			IncludeNotReady: cmd.newSettings.includeNotReady,
		},
	}

	if cmd.newSettings.output != "" {
		encodedOutput, err := utils2.Encode(cmd.newSettings.output, resource)
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

	waitTime := int(cmd.Flags.Timeout.Seconds())
	err := utils2.NewSpinnerWithTimeout("Waiting for update to complete...", waitTime, func() error {

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
