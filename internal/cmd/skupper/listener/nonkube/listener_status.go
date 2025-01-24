package nonkube

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdListenerStatus struct {
	listenerHandler *fs.ListenerHandler
	CobraCmd        *cobra.Command
	Flags           *common.CommandListenerStatusFlags
	namespace       string
	listenerName    string
	output          string
}

func NewCmdListenerStatus() *CmdListenerStatus {
	return &CmdListenerStatus{}
}

func (cmd *CmdListenerStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
}

func (cmd *CmdListenerStatus) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// Validate arguments name if specified
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("listener name must not be empty"))
		} else {
			ok, err := resourceStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("listener name is not valid: %s", err))
			} else {
				cmd.listenerName = args[0]
			}
		}
	}
	// Validate that there is a listener with this name in the namespace
	if cmd.listenerName != "" {
		listener, err := cmd.listenerHandler.Get(cmd.listenerName, opts)
		if listener == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("listener %s does not exist in namespace %s", cmd.listenerName, cmd.namespace))
		}
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.Flags.Output
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdListenerStatus) Run() error {
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: true}
	if cmd.listenerName == "" {
		listeners, err := cmd.listenerHandler.List()
		if listeners == nil || err != nil {
			fmt.Println("no listeners found:")
			return err
		}
		if cmd.output != "" {
			for _, listener := range listeners {
				encodedOutput, err := utils.Encode(cmd.output, listener)
				if err != nil {
					return err
				}
				fmt.Println(encodedOutput)
			}
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
			_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
				"NAME", "STATUS", "ROUTING-KEY", "HOST", "PORT"))
			for _, listener := range listeners {
				status := "Not Ready"
				if listener.IsConfigured() {
					status = "Ok"
				}
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%d",
					listener.Name, status, listener.Spec.RoutingKey, listener.Spec.Host, listener.Spec.Port))
			}
			_ = tw.Flush()
		}
	} else {
		listener, err := cmd.listenerHandler.Get(cmd.listenerName, opts)
		if listener == nil || err != nil {
			fmt.Println("No listeners found:", err)
			return err
		}
		if cmd.output != "" {
			encodedOutput, err := utils.Encode(cmd.output, listener)
			if err != nil {
				return err
			}
			fmt.Println(encodedOutput)
		} else {
			status := "Not Ready"
			if listener.IsConfigured() {
				status = "Ok"
			}
			tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
			fmt.Fprintln(tw, fmt.Sprintf("Name:\t%s\nStatus:\t%s\nRouting key:\t%s\nHost:\t%s\nPort:\t%d\nTlsCredentials:\t%s\n",
				listener.Name, status, listener.Spec.RoutingKey, listener.Spec.Host, listener.Spec.Port, listener.Spec.TlsCredentials))
			_ = tw.Flush()
		}
	}
	return nil
}

func (cmd *CmdListenerStatus) InputToOptions()  {}
func (cmd *CmdListenerStatus) WaitUntil() error { return nil }
