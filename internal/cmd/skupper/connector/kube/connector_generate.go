package kube

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdConnectorGenerate struct {
	client              skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd            *cobra.Command
	Flags               *common.CommandConnectorGenerateFlags
	namespace           string
	name                string
	port                int
	output              string
	host                string
	selector            string
	tlsCredentials      string
	routingKey          string
	connectorType       string
	includeNotReadyPods bool
}

func NewCmdConnectorGenerate() *CmdConnectorGenerate {

	return &CmdConnectorGenerate{}
}

func (cmd *CmdConnectorGenerate) NewClient(cobraCommand *cobra.Command, args []string) {

	cmd.namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdConnectorGenerate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	workloadStringValidator := validator.NewWorkloadStringValidator(common.WorkloadTypes)
	selectorStringValidator := validator.NewSelectorStringValidator()
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

	// Validate flags
	if cmd.Flags != nil && cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}
	if cmd.Flags != nil && cmd.Flags.TlsCredentials != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.TlsCredentials)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("tlsCredentials is not valid: %s", err))
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
		_, _, ok, err := workloadStringValidator.Evaluate(cmd.Flags.Workload)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("workload is not valid: %s", err))
		}
	}
	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdConnectorGenerate) InputToOptions() {
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
	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.connectorType = cmd.Flags.ConnectorType
	cmd.output = cmd.Flags.Output
	cmd.includeNotReadyPods = cmd.Flags.IncludeNotReadyPods
}

func (cmd *CmdConnectorGenerate) Run() error {
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

	encodedOutput, err := utils.Encode(cmd.output, resource)
	fmt.Println(encodedOutput)
	return err
}

func (cmd *CmdConnectorGenerate) WaitUntil() error { return nil }
