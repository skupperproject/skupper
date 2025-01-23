package nonkube

import (
	"fmt"
	"net"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SiteUpdates struct {
	subjectAlternativeNames []string
	bindHost                string
	options                 map[string]string
}

type CmdSiteUpdate struct {
	siteHandler             *fs.SiteHandler
	routerAccessHandler     *fs.RouterAccessHandler
	CobraCmd                *cobra.Command
	Flags                   *common.CommandSiteUpdateFlags
	options                 map[string]string
	siteName                string
	namespace               string
	linkAccessEnabled       bool
	bindHost                string
	output                  string
	routerAccessName        string
	subjectAlternativeNames []string
	newSettings             SiteUpdates
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

func (cmd *CmdSiteUpdate) ValidateInput(args []string) []error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	hostStringValidator := validator.NewHostStringValidator()

	if cmd.Flags.ServiceAccount != "" {
		fmt.Println("Warning: --service-account flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
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
		} else {
			// save existing values

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

	// Validate flags
	if cmd.Flags != nil && cmd.Flags.BindHost != "" {
		ip := net.ParseIP(cmd.Flags.BindHost)
		ok, _ := hostStringValidator.Evaluate(cmd.Flags.BindHost)
		if !ok && ip == nil {
			validationErrors = append(validationErrors, fmt.Errorf("bindhost is not valid: a valid IP address or hostname is expected"))
		} else {
			cmd.newSettings.bindHost = cmd.Flags.BindHost
		}
	}
	if cmd.Flags != nil && len(cmd.Flags.SubjectAlternativeNames) != 0 {
		for _, name := range cmd.Flags.SubjectAlternativeNames {
			ip := net.ParseIP(name)
			ok, _ := hostStringValidator.Evaluate(name)
			if !ok && ip == nil {
				validationErrors = append(validationErrors, fmt.Errorf("SubjectAlternativeNames are not valid: a valid IP address or hostname is expected"))
			} else {
				cmd.newSettings.subjectAlternativeNames = append(cmd.newSettings.subjectAlternativeNames, name)
			}
		}
	}
	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return validationErrors
}

func (cmd *CmdSiteUpdate) InputToOptions() {
	// if EnableLinkAccess flag was explicity set use value otherwise use value from
	// previous create command
	if cmd.CobraCmd.Flags().Changed(common.FlagNameEnableLinkAccess) {
		if cmd.Flags.EnableLinkAccess == true {
			cmd.linkAccessEnabled = true
		} else {
			cmd.linkAccessEnabled = false
		}
	}

	if cmd.linkAccessEnabled == true {
		if cmd.newSettings.bindHost != "" {
			cmd.bindHost = cmd.newSettings.bindHost
		}
		if len(cmd.newSettings.subjectAlternativeNames) != 0 {
			cmd.subjectAlternativeNames = cmd.newSettings.subjectAlternativeNames
		}
	}
	options := make(map[string]string)
	options[common.SiteConfigNameKey] = cmd.siteName

	cmd.options = options
	cmd.output = cmd.Flags.Output
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
		Spec: v2alpha1.SiteSpec{
			Settings: cmd.options,
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
		if err != nil {
			return err
		}
		fmt.Println(encodedSiteOutput)
		if cmd.linkAccessEnabled == true {
			fmt.Println("---")
			encodedRouterAccessOutput, err := utils.Encode(cmd.output, routerAccessResource)
			if err != nil {
				return err
			}
			fmt.Println(encodedRouterAccessOutput)
		}
		return err
	} else {
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
	}

	return nil
}

func (cmd *CmdSiteUpdate) WaitUntil() error {
	//TODO check status of the site
	return nil
}
