package kube

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"

	"github.com/google/uuid"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	k8serrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdTokenIssue struct {
	client    skupperv2alpha1.SkupperV2alpha1Interface
	CobraCmd  *cobra.Command
	Flags     *common.CommandTokenIssueFlags
	namespace string
	grantName string
	fileName  string
	cost      int
}

func NewCmdTokenIssue() *CmdTokenIssue {

	return &CmdTokenIssue{}

}

func (cmd *CmdTokenIssue) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	utils.HandleError(utils.GenericError, err)

	cmd.client = cli.GetSkupperClient().SkupperV2alpha1()
	cmd.namespace = cli.Namespace
}

func (cmd *CmdTokenIssue) ValidateInput(args []string) error {
	var validationErrors []error
	tokenStringValidator := validator.NewFilePathStringValidator()
	timeoutValidator := validator.NewTimeoutInSecondsValidator()
	expirationValidator := validator.NewExpirationInSecondsValidator()
	numberValidator := validator.NewNumberValidator()

	// Validate if AccessGrant CRD is installed
	_, err := cmd.client.AccessGrants(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}

	// Validate token file name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("file name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("file name must not be empty"))
	} else {
		ok, err := tokenStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("token file name is not valid: %s", err))
		} else {
			// check filename and directory are valid to create the token
			if fileInfo, err := os.Stat(args[0]); err == nil && fileInfo.IsDir() {
				validationErrors = append(validationErrors, fmt.Errorf("token file name is a directory"))
			} else {
				directory, filename := filepath.Split(args[0])
				// test token can be create in directory
				file, err := os.CreateTemp(directory, filename)
				if err != nil {
					validationErrors = append(validationErrors, fmt.Errorf("invalid token file name: %s", errors.Unwrap(err)))
				} else {
					os.Remove(file.Name())
				}
			}
			cmd.fileName = args[0]
		}
	}

	// Validate Site CRD is installed and there is already a site defined in the namespace before a token can be created
	siteList, err := cmd.client.Sites(cmd.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		validationErrors = append(validationErrors, utils.HandleMissingCrds(err))
		return errors.Join(validationErrors...)
	}
	if siteList == nil || len(siteList.Items) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("A site must exist in namespace %s before a token can be created", cmd.namespace))
	} else {
		ok, siteName := utils.SiteReady(siteList)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("there is no active skupper site in this namespace"))
		} else {
			ok, siteName = utils.SiteLinkAccessEndpoints(siteList)
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("You must enable link access for this site before you can create a token."))
			} else {
				cmd.grantName = siteName + "-" + uuid.New().String()
			}
		}
	}

	// Validate if we already have a token with this name in the namespace
	if cmd.grantName != "" {
		grant, err := cmd.client.AccessGrants(cmd.namespace).Get(context.TODO(), cmd.grantName, metav1.GetOptions{})
		if grant != nil && !k8serrs.IsNotFound(err) {
			validationErrors = append(validationErrors, fmt.Errorf("There is already a token %s created in namespace %s", cmd.grantName, cmd.namespace))
		}
	}

	// Validate flags
	if cmd.Flags != nil && cmd.Flags.RedemptionsAllowed < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("number of redemptions is not valid"))
	}

	if cmd.Flags != nil && cmd.Flags.ExpirationWindow.String() != "" {
		ok, err := expirationValidator.Evaluate(cmd.Flags.ExpirationWindow)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("expiration time is not valid: %s", err))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Timeout.String() != "" {
		ok, err := timeoutValidator.Evaluate(cmd.Flags.Timeout)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("timeout is not valid: %s", err))
		}
	}

	selectedCost, err := strconv.Atoi(cmd.Flags.Cost)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	}
	ok, err := numberValidator.Evaluate(selectedCost)
	if !ok {
		validationErrors = append(validationErrors, fmt.Errorf("link cost is not valid: %s", err))
	} else {
		cmd.cost = selectedCost
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdTokenIssue) Run() error {
	resource := v2alpha1.AccessGrant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "AccessGrant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cmd.grantName,
		},
		Spec: v2alpha1.AccessGrantSpec{
			RedemptionsAllowed: cmd.Flags.RedemptionsAllowed,
			ExpirationWindow:   cmd.Flags.ExpirationWindow.String(),
		},
	}

	_, err := cmd.client.AccessGrants(cmd.namespace).Create(context.TODO(), &resource, metav1.CreateOptions{})
	return err

}

func (cmd *CmdTokenIssue) WaitUntil() error {
	waitTime := int(cmd.Flags.Timeout.Seconds())
	err := utils.NewSpinnerWithTimeout("Waiting for token status ...", waitTime, func() error {

		accessGrant, err := cmd.client.AccessGrants(cmd.namespace).Get(context.TODO(), cmd.grantName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if accessGrant != nil && accessGrant.IsReady() {

			accessToken := v2alpha1.AccessToken{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "AccessToken",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: accessGrant.Name,
				},
				Spec: v2alpha1.AccessTokenSpec{
					Url:      accessGrant.Status.Url,
					Code:     accessGrant.Status.Code,
					Ca:       accessGrant.Status.Ca,
					LinkCost: cmd.cost,
				},
			}

			encodedResource, err := utils.Encode("yaml", accessToken)
			if err != nil {
				return fmt.Errorf("could not write out generated token: %s", err.Error())
			}

			err = os.WriteFile(cmd.fileName, []byte(encodedResource), 0644)
			if err != nil {
				return fmt.Errorf("could not write to file %s:%s", cmd.fileName, err.Error())
			}

			return nil
		}

		return fmt.Errorf("error getting the resource")
	})

	if err != nil {
		return fmt.Errorf("grant %q not ready yet, check the status for more information", cmd.grantName)
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
