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
	"strconv"
)

var (
	linkUpdateLong = "Change link settings"
)

type UpdateLinkFlags struct {
	tlsSecret string
	cost      string
	output    string
	timeout   string
}

type CmdLinkUpdate struct {
	Client     skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   cobra.Command
	flags      UpdateLinkFlags
	linkName   string
	Namespace  string
	tlsSecret  string
	cost       int
	output     string
	timeout    int
}

func NewCmdLinkUpdate() *CmdLinkUpdate {

	skupperCmd := CmdLinkUpdate{flags: UpdateLinkFlags{}}

	cmd := cobra.Command{
		Use:    "update <name>",
		Short:  "Change link settings",
		Long:   linkUpdateLong,
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

func (cmd *CmdLinkUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkUpdate) AddFlags() {
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.tlsSecret, "tls-secret", "t", "", "the name of a Kubernetes secret containing TLS credentials.")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.cost, "cost", "1", "the configured \"expense\" of sending traffic over the link. ")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.timeout, "timeout", "60", "raise an error if the operation does not complete in the given period of time (expressed in seconds).")
}

func (cmd *CmdLinkUpdate) ValidateInput(args []string) []error {

	var validationErrors []error
	numberValidator := validator.NewNumberValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

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

	if cmd.flags.tlsSecret != "" {
		secret, err := cmd.KubeClient.CoreV1().Secrets(cmd.Namespace).Get(context.TODO(), cmd.flags.tlsSecret, metav1.GetOptions{})
		if secret == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("the TLS secret %q is not available in the namespace: %s", cmd.flags.tlsSecret, err))
		}
	}

	selectedCost, err := strconv.Atoi(cmd.flags.cost)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}
	ok, err := numberValidator.Evaluate(selectedCost)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	selectedTimeout, convErr := strconv.Atoi(cmd.flags.timeout)
	if convErr != nil {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", convErr))
	} else {
		ok, err = timeoutValidator.Evaluate(selectedTimeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdLinkUpdate) InputToOptions() {

	cmd.cost, _ = strconv.Atoi(cmd.flags.cost)
	cmd.tlsSecret = cmd.flags.tlsSecret
	cmd.output = cmd.flags.output
	cmd.timeout, _ = strconv.Atoi(cmd.flags.timeout)

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

	updatedTlsSecret := currentSite.Spec.TlsCredentials
	if cmd.tlsSecret != "" {
		updatedTlsSecret = cmd.tlsSecret
	}

	resource := v1alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:              cmd.linkName,
			Namespace:         cmd.Namespace,
			CreationTimestamp: currentSite.CreationTimestamp,
			ResourceVersion:   currentSite.ResourceVersion,
		},
		Spec: v1alpha1.LinkSpec{
			TlsCredentials: updatedTlsSecret,
			Cost:           updatedCost,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		_, err := cmd.Client.Links(cmd.Namespace).Update(context.TODO(), &resource, metav1.UpdateOptions{})
		return err
	}

}

func (cmd *CmdLinkUpdate) WaitUntil() error {

	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinnerWithTimeout("Waiting for update to complete...", cmd.timeout, func() error {

		resource, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.IsConfigured() {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Link %q not updated yet, check the status for more information\n", cmd.linkName)
	}

	fmt.Printf("Link %q is updated\n", cmd.linkName)

	return nil
}
