/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SiteConfigNameKey string = "name"
)

type CmdSiteCreate struct {
	siteHandler             *fs.SiteHandler
	routerAccessHandler     *fs.RouterAccessHandler
	CobraCmd                *cobra.Command
	Flags                   *common.CommandSiteCreateFlags
	options                 map[string]string
	siteName                string
	linkAccessType          string
	output                  string
	namespace               string
	bindHost                string
	routerAccessName        string
	subjectAlternativeNames []string
}

func NewCmdSiteCreate() *CmdSiteCreate {
	return &CmdSiteCreate{}
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)

}

func (cmd *CmdSiteCreate) ValidateInput(args []string) []error {

	var validationErrors []error

	if cmd.Flags.ServiceAccount != "" {
		fmt.Println("Warning: --service-account flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
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

	if cmd.Flags.BindHost == "" {
		validationErrors = append(validationErrors, fmt.Errorf("bind host should not be empty"))
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
	options[SiteConfigNameKey] = cmd.siteName

	cmd.options = options
	cmd.output = cmd.Flags.Output

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

	cmd.bindHost = cmd.Flags.BindHost
	cmd.routerAccessName = "router-access-" + cmd.siteName
	cmd.subjectAlternativeNames = cmd.Flags.SubjectAlternativeNames

}

func (cmd *CmdSiteCreate) Run() error {

	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.siteName,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.SiteSpec{
			Settings:   cmd.options,
			LinkAccess: cmd.linkAccessType,
		},
	}

	routerAccessResource := v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.routerAccessName,
			Namespace: cmd.namespace,
		},
		Spec: v2alpha1.RouterAccessSpec{
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "inter-router",
					Port: 55671,
				},
				{
					Name: "edge",
					Port: 45671,
				},
			},
			BindHost:                cmd.bindHost,
			SubjectAlternativeNames: cmd.subjectAlternativeNames,
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
		err := cmd.siteHandler.Add(siteResource)
		if err != nil {
			return err
		}

		err = cmd.routerAccessHandler.Add(routerAccessResource)
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
