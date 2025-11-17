package nonkube

import (
	"errors"
	"fmt"
	"net"
	"strconv"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdListenerGenerate struct {
	listenerHandler *fs.ListenerHandler
	CobraCmd        *cobra.Command
	Flags           *common.CommandListenerGenerateFlags
	namespace       string
	listenerName    string
	port            int
	host            string
	tlsCredentials  string
	listenerType    string
	routingKey      string
	output          string
}

func NewCmdListenerGenerate() *CmdListenerGenerate {
	return &CmdListenerGenerate{}
}

func (cmd *CmdListenerGenerate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
}

func (cmd *CmdListenerGenerate) ValidateInput(args []string) error {
	var validationErrors []error

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	hostStringValidator := validator.NewHostStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

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
			cmd.listenerName = args[0]
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
	if cmd.Flags.RoutingKey != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.RoutingKey)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("routing key is not valid: %s", err))
		}
	}

	if cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		}
	}

	if cmd.Flags.Host != "" {
		ip := net.ParseIP(cmd.Flags.Host)
		ok, _ := hostStringValidator.Evaluate(cmd.Flags.Host)
		if !ok && ip == nil {
			validationErrors = append(validationErrors, fmt.Errorf("host is not valid: a valid IP address or hostname is expected"))
		}
	}

	if cmd.Flags.TlsCredentials != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.TlsCredentials)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("tlsCredentials is not valid: %s", err))
		}
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdListenerGenerate) InputToOptions() {
	// default routingkey to name of listener
	if cmd.Flags.RoutingKey == "" {
		cmd.routingKey = cmd.listenerName
	} else {
		cmd.routingKey = cmd.Flags.RoutingKey
	}

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

	if cmd.Flags.Host == "" {
		cmd.host = "0.0.0.0"
	} else {
		cmd.host = cmd.Flags.Host
	}

	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.listenerType = cmd.Flags.ListenerType
	cmd.output = cmd.Flags.Output
}

func (cmd *CmdListenerGenerate) Run() error {
	listenerResource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.listenerName,
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

	encodedOutput, err := utils.Encode(cmd.output, listenerResource)
	fmt.Println(encodedOutput)
	return err

}

func (cmd *CmdListenerGenerate) WaitUntil() error { return nil }
