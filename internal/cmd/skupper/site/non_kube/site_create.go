/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	utils2 "github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

type CmdSiteCreate struct {
	CobraCmd       *cobra.Command
	Flags          *common.CommandSiteCreateFlags
	options        map[string]string
	siteName       string
	linkAccessType string
	output         string
	inputPath      string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	options := make(map[string]string)
	skupperCmd := CmdSiteCreate{options: options}

	return &skupperCmd
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(common.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	if len(args) < 2 || args[0] == "" || args[1] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name and input path must be provided"))
	} else if len(args) > 3 {
		validationErrors = append(validationErrors, fmt.Errorf("only two arguments are allowed for this command."))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
		}
		cmd.siteName = args[0]

		_, err = os.Stat(args[1])
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("input path does not exist: %s", err))
		}
		cmd.inputPath = args[1]
	}

	if cmd.Flags.LinkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.Flags.LinkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if !cmd.Flags.EnableLinkAccess && len(cmd.Flags.LinkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdSiteCreate) InputToOptions() {

	if cmd.Flags.EnableLinkAccess {
		if cmd.Flags.LinkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.Flags.LinkAccessType
		}
	} else {
		cmd.linkAccessType = "none"
	}

	options := make(map[string]string)
	options[site.SiteConfigNameKey] = cmd.siteName

	cmd.options = options

	cmd.output = cmd.Flags.Output

}

func (cmd *CmdSiteCreate) Run() error {

	resource := v1alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: cmd.siteName,
		},
		Spec: v1alpha1.SiteSpec{
			Settings:   cmd.options,
			LinkAccess: cmd.linkAccessType,
		},
	}

	if cmd.output != "" {
		encodedOutput, err := utils2.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		encodedOutput, err := utils2.Encode("yaml", resource)
		if err != nil {
			return err
		}
		fileName := fmt.Sprintf("%s/site-%s.yaml", cmd.inputPath, cmd.siteName)
		err = utils2.WriteFile(fileName, encodedOutput)

		return err
	}

}

func (cmd *CmdSiteCreate) WaitUntil() error {
	//TODO check status of the site
	return nil
}
