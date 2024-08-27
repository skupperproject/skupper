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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

var (
	tokenIssueLong    = "Issue a token file redeemable for a link to the current site."
	tokenIssueExample = "skupper token issue tokenName ~/token1.yaml"
)

type TokenIssue struct {
	timeout     time.Duration
	expiration  time.Duration
	redemptions int
}

type CmdTokenIssue struct {
	client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  cobra.Command
	flags     TokenIssue
	namespace string
	grantName string
	fileName  string
}

func NewCmdTokenIssue() *CmdTokenIssue {

	skupperCmd := CmdTokenIssue{}

	cmd := cobra.Command{
		Use:     "issue <name> <fileName>",
		Short:   "issue a token",
		Long:    tokenIssueLong,
		Example: tokenIssueExample,
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

func (cmd *CmdTokenIssue) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdTokenIssue) AddFlags() {
	cmd.CobraCmd.Flags().IntVarP(&cmd.flags.redemptions, "redemptions-allowed", "r", 1, "The number of times an access token for this grant can be redeemed.")
	cmd.CobraCmd.Flags().DurationVarP(&cmd.flags.expiration, "expiration-window", "e", 15*time.Minute, "The period of time in which an access token for this grant can be redeeme.")
	cmd.CobraCmd.Flags().DurationVarP(&cmd.flags.timeout, "timeout", "t", 60*time.Second, "Raise an error if the operation does not complete in the given period of time.")
}

func (cmd *CmdTokenIssue) ValidateInput(args []string) []error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	tokenStringValidator := validator.NewFilePathStringValidator()

	// Validate grant name and token file name
	if len(args) < 2 {
		validationErrors = append(validationErrors, fmt.Errorf("token name and file name must be configured"))
	} else if len(args) > 2 {
		validationErrors = append(validationErrors, fmt.Errorf("only two arguments are allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("token name must not be empty"))
	} else if args[1] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("file name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("token name is not valid: %s", err))
		} else {
			cmd.grantName = args[0]
		}

		ok, err = tokenStringValidator.Evaluate(args[1])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("token file name is not valid: %s", err))
		} else {
			cmd.fileName = args[1]
		}
	}

	// Validate there is already a site defined in the namespace before a token can be created
	siteList, _ := cmd.client.Sites(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if siteList == nil || len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("A site must exist in namespace %s before a token can be created", cmd.namespace))
	} else {
		if !utils.SiteConfigured(siteList) {
			validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
		}
	}

	// Validate if we already have a token with this name in the namespace
	if cmd.grantName != "" {
		grant, err := cmd.client.AccessGrants(cmd.namespace).Get(context.TODO(), cmd.grantName, metav1.GetOptions{})
		if grant != nil && !errors.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("there is already a token %s created in namespace %s", cmd.grantName, cmd.namespace))
		}
	}

	// Validate flags
	//TBD is there a limit to number of redemptions
	if cmd.flags.redemptions < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("number of redemptions is not valid"))
	}

	//TBD what is valid times
	if cmd.flags.expiration <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("expiration time is not valid"))
	}

	//TBD what is valid timeout
	if cmd.flags.timeout <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid"))
	}

	return validationErrors
}

func (cmd *CmdTokenIssue) Run() error {
	resource := v1alpha1.AccessGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "AccessGrant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cmd.grantName,
		},
		Spec: v1alpha1.AccessGrantSpec{
			RedemptionsAllowed: cmd.flags.redemptions,
			ExpirationWindow:   cmd.flags.expiration.String(),
		},
	}

	_, err := cmd.client.AccessGrants(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err

}

func (cmd *CmdTokenIssue) WaitUntil() error {
	err := utils.NewSpinnerWithTimeout("Waiting for token status ...", int(cmd.flags.timeout.Seconds()), func() error {

		token, err := cmd.client.AccessGrants(cmd.namespace).Get(context.TODO(), cmd.grantName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if token != nil && token.IsReady() {
			// write token to file
			s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
			out, err := os.Create(cmd.fileName)
			if err != nil {
				return fmt.Errorf("Could not write to file " + cmd.fileName + ": " + err.Error())
			}
			err = s.Encode(token, out)
			if err != nil {
				return fmt.Errorf("Could not write out generated token: " + err.Error())
			} else {
				return nil
			}
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("Grant %q not ready yet, check the status for more information\n", cmd.grantName)
	}

	fmt.Printf("\nGrant %q is ready\n", cmd.grantName)
	fmt.Printf("Token file %s created\n", cmd.fileName)
	fmt.Printf("\nTransfer this file to a remote site. At the remote site,\n")
	fmt.Printf("create a link to this site using the \"skupper token redeem\" command:\n")
	fmt.Printf("\n\tskupper token redeem <file>\n")
	fmt.Printf("\nThe token expires after %d use(s) or after %s.\n", cmd.flags.redemptions, cmd.flags.expiration.String())
	return nil
}

func (cmd *CmdTokenIssue) InputToOptions() {}
