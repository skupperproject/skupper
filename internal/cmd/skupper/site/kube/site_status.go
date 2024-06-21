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
	siteStatusLong = `Display the current status of a site.`
)

type CmdSiteStatus struct {
	Client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	Namespace string
}

func NewCmdSiteStatus() *CmdSiteStatus {

	skupperCmd := CmdSiteStatus{}

	cmd := cobra.Command{
		Use:     "status",
		Short:   "Get the site status",
		Long:    siteStatusLong,
		Example: "skupper site status",
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdSiteStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), "")
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}
func (cmd *CmdSiteStatus) AddFlags() {}
func (cmd *CmdSiteStatus) ValidateInput(args []string) []error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not need any arguments"))
	}

	return validationErrors
}

func (cmd *CmdSiteStatus) InputToOptions() {}
func (cmd *CmdSiteStatus) Run() error {

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return err
	}

	if siteList != nil && len(siteList.Items) == 0 {
		fmt.Println("There is no existing Skupper site resource")
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "NAME\tSTATUS")

	for _, site := range siteList.Items {
		fmt.Fprintf(writer, "%s\t%s", site.Name, site.Status.StatusMessage)
		fmt.Fprintln(writer)
	}

	writer.Flush()
	return nil
}
func (cmd *CmdSiteStatus) WaitUntilReady() error { return nil }
