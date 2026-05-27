package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"

	"github.com/spf13/cobra"
)

type CmdSiteUpdate struct {
	siteHandler       *fs.SiteHandler
	CobraCmd          *cobra.Command
	Flags             *common.CommandSiteUpdateFlags
	siteName          string
	namespace         string
	linkAccessEnabled bool
}

func NewCmdSiteUpdate() *CmdSiteUpdate {
	return &CmdSiteUpdate{}
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
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
		} else {
			// Check current linkAccess setting
			if site.Spec.LinkAccess != "" && site.Spec.LinkAccess != "none" {
				cmd.linkAccessEnabled = true
			}
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteUpdate) InputToOptions() {
	// if EnableLinkAccess flag was explicitly set use value otherwise use value from
	// previous create command
	if cmd.CobraCmd.Flags().Changed(common.FlagNameEnableLinkAccess) {
		cmd.linkAccessEnabled = cmd.Flags.EnableLinkAccess
	}

	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
}

func (cmd *CmdSiteUpdate) Run() error {
	// Get existing site to preserve other settings
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	existingSite, err := cmd.siteHandler.Get(cmd.siteName, opts)
	if err != nil {
		return fmt.Errorf("failed to get existing site: %w", err)
	}

	if cmd.linkAccessEnabled {
		existingSite.Spec.LinkAccess = "default"
	} else {
		existingSite.Spec.LinkAccess = "none"
	}

	err = cmd.siteHandler.Add(*existingSite)
	if err != nil {
		return err
	}

	return nil
}

func (cmd *CmdSiteUpdate) WaitUntil() error {
	//TODO check status of the site
	return nil
}
