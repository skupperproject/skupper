package nonkube

import (
	"errors"
	"fmt"
	"os"

	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	"github.com/spf13/cobra"
)

type CmdTokenRedeem struct {
	CobraCmd      *cobra.Command
	Flags         *common.CommandTokenRedeemFlags
	Namespace     string
	siteName      string
	linkHandler   *fs.LinkHandler
	secretHandler *fs.SecretHandler
	fileName      string
	name          string
}

func NewCmdTokenRedeem() *CmdTokenRedeem {
	return &CmdTokenRedeem{}
}

func (cmd *CmdTokenRedeem) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.Namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.linkHandler = fs.NewLinkHandler(cmd.Namespace)
	cmd.secretHandler = fs.NewSecretHandler(cmd.Namespace)
}

func (cmd *CmdTokenRedeem) ValidateInput(args []string) error {
	var validationErrors []error
	tokenStringValidator := validator.NewFilePathStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.Namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.Namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

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

	// Validate if a token file exists
	if cmd.fileName != "" {
		_, err := os.Stat(cmd.fileName)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("cannot open token file: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}
func (cmd *CmdTokenRedeem) InputToOptions() {}
func (cmd *CmdTokenRedeem) Run() error {
	// get data from the access token file
	var accessToken v2alpha1.AccessToken
	var tokenFile []byte
	tokenFile, err := os.ReadFile(cmd.fileName)
	if err != nil {
		err = fmt.Errorf("unable to read token file - %v", err)
		return err
	}
	resource := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	_, _, err = resource.Decode(tokenFile, nil, &accessToken)
	if err != nil {
		return err
	}

	accessToken.Namespace = cmd.Namespace
	if accessToken.Name == "" {
		return fmt.Errorf("token name is required")
	}
	cmd.name = accessToken.Name

	// redeem the access token and store the secret and links into the input resources path
	// to redeem the token we use the namespace as a subject to allow redeeming tokens without an active site.
	decoder, err := nonkubecommon.RedeemAccessToken(&accessToken, cmd.Namespace)
	if err != nil {
		return err
	}

	decoder.Secret.Namespace = cmd.Namespace
	err = cmd.secretHandler.Add(decoder.Secret)
	if err != nil {
		return err
	}

	for _, link := range decoder.Links {
		link.Namespace = cmd.Namespace
		err = cmd.linkHandler.Add(link)
		if err != nil {
			return err
		}
	}

	fmt.Printf("Token %q has been redeemed.\n", cmd.name)

	return nil
}
func (cmd *CmdTokenRedeem) WaitUntil() error { return nil }
