/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/non-kube/client"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteCreate struct {
	pathProvider     client.PathProvider
	CobraCmd         *cobra.Command
	Flags            *common.CommandSiteCreateFlags
	options          map[string]string
	siteName         string
	linkAccessType   string
	output           string
	inputPath        string
	namespace        string
	host             string
	routerAccessName string
}

func NewCmdSiteCreate() *CmdSiteCreate {

	options := make(map[string]string)
	skupperCmd := CmdSiteCreate{options: options}

	return &skupperCmd
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.pathProvider = client.PathProvider{}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.pathProvider.Namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
		cmd.namespace = cmd.pathProvider.Namespace
	}
}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error

	if cmd.Flags.ServiceAccount != "" {
		validationErrors = append(validationErrors, fmt.Errorf("--service-account flag is not supported on this platform"))
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		validationErrors = append(validationErrors, fmt.Errorf("--context flag is not supported on this platform"))
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		validationErrors = append(validationErrors, fmt.Errorf("--kubeconfig flag is not supported on this platform"))
	}

	resourceStringValidator := validator.NewResourceStringValidator()
	linkAccessTypeValidator := validator.NewOptionValidator(common.LinkAccessTypes)
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	if len(args) == 0 || args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
		}
		cmd.siteName = args[0]

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

	if cmd.Flags.Host == "" {
		validationErrors = append(validationErrors, fmt.Errorf("host should not be empty"))
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

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
	cmd.inputPath = cmd.pathProvider.GetDefaultNamespace()
	cmd.host = cmd.Flags.Host
	cmd.routerAccessName = "router-access-" + cmd.siteName

}

func (cmd *CmdSiteCreate) Run() error {

	siteResource := v1alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.siteName,
			Namespace: cmd.namespace,
		},
		Spec: v1alpha1.SiteSpec{
			Settings:   cmd.options,
			LinkAccess: cmd.linkAccessType,
		},
	}

	routerAccessResource := v1alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.routerAccessName,
			Namespace: cmd.namespace,
		},
		Spec: v1alpha1.RouterAccessSpec{
			Roles: []v1alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: 55671,
				},
				{
					Name: "edge",
					Port: 45671,
				},
			},
			BindHost: cmd.host,
		},
	}

	if cmd.output != "" {
		encodedSiteOutput, err := utils.Encode(cmd.output, siteResource)
		fmt.Println(encodedSiteOutput)
		fmt.Println("---")
		encodedRouterAccessOutput, err := utils.Encode(cmd.output, routerAccessResource)
		fmt.Println(encodedRouterAccessOutput)

		return err

	} else {
		encodedSite, err := utils.Encode("yaml", siteResource)
		if err != nil {
			return err
		}
		fileName := fmt.Sprintf("site-%s.yaml", cmd.siteName)
		err = utils.WriteFile(cmd.inputPath, fileName, encodedSite)
		if err != nil {
			return err
		}

		encodedRouterAccess, err := utils.Encode("yaml", routerAccessResource)
		if err != nil {
			return err
		}
		fileName = fmt.Sprintf("%s.yaml", cmd.routerAccessName)
		err = utils.WriteFile(cmd.inputPath, fileName, encodedRouterAccess)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *CmdSiteCreate) WaitUntil() error {
	//TODO check status of the site
	return nil
}
