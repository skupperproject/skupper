package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"text/tabwriter"
)

type CmdLinkStatus struct {
	Client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandLinkStatusFlags
	Namespace string
	output    string
	linkName  string
}

func NewCmdLinkStatus() *CmdLinkStatus {

	return &CmdLinkStatus{}
}

func (cmd *CmdLinkStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkStatus) ValidateInput(args []string) []error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if siteList == nil || len(siteList.Items) == 0 || err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site available"))
	}

	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("this command only accepts one argument"))
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	if len(args) >= 1 && args[0] != "" {
		cmd.linkName = args[0]
	}

	return validationErrors
}

func (cmd *CmdLinkStatus) InputToOptions() {
	cmd.output = cmd.Flags.Output
}
func (cmd *CmdLinkStatus) Run() error {

	if cmd.linkName != "" {

		selectedLink, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if cmd.output != "" {
			return printEncodedOuptut(cmd.output, selectedLink)
		} else {
			displaySingleLink(selectedLink)
		}

	} else {

		linkList, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return err
		}

		if linkList != nil && len(linkList.Items) == 0 {
			fmt.Println("There are no link resources in the namespace")
			return nil
		}

		if cmd.output != "" {
			for _, link := range linkList.Items {
				err := printEncodedOuptut(cmd.output, &link)

				if err != nil {
					return err
				}
			}
		} else {
			displayLinkList(linkList.Items)
		}

	}

	return nil
}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }

func printEncodedOuptut(outputType string, link *v1alpha1.Link) error {
	encodedOutput, err := utils.Encode(outputType, link)
	fmt.Println(encodedOutput)
	return err
}

func displaySingleLink(link *v1alpha1.Link) {
	fmt.Printf("%s\t: %s\n", "Name", link.Name)
	fmt.Printf("%s\t: %s\n", "Status", link.Status.StatusMessage)
	fmt.Printf("%s\t: %d\n", "Cost", link.Spec.Cost)
}

func displayLinkList(linkList []v1alpha1.Link) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "NAME\tSTATUS\tCOST")

	for _, link := range linkList {
		fmt.Fprintf(writer, "%s\t%s\t%d", link.Name, link.Status.StatusMessage, link.Spec.Cost)
		fmt.Fprintln(writer)
	}

	writer.Flush()
}
