package nonkube

import (
	"errors"
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdSiteDelete struct {
	siteHandler         *fs.SiteHandler
	routerAccessHandler *fs.RouterAccessHandler
	listenerHandler     *fs.ListenerHandler
	connectorHandler    *fs.ConnectorHandler
	CobraCmd            *cobra.Command
	namespace           string
	siteName            string
	Flags               *common.CommandSiteDeleteFlags
	all                 bool
}

func NewCmdSiteDelete() *CmdSiteDelete {
	return &CmdSiteDelete{}
}

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)
	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
}

func (cmd *CmdSiteDelete) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: false, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameContext) != nil && cmd.CobraCmd.Flag(common.FlagNameContext).Value.String() != "" {
		fmt.Println("Warning: --context flag is not supported on this platform")
	}

	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig) != nil && cmd.CobraCmd.Flag(common.FlagNameKubeconfig).Value.String() != "" {
		fmt.Println("Warning: --kubeconfig flag is not supported on this platform")
	}

	// Validate arguments name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("site name must be specified"))
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

	if cmd.siteName != "" {
		// Validate that there is already a site with this name
		site, err := cmd.siteHandler.Get(cmd.siteName, opts)
		if site == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("site %s does not exist", cmd.siteName))
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteDelete) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
	cmd.all = cmd.Flags.All
}

func (cmd *CmdSiteDelete) Run() error {
	err := cmd.siteHandler.Delete(cmd.siteName)
	if err != nil {
		return err
	}
	err = cmd.routerAccessHandler.Delete("router-access-" + cmd.siteName)
	if err != nil {
		return err
	}
	// if delete all, also remove all the other resources
	if cmd.all {
		listeners, err := cmd.listenerHandler.List()
		if listeners != nil && err == nil {
			for _, listener := range listeners {
				fmt.Println("delete listener", listener.Name)
				err = cmd.listenerHandler.Delete(listener.Name)
				if err != nil {
					return err
				}
			}
		}
		connectors, err := cmd.connectorHandler.List()
		if connectors != nil && err == nil {
			for _, connector := range connectors {
				err = cmd.connectorHandler.Delete(connector.Name)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (cmd *CmdSiteDelete) WaitUntil() error { return nil }
