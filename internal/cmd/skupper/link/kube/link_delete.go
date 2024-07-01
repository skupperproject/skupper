package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	linkDeleteLong = `Delete a link by name`
)

type CmdLinkDelete struct {
	Client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	Namespace string
	linkName  string
}

func NewCmdLinkDelete() *CmdLinkDelete {

	skupperCmd := CmdLinkDelete{}

	cmd := cobra.Command{
		Use:     "delete <name>",
		Short:   "Delete a link",
		Long:    linkDeleteLong,
		Example: "skupper site delete my-link",
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.Run())
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return skupperCmd.WaitUntilReady()
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdLinkDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkDelete) AddFlags() {}

func (cmd *CmdLinkDelete) ValidateInput(args []string) []error {
	var validationErrors []error

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site in this namespace"))
	}

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("link name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else {
		cmd.linkName = args[0]

		link, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if link == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("the link %q is not available in the namespace", cmd.linkName))
		}
	}

	return validationErrors
}
func (cmd *CmdLinkDelete) InputToOptions() {}

func (cmd *CmdLinkDelete) Run() error {
	err := cmd.Client.Links(cmd.Namespace).Delete(context.TODO(), cmd.linkName, metav1.DeleteOptions{})
	return err
}
func (cmd *CmdLinkDelete) WaitUntilReady() error {
	err := utils.NewSpinner("Waiting for deletion to complete...", 5, func() error {

		resource, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})

		if err == nil && resource != nil {
			return fmt.Errorf("error deleting the resource")
		} else {
			return nil
		}

	})

	if err != nil {
		return fmt.Errorf("Link %q not deleted yet, check the logs for more information\n", cmd.linkName)
	}

	fmt.Printf("Link %q is deleted\n", cmd.linkName)

	return nil
}
