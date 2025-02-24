package kube

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	pkgUtils "github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdConnectorCreate struct {
	client              skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd            *cobra.Command
	Flags               *common.CommandConnectorCreateFlags
	namespace           string
	name                string
	port                int
	host                string
	selector            string
	tlsCredentials      string
	routingKey          string
	connectorType       string
	includeNotReadyPods bool
	timeout             time.Duration
	KubeClient          kubernetes.Interface
	status              string
}

func NewCmdConnectorCreate() *CmdConnectorCreate {

	return &CmdConnectorCreate{}
}

func (cmd *CmdConnectorCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorCreate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	workloadStringValidator := validator.NewWorkloadStringValidator(common.WorkloadTypes)
	selectorStringValidator := validator.NewSelectorStringValidator()
	statusValidator := validator.NewOptionValidator(common.WaitStatusTypes)
	hostStringValidator := validator.NewHostStringValidator()

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

	// Validate if there is already a Connector with this name in the namespace
	if cmd.name != "" {
		connector, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if connector != nil && !k8serrs.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("There is already a connector %s created for namespace %s", cmd.name, cmd.namespace))
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
	if cmd.Flags != nil && cmd.Flags.ConnectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.Flags.ConnectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		}
	}
	// only one of workload, selector or host can be specified
	if cmd.Flags != nil && cmd.Flags.Host != "" {
		if cmd.Flags.Workload != "" || cmd.Flags.Selector != "" {
			validationErrors = append(validationErrors, fmt.Errorf("If host is configured, cannot configure workload or selector"))
		}
		ip := net.ParseIP(cmd.Flags.Host)
		ok, _ := hostStringValidator.Evaluate(cmd.Flags.Host)
		if !ok && ip == nil {
			validationErrors = append(validationErrors, fmt.Errorf("host is not valid: a valid IP address or hostname is expected"))
		}
	}
	if cmd.Flags != nil && cmd.Flags.Selector != "" {
		if cmd.Flags.Workload != "" || cmd.Flags.Host != "" {
			validationErrors = append(validationErrors, fmt.Errorf("If selector is configured, cannot configure workload or host"))
		}
		ok, err := selectorStringValidator.Evaluate(cmd.Flags.Selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
		cmd.selector = cmd.Flags.Selector
	}
	if cmd.Flags != nil && cmd.Flags.Workload != "" {
		if cmd.Flags.Selector != "" || cmd.Flags.Host != "" {
			validationErrors = append(validationErrors, fmt.Errorf("If workload is configured, cannot configure selector or host"))
		}
		//workload get resource-type/resource-name and find selector labels
		resourceType, resourceName, ok, err := workloadStringValidator.Evaluate(cmd.Flags.Workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		} else {
			switch resourceType {
			case "deployment":
				deployment, err := cmd.KubeClient.AppsV1().Deployments(cmd.namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
				if err != nil {
					validationErrors = append(validationErrors, fmt.Errorf("failed trying to get Deployment specified by workload: %s", err))
				} else {
					if deployment.Spec.Selector.MatchLabels != nil {
						cmd.selector = pkgUtils.StringifySelector(deployment.Spec.Selector.MatchLabels)
					} else {
						validationErrors = append(validationErrors, fmt.Errorf("workload, no selector Matchlabels found"))
					}
				}
			case "service":
				service, err := cmd.KubeClient.CoreV1().Services(cmd.namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
				if err != nil {
					validationErrors = append(validationErrors, fmt.Errorf("failed trying to get Service specified by workload: %s", err))
				} else {
					if service.Spec.Selector != nil {
						cmd.selector = pkgUtils.StringifySelector(service.Spec.Selector)
					} else {
						validationErrors = append(validationErrors, fmt.Errorf("workload, no selector labels found"))
					}
				}
			case "daemonset":
				daemonSet, err := cmd.KubeClient.AppsV1().DaemonSets(cmd.namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
				if err != nil {
					validationErrors = append(validationErrors, fmt.Errorf("failed trying to get DaemonSet specified by workload: %s", err))
				} else {
					if daemonSet.Spec.Selector.MatchLabels != nil {
						cmd.selector = pkgUtils.StringifySelector(daemonSet.Spec.Selector.MatchLabels)
					} else {
						validationErrors = append(validationErrors, fmt.Errorf("workload, no selector Matchlabels found"))
					}
				}
			case "statefulset":
				statefulSet, err := cmd.KubeClient.AppsV1().StatefulSets(cmd.namespace).Get(context.TODO(), resourceName, metav1.GetOptions{})
				if err != nil {
					validationErrors = append(validationErrors, fmt.Errorf("failed trying to get StatefulSet specified by workload: %s", err))
				} else {
					if statefulSet.Spec.Selector.MatchLabels != nil {
						cmd.selector = pkgUtils.StringifySelector(statefulSet.Spec.Selector.MatchLabels)
					} else {
						validationErrors = append(validationErrors, fmt.Errorf("workload, no selector Matchlabels found"))
					}
				}
			}
		}
	}
	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Wait != "" {
		ok, err := statusValidator.Evaluate(cmd.Flags.Wait)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("status is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdConnectorCreate) InputToOptions() {

	// workload, selector or host must be specified
	if cmd.Flags.Workload == "" && cmd.Flags.Selector == "" && cmd.Flags.Host == "" {
		// default selector to name of connector
		cmd.selector = "app=" + cmd.name
	}
	if cmd.Flags.Host != "" {
		cmd.host = cmd.Flags.Host
	}
	if cmd.Flags.Selector != "" {
		cmd.selector = cmd.Flags.Selector
	}

	// default routingkey to name of connector
	if cmd.Flags.RoutingKey == "" {
		cmd.routingKey = cmd.name
	} else {
		cmd.routingKey = cmd.Flags.RoutingKey
	}
	cmd.timeout = cmd.Flags.Timeout
	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.connectorType = cmd.Flags.ConnectorType
	cmd.includeNotReadyPods = cmd.Flags.IncludeNotReadyPods
	cmd.status = cmd.Flags.Wait
}

func (cmd *CmdConnectorCreate) Run() error {

	resource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.ConnectorSpec{
			Host:                cmd.host,
			Port:                cmd.port,
			RoutingKey:          cmd.routingKey,
			TlsCredentials:      cmd.tlsCredentials,
			Type:                cmd.connectorType,
			IncludeNotReadyPods: cmd.includeNotReadyPods,
			Selector:            cmd.selector,
		},
	}

	_, err := cmd.client.Connectors(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err
}

func (cmd *CmdConnectorCreate) WaitUntil() error {

	if cmd.status == "none" {
		return nil
	}

	waitTime := int(cmd.timeout.Seconds())

	var connectorCondition *metav1.Condition

	err := utils.NewSpinnerWithTimeout("Waiting for create to complete...", waitTime, func() error {

		resource, err := cmd.client.Connectors(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isConditionFound := false
		isConditionTrue := false

		switch cmd.status {
		case "ready":
			connectorCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_READY)
		default:
			connectorCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_CONFIGURED)

		}

		if connectorCondition != nil {
			isConditionFound = true
			isConditionTrue = connectorCondition.Status == metav1.ConditionTrue
		}

		if resource != nil && isConditionFound && isConditionTrue {
			return nil
		}

		if resource != nil && isConditionFound && !isConditionTrue {
			return fmt.Errorf("error in the condition")
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil && connectorCondition == nil {
		return fmt.Errorf("Connector %q is not yet %s, check the status for more information\n", cmd.name, cmd.status)
	} else if err != nil && connectorCondition.Status == metav1.ConditionFalse {
		return fmt.Errorf("Connector %q is not yet %s: %s\n", cmd.name, cmd.status, connectorCondition.Message)
	}

	fmt.Printf("Connector %q is %s.\n", cmd.name, cmd.status)
	return nil
}
