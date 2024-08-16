package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteUpdate struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           *cobra.Command
	Flags              *common.CommandSiteUpdateFlags
	options            map[string]string
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
}

func NewCmdSiteUpdate() *CmdSiteUpdate {

	skupperCmd := CmdSiteUpdate{}

	return &skupperCmd
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteUpdate) ValidateInput(args []string) []error {

	var validationErrors []error
	linkAccessTypeValidator := validator.NewOptionValidator(utils.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no existing Skupper site resource to update"))
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

	if cmd.Flags.LinkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.Flags.LinkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if !cmd.Flags.EnableLinkAccess && len(cmd.Flags.LinkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.Flags.ServiceAccount != "" {
		svcAccount, err := cmd.KubeClient.CoreV1().ServiceAccounts(cmd.Namespace).Get(context.TODO(), cmd.Flags.ServiceAccount, metav1.GetOptions{})
		if err != nil || svcAccount == nil {
			validationErrors = append(validationErrors, fmt.Errorf("service account name is not valid: %s", err))
		}
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}
func (cmd *CmdSiteUpdate) InputToOptions() {
	cmd.serviceAccountName = cmd.Flags.ServiceAccount

	if cmd.Flags.EnableLinkAccess {
		if cmd.Flags.LinkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.Flags.LinkAccessType
		}
	} else {
		cmd.linkAccessType = "none"
	}

	options := make(map[string]string)
	if cmd.siteName != "" {
		options[site.SiteConfigNameKey] = cmd.siteName
	}

	cmd.options = options

	cmd.output = cmd.Flags.Output

}
func (cmd *CmdSiteUpdate) Run() error {

	currentSite, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	updatedSettings := currentSite.Spec.Settings
	if len(cmd.options) != 0 {
		updatedSettings = cmd.options
	}

	updatedServiceAccount := currentSite.Spec.ServiceAccount
	if cmd.serviceAccountName != "" {
		updatedServiceAccount = cmd.serviceAccountName
	}

	updatedLinkAccessType := currentSite.Spec.LinkAccess
	if cmd.linkAccessType != "" {
		updatedLinkAccessType = cmd.linkAccessType
	}

	resource := v1alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              currentSite.Name,
			Namespace:         currentSite.Namespace,
			CreationTimestamp: currentSite.CreationTimestamp,
			ResourceVersion:   currentSite.ResourceVersion,
		},
		Spec: v1alpha1.SiteSpec{
			Settings:       updatedSettings,
			ServiceAccount: updatedServiceAccount,
			LinkAccess:     updatedLinkAccessType,
		},
		Status: currentSite.Status,
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		_, err := cmd.Client.Sites(cmd.Namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
		return err
	}
}

func (cmd *CmdSiteUpdate) WaitUntil() error {

	// the site resource was not updated
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for update to complete...", 5, func() error {

		resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.IsConfigured() {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Site %q not ready yet, check the status for more information\n", cmd.siteName)
	}

	fmt.Printf("Site %q is updated\n", cmd.siteName)

	return nil
}
