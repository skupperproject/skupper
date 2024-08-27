/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	siteCreateLong = `A site is a place where components of your application are running. 
Sites are linked to form application networks.
There can be only one site definition per namespace.`
)

type CreateFlags struct {
	enableLinkAccess bool
	linkAccessType   string
	serviceAccount   string
	output           string
}

type CmdSiteCreate struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           cobra.Command
	flags              CreateFlags
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	skupperCmd := CmdSiteCreate{flags: CreateFlags{}}

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
			return skupperCmd.WaitUntil()
		},
	}

	skupperCmd.CobraCmd = cmd
	skupperCmd.AddFlags()

	return &skupperCmd
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteCreate) AddFlags() {
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableLinkAccess, "enable-link-access", false, "allow access for incoming links from remote sites (default: false)")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessType, "link-access-type", "", `configure external access for links from remote sites.
Choices: [route|loadbalancer]. Default: On OpenShift, route is the default; 
for other Kubernetes flavors, loadbalancer is the default.`)
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.serviceAccount, "service-account", "", "the Kubernetes service account under which to run the Skupper controller")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(utils.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is already a site created for this namespace"))
	}

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command."))
	} else {
		cmd.siteName = args[0]

		ok, err := resourceStringValidator.Evaluate(cmd.siteName)
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
		svcAccount, err := cmd.KubeClient.CoreV1().ServiceAccounts(cmd.Namespace).Get(context.TODO(), cmd.flags.serviceAccount, metav1.GetOptions{})
		if err != nil || svcAccount == nil {
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

func (cmd *CmdSiteCreate) InputToOptions() {

	cmd.serviceAccountName = cmd.flags.serviceAccount

	if cmd.flags.enableLinkAccess {
		if cmd.flags.linkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.flags.linkAccessType
		}
	}

	cmd.output = cmd.flags.output

}

func (cmd *CmdSiteCreate) Run() error {

	resource := v1alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.siteName,
			Namespace: cmd.Namespace,
		},
		Spec: v1alpha1.SiteSpec{
			ServiceAccount: cmd.serviceAccountName,
			LinkAccess:     cmd.linkAccessType,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		_, err := cmd.Client.Sites(cmd.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
		return err
	}

}

func (cmd *CmdSiteCreate) WaitUntil() error {

	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for status...", 5, func() error {

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
		return fmt.Errorf("Site %q not configured yet, check the status for more information\n", cmd.siteName)
	}

	fmt.Printf("Site %q is configured. Check the status to see when it is ready\n", cmd.siteName)

	return nil
}
