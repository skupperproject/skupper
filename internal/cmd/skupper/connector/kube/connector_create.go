package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"strconv"
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

type ConnectorCreate struct {
	routingKey      string
	host            string
	selector        string
	tlsSecret       string
	connectorType   string
	includeNotReady bool
	workload        string
	timeout         time.Duration
	output          string
}

type CmdConnectorCreate struct {
	client     skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandConnectorCreateFlags
	namespace  string
	name       string
	port       int
	output     string
	KubeClient kubernetes.Interface
}

func NewCmdConnectorCreate() *CmdConnectorCreate {

	return &CmdConnectorCreate{}
}

func (cmd *CmdConnectorCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorCreate) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
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
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector name is not valid: %s", err))
		} else {
			cmd.name = args[0]
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

	// Validate there is already a site defined in the namespace before a connector can be created
	siteList, _ := cmd.client.Sites(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList == nil || len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("A site must exist in namespace %s before a connector can be created", cmd.namespace))
	} else {
		if !utils.SiteConfigured(siteList) {
			validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
		}
	}

	// Validate if there is already a Connector with this name in the namespace
	if cmd.name != "" {
		connector, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if connector != nil && !errors.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("there is already a connector %s created for namespace %s", cmd.name, cmd.namespace))
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
	if cmd.Flags.ConnectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.Flags.ConnectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		}
	}
	if cmd.Flags.Selector != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.Flags.Selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
	}
	//TBD no workload in connector CRD
	if cmd.Flags.Workload != "" {
		ok, err := workloadStringValidator.Evaluate(cmd.Flags.Workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		}
	}
	//TBD what is valid timeout
	if cmd.Flags.Timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}
	// workload, selector or host must be specified
	if cmd.Flags.Workload == "" && cmd.Flags.Selector == "" && cmd.Flags.Host == "" {
		validationErrors = append(validationErrors, fmt.Errorf("One of the following options must be set: workload, selector, host"))
	}
	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.Flags.Output
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
			Host:            cmd.Flags.Host,
			Port:            cmd.port,
			RoutingKey:      cmd.Flags.RoutingKey,
			TlsCredentials:  cmd.Flags.TlsSecret,
			Type:            cmd.Flags.ConnectorType,
			IncludeNotReady: cmd.Flags.IncludeNotReady,
			Selector:        cmd.Flags.Selector,
			//Workload:       cmd.Flags.workload,
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

func (cmd *CmdConnectorCreate) WaitUntil() error {
	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	waitTime := int(cmd.Flags.Timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for create to complete...", waitTime, func() error {

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

func (cmd *CmdConnectorCreate) InputToOptions() {}
