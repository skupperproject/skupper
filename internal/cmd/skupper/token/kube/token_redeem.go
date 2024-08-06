package kube

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var (
	tokenRedeemLong    = "Redeem a token file in order to create a link to a remote site."
	tokenRedeemExample = "skupper token redeem ~/token1.yaml"
)

type TokenRedeem struct {
	timeout time.Duration
}

type CmdTokenRedeem struct {
	client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	flags     TokenRedeem
	name      string
	namespace string
	fileName  string
}

func NewCmdTokenRedeem() *CmdTokenRedeem {

	skupperCmd := CmdTokenRedeem{}

	cmd := cobra.Command{
		Use:     "redeem <filename>",
		Short:   "redeem a token",
		Long:    tokenRedeemLong,
		Example: tokenRedeemExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
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

func (cmd *CmdTokenRedeem) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdTokenRedeem) AddFlags() {
	cmd.CobraCmd.Flags().DurationVarP(&cmd.flags.timeout, "timeout", "t", 60*time.Second, "Raise an error if the operation does not complete in the given period of time.")
}

func (cmd *CmdTokenRedeem) ValidateInput(args []string) []error {
	var validationErrors []error
	tokenStringValidator := validator.NewFilePathStringValidator()

	// Validate token file name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("token file name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("file name must not be empty"))
	} else {
		ok, err := tokenStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("token file name is not valid: %s", err))
		} else {
			cmd.fileName = args[0]
		}
	}

	// Validate there is already a site defined in the namespace before a token can be redeemed
	siteList, _ := cmd.client.Sites(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList == nil || len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("A site must exist in namespace %s before a token can be redeemed", cmd.namespace))
	} else {
		if !utils.SiteConfigured(siteList) {
			validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
		}
	}

	// Validate if token file exists
	if cmd.fileName != "" {
		_, err := os.Stat(cmd.fileName)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("token file does not exist: %s", err))
		}
	}

	//TBD what is valid times
	if cmd.flags.timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}

	return validationErrors
}

func (cmd *CmdTokenRedeem) Run() error {

	// get data from token file
	var grant v1alpha1.AccessGrant
	var tokenFile []byte
	tokenFile, err := os.ReadFile(cmd.fileName)
	if err != nil {
		err = fmt.Errorf("unable to read token file - %v", err)
		return err
	}
	yamlS := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	yamlS.Decode(tokenFile, nil, &grant)
	cmd.name = grant.Name

	// create AccessToken
	resource := v1alpha1.AccessToken{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "AccessToken",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.name,
			Namespace: cmd.namespace,
		},
		Spec: v1alpha1.AccessTokenSpec{
			Url:  grant.Status.Url,
			Ca:   grant.Status.Ca,
			Code: grant.Status.Code,
		},
	}

	_, err = cmd.client.AccessTokens(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err
}

func (cmd *CmdTokenRedeem) WaitUntil() error {

	err := utils.NewSpinnerWithTimeout("Waiting for token status ...", int(cmd.flags.timeout.Seconds()), func() error {

		token, err := cmd.client.AccessTokens(cmd.namespace).Get(context.TODO(), cmd.name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if token != nil && token.IsRedeemed() {
			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Token %q redeem not ready yet, check the logs for more information\n", cmd.name)
	}

	fmt.Printf("Token %q has been redeemed\n", cmd.name)
	fmt.Printf("You can now safely delete %s\n", cmd.fileName)
	return nil
}

func (cmd *CmdTokenRedeem) InputToOptions() {}
