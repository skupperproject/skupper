package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"time"

	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteDelete struct {
	Client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandSiteDeleteFlags
	Namespace string
	siteName  string
	timeout   time.Duration
	wait      bool
}

func NewCmdSiteDelete() *CmdSiteDelete {

	skupperCmd := CmdSiteDelete{}

	return &skupperCmd
}

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteDelete) ValidateInput(args []string) []error {
	var validationErrors []error
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

	//Validate if there is already a site defined in the namespace
	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		validationErrors = append(validationErrors, err)
	} else if siteList == nil || (siteList != nil && len(siteList.Items) == 0) {
		validationErrors = append(validationErrors, fmt.Errorf("there is no existing Skupper site resource to delete"))
	} else {

		if len(args) > 1 {
			validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
		} else if len(args) == 1 {

			selectedSite := args[0]
			for _, s := range siteList.Items {
				if s.Name == selectedSite {
					cmd.siteName = s.Name
				}
			}

			if cmd.siteName == "" {
				validationErrors = append(validationErrors, fmt.Errorf("site with name %q is not available", selectedSite))
			}
		} else if len(args) == 0 {
			if len(siteList.Items) > 1 {
				validationErrors = append(validationErrors, fmt.Errorf("site name is required because there are several sites in this namespace"))
			} else if len(siteList.Items) == 1 {
				cmd.siteName = siteList.Items[0].Name
			}
		}
	}

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	return validationErrors
}
func (cmd *CmdSiteDelete) InputToOptions() {
	cmd.timeout = cmd.Flags.Timeout
	cmd.wait = cmd.Flags.Wait
}

func (cmd *CmdSiteDelete) Run() error {
	err := cmd.Client.Sites(cmd.Namespace).Delete(context.TODO(), cmd.siteName, metav1.DeleteOptions{})
	return err
}
func (cmd *CmdSiteDelete) WaitUntil() error {

	if cmd.wait {
		waitTime := int(cmd.timeout.Seconds())
		err := utils.NewSpinnerWithTimeout("Waiting for deletion to complete...", waitTime, func() error {

			resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})

			if err == nil && resource != nil {
				return fmt.Errorf("error deleting the resource")
			} else {
				return nil
			}

		})

		if err != nil {
			return fmt.Errorf("Site %q not deleted yet, check the status for more information\n", cmd.siteName)
		}

		fmt.Printf("Site %q is deleted\n", cmd.siteName)
	}

	return nil
}
