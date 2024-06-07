/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

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
	siteCreateLong = `A site is a place where components of your application are running. 
Sites are linked to form application networks.
There can be only one site definition per namespace.`

	linkAccessTypes = []string{"route", "loadbalancer", "nodeport", "nginx-ingress-v1", "contour-http-proxy", "ingress"}
)

type CreateFlags struct {
	enableLinkAccess bool
	linkAccessType   string
	linkAccessHost   string
	serviceAccount   string
}

type CmdSiteCreate struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd           cobra.Command
	flags              CreateFlags
	options            map[string]string
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	options := make(map[string]string)
	skupperCmd := CmdSiteCreate{options: options, flags: CreateFlags{}}

	cmd := cobra.Command{
		Use:    "create <name>",
		Short:  "Create a new site",
		Long:   siteCreateLong,
		PreRun: skupperCmd.NewClient,
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

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient("", "", "")
	utils.HandleError(err)

	cmd.Client = cli.SkupperClient.SkupperV1alpha1()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteCreate) AddFlags() {
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableLinkAccess, "enable-link-access", false, "Enable external access for links from remote sites")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessType, "link-access-type", "", `Select the means of opening external access 
One of: [route|loadbalancer|nodeport|nginx-ingress-v1|contour-http-proxy|ingress] 
Default: route if the environment is OpenShift, otherwise loadbalancer`)
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessHost, "link-access-host", "", "The host or IP address at which to expose link access")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.serviceAccount, "service-account", "", "Specify the service account")
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	stringValidator := validator.NewStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(linkAccessTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is already a site created for this namespace"))
	}

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else {
		cmd.siteName = args[0]

		ok, err := stringValidator.Evaluate(cmd.siteName)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
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
		ok, err := stringValidator.Evaluate(cmd.flags.serviceAccount)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("service account name is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdSiteCreate) InputToOptions() {

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
	options[site.SiteConfigNameKey] = cmd.siteName

	if cmd.flags.linkAccessHost != "" {
		options[site.SiteConfigIngressHostKey] = cmd.flags.linkAccessHost
	}

	cmd.options = options

}

func (cmd *CmdSiteCreate) Run() error {

	resource := v1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{Name: cmd.siteName},
		Spec: v1alpha1.SiteSpec{
			Settings:       cmd.options,
			ServiceAccount: cmd.serviceAccountName,
			LinkAccess:     cmd.linkAccessType,
		},
	}

	_, err := cmd.Client.Sites(cmd.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err
}

func (cmd *CmdSiteCreate) WaitUntilReady() error {

	err := utils.NewSpinner("Waiting for site...", 5, func() error {

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

	fmt.Printf("Site %q is ready\n", cmd.siteName)

	return nil
}
