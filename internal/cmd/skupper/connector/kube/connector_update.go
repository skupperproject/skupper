package kube

import (
	"context"
	"errors"
	"fmt"
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

type ConnectorUpdates struct {
	routingKey          string
	host                string
	tlsCredentials      string
	connectorType       string
	port                int
	workload            string
	selector            string
	includeNotReadyPods bool
	timeout             time.Duration
}

type CmdConnectorUpdate struct {
	client           skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd         *cobra.Command
	Flags            *common.CommandConnectorUpdateFlags
	namespace        string
	name             string
	resourceVersion  string
	newSettings      ConnectorUpdates
	existingHost     string
	existingSelector string
	KubeClient       kubernetes.Interface
	status           string
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {

	return &CmdConnectorUpdate{}

}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
	cmd.KubeClient = cli.Kube
}

func (cmd *CmdConnectorUpdate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	workloadStringValidator := validator.NewWorkloadStringValidator(common.WorkloadTypes)
	selectorStringValidator := validator.NewSelectorStringValidator()
	statusValidator := validator.NewOptionValidator(common.WaitStatusTypes)

	// Check if Connector CRD is installed
	_, err := cmd.client.Connectors(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}

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
		if connector == nil || k8serrs.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("connector %s must exist in namespace %s to be updated", cmd.name, cmd.namespace))
		} else {
			// save existing values
			cmd.resourceVersion = connector.ResourceVersion
			cmd.newSettings.port = connector.Spec.Port
			cmd.newSettings.tlsCredentials = connector.Spec.TlsCredentials
			cmd.newSettings.connectorType = connector.Spec.Type
			cmd.newSettings.includeNotReadyPods = connector.Spec.IncludeNotReadyPods
			cmd.newSettings.routingKey = connector.Spec.RoutingKey
			cmd.existingHost = connector.Spec.Host
			cmd.existingSelector = connector.Spec.Selector
		}
	}

	// Validate flags
	if cmd.Flags != nil && cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		} else {
			cmd.newSettings.routingKey = cmd.Flags.RoutingKey
		}
	}
	if cmd.Flags != nil && cmd.Flags.TlsCredentials != "" {
		_, err := cmd.KubeClient.CoreV1().Secrets(cmd.namespace).Get(context.TODO(), cmd.Flags.TlsCredentials, metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("tls-secret is not valid: does not exist"))
		} else {
			cmd.newSettings.tlsCredentials = cmd.Flags.TlsCredentials
		}
	}
	if cmd.Flags != nil && cmd.Flags.ConnectorType != "" {
		ok, err := connectorTypeValidator.Evaluate(cmd.Flags.ConnectorType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector type is not valid: %s", err))
		} else {
			cmd.newSettings.connectorType = cmd.Flags.ConnectorType
		}
	}
	if cmd.Flags != nil && cmd.Flags.Port != 0 {
		ok, err := numberValidator.Evaluate(cmd.Flags.Port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("connector port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.Flags.Port
		}
	}
	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}
	//TBD what characters are not allowed for host flag
	if cmd.Flags != nil && cmd.Flags.Host != "" {
		cmd.newSettings.host = cmd.Flags.Host
	}
	if cmd.Flags != nil && cmd.Flags.Selector != "" {
		ok, err := selectorStringValidator.Evaluate(cmd.Flags.Selector)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("selector is not valid: %s", err))
		}
		cmd.newSettings.selector = cmd.Flags.Selector
	}
	//workload get resource-type/resource-name and find selector labels
	if cmd.Flags != nil && cmd.Flags.Workload != "" {
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
						cmd.newSettings.workload = pkgUtils.StringifySelector(deployment.Spec.Selector.MatchLabels)
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
						cmd.newSettings.workload = pkgUtils.StringifySelector(service.Spec.Selector)
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
						cmd.newSettings.workload = pkgUtils.StringifySelector(daemonSet.Spec.Selector.MatchLabels)
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
						cmd.newSettings.workload = pkgUtils.StringifySelector(statefulSet.Spec.Selector.MatchLabels)
					} else {
						validationErrors = append(validationErrors, fmt.Errorf("workload, no selector Matchlabels found"))
					}
				}
			}
		}
	}
	//Can only have one workload/Selector/host set
	if cmd.newSettings.host == "" && cmd.newSettings.selector == "" && cmd.newSettings.workload == "" {
		// no host/selector/workload being modified use existing values
		cmd.newSettings.selector = cmd.existingSelector
		cmd.newSettings.host = cmd.existingHost
	} else {
		if cmd.newSettings.host != "" && (cmd.newSettings.workload != "" || cmd.newSettings.selector != "") {
			validationErrors = append(validationErrors, fmt.Errorf("If host is configured, cannot configure workload or selector"))
		}
		if cmd.newSettings.selector != "" && (cmd.newSettings.host != "" || cmd.newSettings.workload != "") {
			validationErrors = append(validationErrors, fmt.Errorf("If selector is configured, cannot configure workload or host"))
		}
		if cmd.newSettings.workload != "" {
			if cmd.newSettings.selector != "" || cmd.newSettings.host != "" {
				validationErrors = append(validationErrors, fmt.Errorf("If workload is configured, cannot configure selector or host"))
			}
			cmd.newSettings.selector = cmd.newSettings.workload
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

func (cmd *CmdConnectorUpdate) Run() error {

	resource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            cmd.name,
			Namespace:       cmd.namespace,
			ResourceVersion: cmd.resourceVersion,
		},
		Spec: v2alpha1.ConnectorSpec{
			Host:                cmd.newSettings.host,
			Port:                cmd.newSettings.port,
			RoutingKey:          cmd.newSettings.routingKey,
			TlsCredentials:      cmd.newSettings.tlsCredentials,
			Type:                cmd.newSettings.connectorType,
			Selector:            cmd.newSettings.selector,
			IncludeNotReadyPods: cmd.newSettings.includeNotReadyPods,
		},
	}

	_, err := cmd.client.Connectors(cmd.namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
	return err
}

func (cmd *CmdConnectorUpdate) WaitUntil() error {

	if cmd.status == "none" {
		return nil
	}

	waitTime := int(cmd.Flags.Timeout.Seconds())
	var connectorCondition *metav1.Condition
	err := utils.NewSpinnerWithTimeout("Waiting for update to complete...", waitTime, func() error {

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

func (cmd *CmdConnectorUpdate) InputToOptions() {
	cmd.status = cmd.Flags.Wait
}
