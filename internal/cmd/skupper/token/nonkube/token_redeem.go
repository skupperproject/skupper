package nonkube

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	"github.com/spf13/cobra"
)

type CmdTokenRedeem struct {
	CobraCmd           *cobra.Command
	Flags              *common.CommandTokenRedeemFlags
	Namespace          string
	siteName           string
	tokenHandler       *fs.TokenHandler
	accessTokenHandler *fs.AccessTokenHandler
	fileName           string
	name               string
}

func NewCmdTokenRedeem() *CmdTokenRedeem {
	return &CmdTokenRedeem{}
}

func (cmd *CmdTokenRedeem) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.Namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.tokenHandler = fs.NewTokenHandler(cmd.Namespace)
	cmd.accessTokenHandler = fs.NewAccessTokenHandler(cmd.Namespace)
}

func (cmd *CmdTokenRedeem) ValidateInput(args []string) error {
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
	// get data from the token file
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
	cmd.name = accessToken.Name

	err = cmd.accessTokenHandler.Add(accessToken)
	if err != nil {
		return err
	}

	fmt.Printf("Token %q has been created.\n", cmd.name)

	return nil
}
func (cmd *CmdTokenRedeem) WaitUntil() error { return nil }
