package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	siteUpdateLong = `Change site settings of a given site.`
)

type UpdateFlags struct {
	enableLinkAccess bool
	linkAccessType   string
	serviceAccount   string
	output           string
}

type CmdSiteUpdate struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd           cobra.Command
	flags              UpdateFlags
	options            map[string]string
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
}

func NewCmdSiteUpdate() *CmdSiteUpdate {

	skupperCmd := CmdSiteUpdate{}

	cmd := cobra.Command{
		Use:     "update <name>",
		Short:   "Change site settings",
		Long:    siteUpdateLong,
		Example: "skupper site update my-site --enable-link-access",
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			skupperCmd.InputToOptions()
			utils.HandleError(skupperCmd.Run())
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			return skupperCmd.WaitUntilReady()
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), "")
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteUpdate) AddFlags() {
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableLinkAccess, "enable-link-access", false, "allow access for incoming links from remote sites (default: false)")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessType, "link-access-type", "", `configure external access for links from remote sites.
Choices: [route|loadbalancer]. Default: On OpenShift, route is the default; 
for other Kubernetes flavors, loadbalancer is the default.`)
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.serviceAccount, "service-account", "skupper-controller", "the Kubernetes service account under which to run the Skupper controller")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdSiteUpdate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(utils.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no existing Skupper site resource to update"))
	} else if len(siteList.Items) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("there are several sites in this namespace and and it should be only one"))
	} else {
		currentSite := siteList.Items[0]

		if len(args) > 1 {
			validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
		} else if len(args) <= 1 {

			if len(args) > 0 && currentSite.Name != args[0] {
				validationErrors = append(validationErrors, fmt.Errorf("site with name %q is not available", args[0]))
			} else {
				cmd.siteName = currentSite.Name
			}
		}
	}

	if cmd.flags.linkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.flags.linkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if !cmd.flags.enableLinkAccess && len(cmd.flags.linkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.flags.serviceAccount != "" {
		ok, err := resourceStringValidator.Evaluate(cmd.flags.serviceAccount)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("service account name is not valid: %s", err))
		}
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}
func (cmd *CmdSiteUpdate) InputToOptions() {
	cmd.serviceAccountName = cmd.flags.serviceAccount

	if cmd.flags.enableLinkAccess {
		if cmd.flags.linkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.flags.linkAccessType
		}
	} else {
		cmd.linkAccessType = "none"
	}

	options := make(map[string]string)
	if cmd.siteName != "" {
		options[site.SiteConfigNameKey] = cmd.siteName
	}

	cmd.options = options

	cmd.output = cmd.flags.output

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

func (cmd *CmdSiteUpdate) WaitUntilReady() error {

	// the site resource was not updated
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for update to complete...", 5, func() error {

		resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.Status.StatusMessage == "OK" {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Site %q not ready yet, check the logs for more information\n", cmd.siteName)
	}

	fmt.Printf("Site %q is updated\n", cmd.siteName)

	return nil
}
