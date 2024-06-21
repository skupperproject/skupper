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
	siteDeleteLong = `Delete a site by name`
)

type CmdSiteDelete struct {
	Client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	Namespace string
	siteName  string
}

func NewCmdSiteDelete() *CmdSiteDelete {

	skupperCmd := CmdSiteDelete{}

	cmd := cobra.Command{
		Use:     "delete",
		Short:   "Delete a site",
		Long:    siteDeleteLong,
		Example: "skupper site delete my-site",
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

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), "")
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteDelete) AddFlags() {}

func (cmd *CmdSiteDelete) ValidateInput(args []string) []error {
	var validationErrors []error

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command."))
	} else {

		//Validate if there is already a site defined in the namespace
		site, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), args[0], metav1.GetOptions{})
		if err != nil {
			validationErrors = append(validationErrors, err)
		} else if site == nil {
			validationErrors = append(validationErrors, fmt.Errorf("there is no site with name %q", args[0]))
		} else {
			cmd.siteName = args[0]
		}
	}

	return validationErrors
}
func (cmd *CmdSiteDelete) InputToOptions() {}

func (cmd *CmdSiteDelete) Run() error {
	err := cmd.Client.Sites(cmd.Namespace).Delete(context.TODO(), cmd.siteName, metav1.DeleteOptions{})
	return err
}
func (cmd *CmdSiteDelete) WaitUntilReady() error {
	err := utils.NewSpinner("Waiting for deletion to complete...", 5, func() error {

		resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})

		if err == nil && resource != nil {
			return fmt.Errorf("error deleting the resource")
		} else {
			return nil
		}

	})

	if err != nil {
		return fmt.Errorf("Site %q not deleted yet, check the logs for more information\n", cmd.siteName)
	}

	fmt.Printf("Site %q is deleted\n", cmd.siteName)

	return nil
}