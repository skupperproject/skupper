/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

type CmdSiteCreate struct {
	Client             skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient         kubernetes.Interface
	CobraCmd           *cobra.Command
	Flags              *common.CommandSiteCreateFlags
	siteName           string
	serviceAccountName string
	Namespace          string
	linkAccessType     string
	output             string
	timeout            time.Duration
}

func NewCmdSiteCreate() *CmdSiteCreate {

	skupperCmd := CmdSiteCreate{}

	return &skupperCmd
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(common.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is already a site created for this namespace"))
	}

	if cmd.Flags != nil && cmd.Flags.BindHost != "" {
		fmt.Println("Warning: --bind-host flag is not supported on this platform")
	}

	if cmd.Flags != nil && cmd.Flags.SubjectAlternativeNames != nil {
		fmt.Println("Warning: --subject-alternative-names flag is not supported on this platform")
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

	if cmd.Flags != nil && cmd.Flags.LinkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.Flags.LinkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && !cmd.Flags.EnableLinkAccess && len(cmd.Flags.LinkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.Flags != nil && cmd.Flags.ServiceAccount != "" {
		svcAccount, err := cmd.KubeClient.CoreV1().ServiceAccounts(cmd.Namespace).Get(context.TODO(), cmd.Flags.ServiceAccount, metav1.GetOptions{})
		if err != nil || svcAccount == nil {
			validationErrors = append(validationErrors, fmt.Errorf("service account name is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
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

func (cmd *CmdSiteCreate) InputToOptions() {

	cmd.serviceAccountName = cmd.Flags.ServiceAccount

	if cmd.Flags.EnableLinkAccess {
		if cmd.Flags.LinkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.Flags.LinkAccessType
		}
	}

	cmd.output = cmd.Flags.Output
	cmd.timeout = cmd.Flags.Timeout

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

	waitTime := int(cmd.timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for status...", waitTime, func() error {

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
