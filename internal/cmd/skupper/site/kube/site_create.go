/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdSiteCreate struct {
	Client         skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient     kubernetes.Interface
	CobraCmd       *cobra.Command
	Flags          *common.CommandSiteCreateFlags
	siteName       string
	Namespace      string
	linkAccessType string
	HA             bool
	timeout        time.Duration
	status         string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	skupperCmd := CmdSiteCreate{}

	return &skupperCmd
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(common.LinkAccessTypes)
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	statusValidator := validator.NewOptionValidator(common.WaitStatusTypes)

	//Validate if CRDs are installed and if there is already a site defined in the namespace
	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}
	if siteList != nil && len(siteList.Items) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("There is already a site created for this namespace"))
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

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Wait != "" {
		ok, err := statusValidator.Evaluate(cmd.Flags.Wait)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("status is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteCreate) InputToOptions() {

	if cmd.Flags.EnableLinkAccess {
		if cmd.Flags.LinkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.Flags.LinkAccessType
		}
	}

	cmd.timeout = cmd.Flags.Timeout
	cmd.status = cmd.Flags.Wait
	cmd.HA = cmd.Flags.EnableHA

}

func (cmd *CmdSiteCreate) Run() error {

	resource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.siteName,
			Namespace: cmd.Namespace,
		},
		Spec: v2alpha1.SiteSpec{
			LinkAccess: cmd.linkAccessType,
			HA:         cmd.HA,
		},
	}

	_, err := cmd.Client.Sites(cmd.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err

}

func (cmd *CmdSiteCreate) WaitUntil() error {

	if cmd.status == "none" {
		return nil
	}

	waitTime := int(cmd.timeout.Seconds())

	var siteCondition *metav1.Condition

	err := utils.NewSpinnerWithTimeout("Waiting for status...", waitTime, func() error {

		resource, err := cmd.Client.Sites(cmd.Namespace).Get(context.TODO(), cmd.siteName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isConditionFound := false
		isConditionTrue := false

		switch cmd.status {
		case "configured":
			siteCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_CONFIGURED)
		default:
			siteCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_READY)
		}

		if siteCondition != nil {
			isConditionFound = true
			isConditionTrue = siteCondition.Status == metav1.ConditionTrue
		}

		if resource != nil && isConditionFound && isConditionTrue {
			return nil
		}

		if resource != nil && isConditionFound && !isConditionTrue {
			return fmt.Errorf("error in the condition")
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil && siteCondition == nil {
		return fmt.Errorf("Site %q is not yet %s, check the status for more information\n", cmd.siteName, cmd.status)
	} else if err != nil && siteCondition.Status == metav1.ConditionFalse {
		return fmt.Errorf("Site %q is not yet %s: %s\n", cmd.siteName, cmd.status, siteCondition.Message)
	}

	fmt.Printf("Site %q is %s.\n", cmd.siteName, cmd.status)

	return nil
}
