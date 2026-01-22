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

type CmdConnectorStatus struct {
	connectorHandler *fs.ConnectorHandler
	CobraCmd         *cobra.Command
	Flags            *common.CommandConnectorStatusFlags
	namespace        string
	connectorName    string
	output           string
}

func NewCmdConnectorStatus() *CmdConnectorStatus {
	return &CmdConnectorStatus{}
}

func (cmd *CmdConnectorStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
}

func (cmd *CmdConnectorStatus) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// Validate arguments name if specified
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("connector name must not be empty"))
		} else {
			ok, err := resourceStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("connector name is not valid: %s", err))
			} else {
				cmd.connectorName = args[0]
			}
		}
	}
	// Validate that there is a connector with this name in the namespace
	if cmd.connectorName != "" {
		connector, err := cmd.connectorHandler.Get(cmd.connectorName, opts)
		if connector == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("connector %s does not exist in namespace %s", cmd.connectorName, cmd.namespace))
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

func (cmd *CmdConnectorStatus) Run() error {
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: true}
	if cmd.connectorName == "" {
		resources, err := cmd.connectorHandler.List(fs.GetOptions{RuntimeFirst: true})
		if err != nil || resources == nil || len(resources) == 0 {
			fmt.Println("No connectors found")
			return err
		}
		if cmd.output != "" {
			for _, resource := range resources {
				encodedOutput, err := utils.Encode(cmd.output, resource)
				if err != nil {
					return err
				}
				fmt.Println(encodedOutput)
			}
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
			_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s",
				"NAME", "STATUS", "ROUTING-KEY", "SELECTOR", "HOST", "PORT", "HAS MATCHING LISTENER", "MESSAGE"))
			for _, resource := range resources {
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d\t%t\t%s",
					resource.Name, resource.Status.StatusType, resource.Spec.RoutingKey,
					resource.Spec.Selector, resource.Spec.Host, resource.Spec.Port, resource.Status.HasMatchingListener, resource.Status.Message))
			}
			_ = tw.Flush()
		}
	} else {
		resource, err := cmd.connectorHandler.Get(cmd.connectorName, opts)
		if err != nil || resource == nil {
			fmt.Println("No connectors found")
			return err
		}
		if cmd.output != "" {
			encodedOutput, err := utils.Encode(cmd.output, resource)
			if err != nil {
				return err
			}
			fmt.Println(encodedOutput)
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
			fmt.Fprintln(tw, fmt.Sprintf("Name:\t%s\nStatus:\t%s\nRouting key:\t%s\nSelector:\t%s\nHost:\t%s\nPort:\t%d\nHas Matching Listener:%t\nMessage:\t%s\n",
				resource.Name, resource.Status.StatusType, resource.Spec.RoutingKey, resource.Spec.Selector,
				resource.Spec.Host, resource.Spec.Port, resource.Status.HasMatchingListener, resource.Status.Message))
			_ = tw.Flush()
		}
	}

	return nil
}

func (cmd *CmdConnectorStatus) InputToOptions()  {}
func (cmd *CmdConnectorStatus) WaitUntil() error { return nil }
