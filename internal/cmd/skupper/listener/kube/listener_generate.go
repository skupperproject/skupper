package kube

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdListenerGenerate struct {
	client         skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd       *cobra.Command
	Flags          *common.CommandListenerGenerateFlags
	namespace      string
	name           string
	port           int
	host           string
	tlsCredentials string
	listenerType   string
	routingKey     string
	output         string
}

func NewCmdListenerGenerate() *CmdListenerGenerate {

	return &CmdListenerGenerate{}

}

func (cmd *CmdListenerGenerate) NewClient(cobraCommand *cobra.Command, args []string) {

	cmd.namespace = cobraCommand.Flag("namespace").Value.String()
}

func (cmd *CmdListenerGenerate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

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

	if cmd.Flags != nil && cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
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

func (cmd *CmdListenerGenerate) InputToOptions() {
	// default host and routingkey to name of listener
	if cmd.Flags.Host == "" {
		cmd.host = cmd.name
	} else {
		cmd.host = cmd.Flags.Host
	}
	if cmd.Flags.RoutingKey == "" {
		cmd.routingKey = cmd.name
	} else {
		cmd.routingKey = cmd.Flags.RoutingKey
	}

	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.listenerType = cmd.Flags.ListenerType
	cmd.output = cmd.Flags.Output
}

func (cmd *CmdListenerGenerate) Run() error {

	resource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.ListenerSpec{
			Host:           cmd.host,
			Port:           cmd.port,
			RoutingKey:     cmd.routingKey,
			TlsCredentials: cmd.tlsCredentials,
			Type:           cmd.listenerType,
		},
	}

	encodedOutput, err := utils.Encode(cmd.output, resource)
	fmt.Println(encodedOutput)
	return err
}

func (cmd *CmdListenerGenerate) WaitUntil() error { return nil }
