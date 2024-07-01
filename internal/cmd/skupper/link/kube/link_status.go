package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"text/tabwriter"
)

var (
	linkStatusLong = `Display the status of links in the current site.`
)

type CmdLinkStatus struct {
	Client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	Namespace string
}

func NewCmdLinkStatus() *CmdLinkStatus {

	skupperCmd := CmdLinkStatus{}

	cmd := cobra.Command{
		Use:     "status",
		Short:   "Display the status",
		Long:    linkStatusLong,
		Example: "skupper link status",
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdLinkStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}
func (cmd *CmdLinkStatus) AddFlags() {}
func (cmd *CmdLinkStatus) ValidateInput(args []string) []error {
	var validationErrors []error

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if siteList == nil || len(siteList.Items) == 0 || err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site available"))
	}

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not need any arguments"))
	}

	return validationErrors
}

func (cmd *CmdLinkStatus) InputToOptions() {}
func (cmd *CmdLinkStatus) Run() error {

	linkList, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return err
	}

	if linkList != nil && len(linkList.Items) == 0 {
		fmt.Println("There are no link resources in the namespace")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "NAME\tSTATUS\tCOST")

	for _, link := range linkList.Items {
		fmt.Fprintf(writer, "%s\t%s\t%d", link.Name, link.Status.StatusMessage, link.Spec.Cost)
		fmt.Fprintln(writer)
	}

	writer.Flush()
	return nil
}
func (cmd *CmdLinkStatus) WaitUntilReady() error { return nil }
