/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type CmdLinkUpdate struct {
	Client         skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient     kubernetes.Interface
	CobraCmd       *cobra.Command
	Flags          *common.CommandLinkUpdateFlags
	linkName       string
	Namespace      string
	tlsCredentials string
	cost           int
	timeout        time.Duration
	status         string
}

func NewCmdLinkUpdate() *CmdLinkUpdate {
	return &CmdLinkUpdate{}
}

func (cmd *CmdLinkUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkUpdate) ValidateInput(args []string) error {

	var validationErrors []error
	numberValidator := validator.NewNumberValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	statusValidator := validator.NewOptionValidator(common.WaitStatusTypes)

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
		link, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), args[0], metav1.GetOptions{})
		if link == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("the link %q is not available in the namespace: %s", args[0], err))
		}
		cmd.linkName = args[0]
	}

	if cmd.Flags.TlsCredentials != "" {
		secret, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).Get(context.TODO(), cmd.Flags.TlsCredentials, metav1.GetOptions{})
		if secret == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("the TLS secret %q is not available in the namespace: %s", cmd.Flags.TlsCredentials, err))
		}
	}

	selectedCost, err := strconv.Atoi(cmd.Flags.Cost)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}
	ok, err := numberValidator.Evaluate(selectedCost)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}

	ok, err = timeoutValidator.Evaluate(cmd.Flags.Timeout)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
	}

	if cmd.Flags != nil && cmd.Flags.Wait != "" {
		ok, err := statusValidator.Evaluate(cmd.Flags.Wait)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("status is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkUpdate) InputToOptions() {

	cmd.cost, _ = strconv.Atoi(cmd.Flags.Cost)
	cmd.tlsCredentials = cmd.Flags.TlsCredentials
	cmd.timeout = cmd.Flags.Timeout
	cmd.status = cmd.Flags.Wait

}

func (cmd *CmdLinkUpdate) Run() error {

	currentSite, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	updatedCost := currentSite.Spec.Cost
	if cmd.cost != currentSite.Spec.Cost {
		updatedCost = cmd.cost
	}

	updatedTlsCredentials := currentSite.Spec.TlsCredentials
	if cmd.tlsCredentials != "" {
		updatedTlsCredentials = cmd.tlsCredentials
	}

	resource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              cmd.linkName,
			Namespace:         cmd.Namespace,
			CreationTimestamp: currentSite.CreationTimestamp,
			ResourceVersion:   currentSite.ResourceVersion,
		},
		Spec: v2alpha1.LinkSpec{
			TlsCredentials: updatedTlsCredentials,
			Cost:           updatedCost,
		},
	}

	_, err = cmd.Client.Links(cmd.Namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
	return err

}

func (cmd *CmdLinkUpdate) WaitUntil() error {

	if cmd.status == "none" {
		return nil
	}

	waitTime := int(cmd.timeout.Seconds())
	var linkCondition *metav1.Condition
	err := utils.NewSpinnerWithTimeout("Waiting for update to complete...", waitTime, func() error {

		resource, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		isConditionFound := false
		isConditionTrue := false

		switch cmd.status {
		case "configured":
			linkCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_CONFIGURED)
		default:
			linkCondition = meta.FindStatusCondition(resource.Status.Conditions, v2alpha1.CONDITION_TYPE_READY)
		}

		if linkCondition != nil {
			isConditionFound = true
			isConditionTrue = linkCondition.Status == metav1.ConditionTrue
		}

		if resource != nil && isConditionFound && isConditionTrue {
			return nil
		}

		if resource != nil && isConditionFound && !isConditionTrue {
			return fmt.Errorf("error in the condition")
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil && linkCondition == nil {
		return fmt.Errorf("Link %q is not yet %s, check the status for more information\n", cmd.linkName, cmd.status)
	} else if err != nil && linkCondition.Status == metav1.ConditionFalse {
		return fmt.Errorf("Link %q is not yet %s: %s\n", cmd.linkName, cmd.status, linkCondition.Message)
	}

	fmt.Printf("Link %q is updated\n", cmd.linkName)

	return nil
}
