package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CmdSiteUpdate struct {
	siteHandler             *fs.SiteHandler
	routerAccessHandler     *fs.RouterAccessHandler
	CobraCmd                *cobra.Command
	Flags                   *common.CommandSiteUpdateFlags
	siteName                string
	namespace               string
	linkAccessEnabled       bool
	bindHost                string
	routerAccessName        string
	subjectAlternativeNames []string
}

func NewCmdSiteUpdate() *CmdSiteUpdate {
	return &CmdSiteUpdate{}
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)
}

func (cmd *CmdSiteUpdate) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
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

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("site name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
	} else {
		ok, err := resourceStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
		} else {
			cmd.siteName = args[0]
		}
	}

	// Validate that there is already a site with this name in the namespace
	if cmd.siteName != "" {
		site, err := cmd.siteHandler.Get(cmd.siteName, opts)
		if site == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("site %s must exist to be updated", cmd.siteName))
		}

		routerAccessName := "router-access-" + cmd.siteName
		routerAccess, err := cmd.routerAccessHandler.Update(routerAccessName)
		if err == nil && routerAccess != nil {
			// save existing values
			cmd.bindHost = routerAccess.Spec.BindHost
			cmd.subjectAlternativeNames = routerAccess.Spec.SubjectAlternativeNames
			cmd.linkAccessEnabled = true
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteUpdate) InputToOptions() {
	// if EnableLinkAccess flag was explicitly set use value otherwise use value from
	// previous create command
	if cmd.CobraCmd.Flags().Changed(common.FlagNameEnableLinkAccess) {
		if cmd.Flags.EnableLinkAccess == true {
			cmd.linkAccessEnabled = true
		} else {
			cmd.linkAccessEnabled = false
		}
	}

	cmd.routerAccessName = "router-access-" + cmd.siteName

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}

}

func (cmd *CmdSiteUpdate) Run() error {

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
			BindHost:                cmd.bindHost,
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
	} else {
		fmt.Println("link access not enabled, router access removed", cmd.routerAccessName)
		err = cmd.routerAccessHandler.Delete(cmd.routerAccessName)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *CmdSiteUpdate) WaitUntil() error {
	//TODO check status of the site
	return nil
}
