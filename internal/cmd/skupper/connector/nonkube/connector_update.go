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

type ConnectorUpdates struct {
	routingKey     string
	host           string
	connectorType  string
	port           int
	tlsCredentials string
}
type CmdConnectorUpdate struct {
	connectorHandler *fs.ConnectorHandler
	CobraCmd         *cobra.Command
	Flags            *common.CommandConnectorUpdateFlags
	namespace        string
	connectorName    string
	newSettings      ConnectorUpdates
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {
	return &CmdConnectorUpdate{}
}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
}

func (cmd *CmdConnectorUpdate) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	connectorTypeValidator := validator.NewOptionValidator(common.ConnectorTypes)
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
			cmd.connectorName = args[0]
		}
	}

	// Validate that there is already a connector with this name in the namespace
	if cmd.connectorName != "" {
		connector, err := cmd.connectorHandler.Get(cmd.connectorName, opts)
		if connector == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("connector %s must exist in namespace %s to be updated", cmd.connectorName, cmd.namespace))
		} else {
			// save existing values
			cmd.newSettings.host = connector.Spec.Host
			cmd.newSettings.port = connector.Spec.Port
			cmd.newSettings.connectorType = connector.Spec.Type
			cmd.newSettings.tlsCredentials = connector.Spec.TlsCredentials
			cmd.newSettings.routingKey = connector.Spec.RoutingKey
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
	if cmd.Flags.Host != "localhost" {
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
	return errors.Join(validationErrors...)
}

func (cmd *CmdConnectorUpdate) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
}

func (cmd *CmdConnectorUpdate) Run() error {
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.connectorName,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.ConnectorSpec{
			Host:           cmd.newSettings.host,
			Port:           cmd.newSettings.port,
			RoutingKey:     cmd.newSettings.routingKey,
			TlsCredentials: cmd.newSettings.tlsCredentials,
			Type:           cmd.newSettings.connectorType,
		},
	}

	err := cmd.connectorHandler.Add(connectorResource)
	return err

}

func (cmd *CmdConnectorUpdate) WaitUntil() error { return nil }
