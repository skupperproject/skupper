package kube

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	listenerStatusLong    = "Display status of all listeners or a specific listener"
	listenerStatusExample = "skupper listener status backend"
)

type ListenerStatus struct {
	output string
}

type CmdListenerStatus struct {
	client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	flags     ListenerStatus
	namespace string
	name      string
	output    string
}

func NewCmdListenerStatus() *CmdListenerStatus {

	skupperCmd := CmdListenerStatus{flags: ListenerStatus{}}

	cmd := cobra.Command{
		Use:     "status <name>",
		Short:   "get status of listeners",
		Long:    listenerStatusLong,
		Example: listenerStatusExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdListenerStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdListenerStatus) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print status of listeners Choices: json, yaml")
}

func (cmd *CmdListenerStatus) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

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
				cmd.name = args[0]
			}
		}
	}

	// Validate that there is a listener with this name in the namespace
	if cmd.name != "" {
		listener, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil || listener == nil {
			validationErrors = append(validationErrors, fmt.Errorf("listener %s does not exist in namespace %s", cmd.name, cmd.namespace))
		}
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.flags.output
		}
	}

	return validationErrors
}
func (cmd *CmdListenerStatus) Run() error {
	if cmd.name == "" {
		resources, err := cmd.client.Listeners(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil || resources == nil || len(resources.Items) == 0 {
			fmt.Println("No listeners found")
			return err
		}
		if cmd.output != "" {
			for _, resource := range resources.Items {
				encodedOutput, err := utils.Encode(cmd.output, resource)
				if err != nil {
					return err
				}
				fmt.Println(encodedOutput)
			}
		} else {
			tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
			_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s",
				"NAME", "STATUS", "ROUTING-KEY", "HOST", "PORT", "MATCHING-CONNECTORS"))
			for _, resource := range resources.Items {
				fmt.Fprintln(tw, fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%d",
					resource.Name, resource.Status.StatusMessage, resource.Spec.RoutingKey, resource.Spec.Host,
					resource.Spec.Port, resource.Status.MatchingConnectorCount))
			}
			_ = tw.Flush()
		}
	} else {
		resource, err := cmd.client.Listeners(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if resource == nil || errors.IsNotFound(err) {
			fmt.Println("No listeners found")
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
			fmt.Fprintln(tw, fmt.Sprintf("Name:\t%s\nStatus:\t%s\nRouting key:\t%s\nHost:\t%s\nPort:\t%d\nConnectors:\t%d\n",
				resource.Name, resource.Status.StatusMessage, resource.Spec.RoutingKey, resource.Spec.Host,
				resource.Spec.Port, resource.Status.MatchingConnectorCount))
			_ = tw.Flush()
		}
	}

	return nil
}

func (cmd *CmdListenerStatus) InputToOptions()  {}
func (cmd *CmdListenerStatus) WaitUntil() error { return nil }
