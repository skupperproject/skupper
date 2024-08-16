package kube

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	utils2 "github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"os"
	"time"

	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdTokenIssue struct {
	client    skupperv1alpha1.SkupperV1alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandTokenIssueFlags
	namespace string
	grantName string
	fileName  string
}

func NewCmdTokenIssue() *CmdTokenIssue {

	return &CmdTokenIssue{}

}

func (cmd *CmdTokenIssue) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils2.HandleError(err)

	cmd.client = cli.GetSkupperClient().SkupperV1alpha1()
	cmd.namespace = cli.Namespace
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
		if !utils2.SiteReady(siteList) {
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
	if cmd.Flags.RedemptionsAllowed < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("number of redemptions is not valid"))
	}

	//TBD what is valid times
	if cmd.Flags.ExpirationWindow <= 0*time.Minute {
		validationErrors = append(validationErrors, fmt.Errorf("expiration time is not valid"))
	}

	//TBD what is valid timeout --> use timeoutValidator
	if cmd.Flags.Timeout <= 0*time.Minute {
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
			RedemptionsAllowed: cmd.Flags.RedemptionsAllowed,
			ExpirationWindow:   cmd.Flags.ExpirationWindow.String(),
		},
	}

	_, err := cmd.client.AccessGrants(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err

}

func (cmd *CmdTokenIssue) WaitUntil() error {
	err := utils2.NewSpinnerWithTimeout("Waiting for token status ...", int(cmd.Flags.Timeout.Seconds()), func() error {

		accessGrant, err := cmd.client.AccessGrants(cmd.namespace).Get(context.TODO(), cmd.grantName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if accessGrant != nil && accessGrant.IsReady() {

			accessToken := v1alpha1.AccessToken{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v1alpha1",
					Kind:       "AccessToken",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: accessGrant.Name,
				},
				Spec: v1alpha1.AccessTokenSpec{
					Url:  accessGrant.Status.Url,
					Code: accessGrant.Status.Code,
					Ca:   accessGrant.Status.Ca,
				},
			}

			encodedResource, err := utils.Encode("yaml", accessToken)
			if err != nil {
				return fmt.Errorf("Could not write out generated token: " + err.Error())
			}

			err = os.WriteFile(cmd.fileName, []byte(encodedResource), 0644)
			if err != nil {
				return fmt.Errorf("Could not write to file " + cmd.fileName + ": " + err.Error())
			}

			return nil
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
	fmt.Printf("\nThe token expires after %d use(s) or after %s.\n", cmd.Flags.RedemptionsAllowed, cmd.Flags.ExpirationWindow.String())
	return nil
}

func (cmd *CmdTokenIssue) InputToOptions() {}
