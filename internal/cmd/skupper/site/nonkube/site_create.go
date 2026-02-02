/*
Copyright © 2024 Skupper Team <skupper@googlegroups.com>
*/
package nonkube

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteCreate struct {
	siteHandler             *fs.SiteHandler
	routerAccessHandler     *fs.RouterAccessHandler
	CobraCmd                *cobra.Command
	Flags                   *common.CommandSiteCreateFlags
	siteName                string
	linkAccessEnabled       bool
	namespace               string
	routerAccessName        string
	subjectAlternativeNames []string
	logger                  *slog.Logger
}

func NewCmdSiteCreate() *CmdSiteCreate {
	return &CmdSiteCreate{
		logger: slog.New(slog.Default().Handler()).With("component", "nonkube.siteCreate"),
	}
}

func (cmd *CmdSiteCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)

}

func (cmd *CmdSiteCreate) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()
	namespaceStringValidator := validator.NamespaceStringValidator()

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	if cmd.namespace != "" {
		ok, err := namespaceStringValidator.Evaluate(cmd.namespace)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("namespace is not valid: %s", err))
		}
	}

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

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteCreate) InputToOptions() {

	if cmd.Flags.EnableLinkAccess {
		sanByDefault, err := utils.GetSansByDefault()
		if err != nil {
			cmd.logger.Error("Error getting SANs by default")
		}

		cmd.linkAccessEnabled = true
		cmd.routerAccessName = "router-access-" + cmd.siteName
		cmd.subjectAlternativeNames = sanByDefault
	}

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

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
			SubjectAlternativeNames: cmd.subjectAlternativeNames,
		},
	}

	err := cmd.siteHandler.Add(siteResource)
	if err != nil {
		return err
	}

	if cmd.linkAccessEnabled == true {
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
