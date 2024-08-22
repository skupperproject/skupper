package kube

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

type CmdTokenRedeem struct {
	client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandTokenRedeemFlags
	name      string
	namespace string
	fileName  string
}

func NewCmdTokenRedeem() *CmdTokenRedeem {
	return &CmdTokenRedeem{}
}

func (cmd *CmdTokenRedeem) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdTokenRedeem) ValidateInput(args []string) error {
	var validationErrors []error
	tokenStringValidator := validator.NewFilePathStringValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()

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
		ok, _ := utils.SiteReady(siteList)
		if !ok {
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

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdTokenRedeem) Run() error {

	// get data from token file
	var accessToken v2alpha1.AccessToken
	var tokenFile []byte
	tokenFile, err := os.ReadFile(cmd.fileName)
	if err != nil {
		err = fmt.Errorf("unable to read token file - %v", err)
		return err
	}
	yamlS := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	yamlS.Decode(tokenFile, nil, &accessToken)

	accessToken.Namespace = cmd.namespace
	cmd.name = accessToken.Name

	_, err = cmd.client.AccessTokens(cmd.namespace).Create(context.TODO(), &accessToken, metav1.CreateOptions{})
	return err
}

func (cmd *CmdTokenRedeem) WaitUntil() error {
	waitTime := int(cmd.Flags.Timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for token status ...", waitTime, func() error {

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
		return fmt.Errorf("Token %q redeem not ready yet, check the status for more information\n", cmd.name)
	}

	fmt.Printf("Token %q has been redeemed\n", cmd.name)
	fmt.Printf("You can now safely delete %s\n", cmd.fileName)
	return nil
}

func (cmd *CmdTokenRedeem) InputToOptions() {}
