/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"strconv"
)

var (
	linkCreateLong = `Create a new link`
)

type CreateLinkFlags struct {
	tlsSecret string
	cost      string
	output    string
}

type CmdLinkCreate struct {
	Client     skupperv1alpha1.SkupperV1alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   cobra.Command
	flags      CreateLinkFlags
	linkName   string
	Namespace  string
	tlsSecret  string
	cost       int
	output     string
}

func NewCmdLinkCreate() *CmdLinkCreate {

	skupperCmd := CmdLinkCreate{flags: CreateLinkFlags{}}

	cmd := cobra.Command{
		Use:    "create <name>",
		Short:  "Create a new link",
		Long:   linkCreateLong,
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

func (cmd *CmdLinkCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.Client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.KubeClient = cli.GetKubeClient()
	cmd.Namespace = cli.Namespace
}

func (cmd *CmdLinkCreate) AddFlags() {
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.tlsSecret, "tls-secret", "", "the name of a Kubernetes secret containing TLS credentials.")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.cost, "cost", "1", "the configured \"expense\" of sending traffic over the link. ")
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdLinkCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	numberValidator := validator.NewNumberValidator()
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

	//Validate if there is already a site defined in the namespace
	siteList, _ := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList != nil && len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("there is no skupper site in this namespace"))
	}

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("link name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command."))
	} else {
		cmd.linkName = args[0]

		ok, err := resourceStringValidator.Evaluate(cmd.linkName)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link name is not valid: %s", err))
		}
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

	return validationErrors
}

func (cmd *CmdLinkCreate) InputToOptions() {

	cmd.cost, _ = strconv.Atoi(cmd.flags.cost)
	cmd.tlsSecret = cmd.flags.tlsSecret
	cmd.output = cmd.flags.output

}

func (cmd *CmdLinkCreate) Run() error {

	resource := v1alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.linkName,
			Namespace: cmd.Namespace,
		},
		Spec: v1alpha1.LinkSpec{
			TlsCredentials: cmd.tlsSecret,
			Cost:           cmd.cost,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		_, err := cmd.Client.Links(cmd.Namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
		return err
	}

}

func (cmd *CmdLinkCreate) WaitUntilReady() error {

	// the site resource was not created
	if cmd.output != "" {
		return nil
	}

	err := utils.NewSpinner("Waiting for status...", 5, func() error {

		resource, err := cmd.Client.Links(cmd.Namespace).Get(context.TODO(), cmd.linkName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if resource != nil && resource.Status.StatusMessage == "OK" {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Link %q not ready yet, check the logs for more information\n", cmd.linkName)
	}

	fmt.Printf("Link %q is ready\n", cmd.linkName)

	return nil
}
