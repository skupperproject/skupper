package nonkube

import (
	"errors"
	"fmt"
	"net"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ListenerUpdates struct {
	routingKey     string
	host           string
	tlsCredentials string
	listenerType   string
	port           int
}
type CmdListenerUpdate struct {
	listenerHandler *fs.ListenerHandler
	CobraCmd        *cobra.Command
	Flags           *common.CommandListenerUpdateFlags
	namespace       string
	listenerName    string
	newSettings     ListenerUpdates
}

func NewCmdListenerUpdate() *CmdListenerUpdate {
	return &CmdListenerUpdate{}
}

func (cmd *CmdListenerUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
}

func (cmd *CmdListenerUpdate) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	listenerTypeValidator := validator.NewOptionValidator(common.ListenerTypes)
	hostStringValidator := validator.NewHostStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("listener name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener name is not valid: %s", err))
		} else {
			cmd.listenerName = args[0]
		}
	}

	// Validate that there is already a listener with this name in the namespace
	if cmd.listenerName != "" {
		listener, err := cmd.listenerHandler.Get(cmd.listenerName, opts)
		if listener == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("listener %s must exist in namespace %s to be updated", cmd.listenerName, cmd.namespace))
		} else {
			// save existing values
			cmd.newSettings.host = listener.Spec.Host
			cmd.newSettings.port = listener.Spec.Port
			cmd.newSettings.tlsCredentials = listener.Spec.TlsCredentials
			cmd.newSettings.listenerType = listener.Spec.Type
			cmd.newSettings.routingKey = listener.Spec.RoutingKey
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
	if cmd.Flags.Host != "" {
		ip := net.ParseIP(cmd.Flags.Host)
		ok, _ := hostStringValidator.Evaluate(cmd.Flags.Host)
		if !ok && ip == nil {
			validationErrors = append(validationErrors, fmt.Errorf("host is not valid: a valid IP address or hostname is expected"))
		} else {
			cmd.newSettings.host = cmd.Flags.Host
		}
	}
	if cmd.Flags.TlsCredentials != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.Flags.TlsCredentials)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("tlsCredentials value is not valid: %s", err))
		} else {
			cmd.newSettings.tlsCredentials = cmd.Flags.TlsCredentials
		}
	}
	if cmd.Flags.ListenerType != "" {
		ok, err := listenerTypeValidator.Evaluate(cmd.Flags.ListenerType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener type is not valid: %s", err))
		} else {
			cmd.newSettings.listenerType = cmd.Flags.ListenerType
		}
	}
	if cmd.Flags.Port != 0 {
		ok, err := numberValidator.Evaluate(cmd.Flags.Port)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("listener port is not valid: %s", err))
		} else {
			cmd.newSettings.port = cmd.Flags.Port
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdListenerUpdate) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
	// user wants to clear TlsCredentials
	if cmd.CobraCmd.Flags().Changed(common.FlagNameTlsCredentials) && cmd.Flags.TlsCredentials == "" {
		cmd.newSettings.tlsCredentials = ""
	}
}

func (cmd *CmdListenerUpdate) Run() error {
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
			Host:           cmd.newSettings.host,
			Port:           cmd.newSettings.port,
			RoutingKey:     cmd.newSettings.routingKey,
			TlsCredentials: cmd.newSettings.tlsCredentials,
			Type:           cmd.newSettings.listenerType,
		},
	}

	err := cmd.listenerHandler.Add(listenerResource)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *CmdListenerUpdate) WaitUntil() error { return nil }
