/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
)

var (
	siteCreateLong = `A site is a place where components of your application are running. 
Sites are linked to form application networks. This command will create a file in the specified input path 
for storing non-kube resources`
)

type CreateFlags struct {
	enableLinkAccess bool
	linkAccessType   string
	output           string
}

type CmdSiteCreate struct {
	CobraCmd       cobra.Command
	flags          CreateFlags
	options        map[string]string
	siteName       string
	linkAccessType string
	output         string
	inputPath      string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	options := make(map[string]string)
	skupperCmd := CmdSiteCreate{options: options, flags: CreateFlags{}}

	cmd := cobra.Command{
		Use:    "create <name> <input-path>",
		Short:  "Create a new site",
		Long:   siteCreateLong,
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

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {}

func (cmd *CmdSiteCreate) AddFlags() {
	cmd.CobraCmd.Flags().BoolVar(&cmd.flags.enableLinkAccess, "enable-link-access", false, "allow access for incoming links from remote sites (default: false)")
	cmd.CobraCmd.Flags().StringVar(&cmd.flags.linkAccessType, "link-access-type", "", `configure external access for links from remote sites.
Choices: [route|loadbalancer]. Default: On OpenShift, route is the default; 
for other Kubernetes flavors, loadbalancer is the default.`)
	cmd.CobraCmd.Flags().StringVarP(&cmd.flags.output, "output", "o", "", "print resources to the console instead of submitting them to the Skupper controller. Choices: json, yaml")
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(utils.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(utils.OutputTypes)

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

	if cmd.flags.linkAccessType != "" {
		ok, err := linkAccessTypeValidator.Evaluate(cmd.flags.linkAccessType)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("link access type is not valid: %s", err))
		}
	}

	if !cmd.flags.enableLinkAccess && len(cmd.flags.linkAccessType) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("for the site to work with this type of linkAccess, the --enable-link-access option must be set to true"))
	}

	if cmd.flags.output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.flags.output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdSiteCreate) InputToOptions() {

	if cmd.flags.enableLinkAccess {
		if cmd.flags.linkAccessType == "" {
			cmd.linkAccessType = "default"
		} else {
			cmd.linkAccessType = cmd.flags.linkAccessType
		}
	} else {
		cmd.linkAccessType = "none"
	}

	options := make(map[string]string)
	options[site.SiteConfigNameKey] = cmd.siteName

	cmd.options = options

	cmd.output = cmd.flags.output

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
		encodedOutput, err := utils.Encode(cmd.output, resource)
		fmt.Println(encodedOutput)

		return err

	} else {
		encodedOutput, err := utils.Encode("yaml", resource)
		if err != nil {
			return err
		}
		fileName := fmt.Sprintf("%s/site-%s.yaml", cmd.inputPath, cmd.siteName)
		err = utils.WriteFile(fileName, encodedOutput)

		return err
	}

}

func (cmd *CmdSiteCreate) WaitUntilReady() error {
	//TODO check status of the site
	return nil
}
