/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package site

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strconv"
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
	Client   *client.VanClient
	CobraCmd cobra.Command
	flags    CreateFlags
	options  map[string]string
	siteName string
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
			utils.HandleError(skupperCmd.InputToOptions(args))
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

func (cmd *CmdSiteCreate) AddFlags() {
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableLinkAccess, "enable-link-access", false, "Enable external access for links from remote sites")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessType, "link-access-type", "", `Select the means of opening external access 
One of: [route|loadbalancer|nodeport|nginx-ingress-v1|contour-http-proxy|ingress] 
Default: route if the environment is OpenShift, otherwise loadbalancer`)
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessHost, "link-access-host", "", "The host or IP address at which to expose link access")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.serviceAccount, "service-account", "", "Specify the service account")
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient("", "", "")
	utils.HandleError(err)

	cmd.Client = cli
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	stringValidator := validator.NewStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(linkAccessTypes)

	//Validate if there is already a site defined in the namespace
	siteList, err := cmd.Client.GetSkupperClient().SkupperV1alpha1().Sites(cmd.Client.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil && siteList != nil && len(siteList.Items) > 0 {
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

	if cmd.flags.enableLinkAccess {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.flags.linkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if !cmd.flags.enableLinkAccess && len(cmd.flags.linkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	return validationErrors
}

func (cmd *CmdSiteCreate) InputToOptions(args []string) error {

	options := make(map[string]string)

	options["name"] = args[0]
	options["linkAccess"] = strconv.FormatBool(cmd.flags.enableLinkAccess)

	//TODO: set value route or loadbalancer as defaults depending if it's openshift or kubernetes
	if cmd.flags.enableLinkAccess && cmd.flags.linkAccessType == "" {
		options["linkAccessType"] = "loadbalancer"
	}
	if cmd.flags.linkAccessType != "" {
		options["linkAccessType"] = cmd.flags.linkAccessType
	}

	if cmd.flags.linkAccessHost != "" {
		options["linkAccessHost"] = cmd.flags.linkAccessHost
	}

	if len(cmd.flags.serviceAccount) > 0 {
		options["serviceAccount"] = cmd.flags.serviceAccount
	}

	cmd.options = options

	return nil
}

func (cmd *CmdSiteCreate) Run() error {

	siteName := cmd.options["name"]

	resource := v1alpha1.Site{
		ObjectMeta: metav1.ObjectMeta{Name: siteName},
		Spec: v1alpha1.SiteSpec{
			Settings: cmd.options,
		},
	}

	_, err := cmd.Client.GetSkupperClient().SkupperV1alpha1().Sites(cmd.Client.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	utils.HandleError(err)
	return nil
}

func (cmd *CmdSiteCreate) WaitUntilReady() error {

	err := utils.NewSpinner("Waiting for site...", 5, func() error {

		resource, err := cmd.Client.GetSkupperClient().SkupperV1alpha1().Sites(cmd.Client.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Site \"%s\" not ready yet, check the logs for more information\n", cmd.siteName)
	}

	err = utils.NewSpinner("Waiting for status...", 5, func() error {

		configmap, err := cmd.Client.KubeClient.CoreV1().ConfigMaps(cmd.Client.Namespace).Get(context.TODO(), types.NetworkStatusConfigMapName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if configmap != nil && configmap.Data != nil && len(configmap.Data) > 0 {
			return nil
		}

		return fmt.Errorf("error getting the status updated")
	})

	if err != nil {
		fmt.Println("Status is not ready yet, check the logs for more information")
	}

	fmt.Printf("Site \"%s\" is ready\n", cmd.siteName)

	return nil
}
