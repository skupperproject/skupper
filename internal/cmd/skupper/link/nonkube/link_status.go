package nonkube

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdLinkStatus struct {
	linkHandler *fs.LinkHandler
	siteHandler *fs.SiteHandler
	CobraCmd    *cobra.Command
	Flags       *common.CommandLinkStatusFlags
	namespace   string
	linkName    string
}

func NewCmdLinkStatus() *CmdLinkStatus {
	return &CmdLinkStatus{}
}

func (cmd *CmdLinkStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.linkHandler = fs.NewLinkHandler(cmd.namespace)
	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
}

func (cmd *CmdLinkStatus) ValidateInput(args []string) error {
	var validationErrors []error
	resourceStringValidator := validator.NewResourceStringValidator()

	// Validate arguments name if specified
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("link name must not be empty"))
		} else {
			ok, err := resourceStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("link name is not valid: %s", err))
			} else {
				cmd.linkName = args[0]
			}
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkStatus) Run() error {
	opts := fs.GetOptions{LogWarning: false}
	links, err := cmd.linkHandler.List(opts)
	if links == nil || err != nil {
		fmt.Println("no links found")
		return nil
	}
	if cmd.linkName == "" {
		tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
		_, _ = fmt.Fprintln(tw, fmt.Sprintf("%s\t%s",
			"NAME", "STATUS"))
		for _, link := range links {
			status := "Not Ready"
			if link.IsConfigured() {
				status = "Ok"
			}
			fmt.Fprintln(tw, fmt.Sprintf("%s\t%s",
				link.Name, status))
		}
		_ = tw.Flush()
	} else {
		for _, link := range links {
			if link.Name == cmd.linkName {
				status := "Not Ready"
				if link.IsConfigured() {
					status = "Ok"
				}

				// get the site and determine role of router, default to interRouter
				endpointName := ""
				endPointType := common.InterRouterRole
				sites, err := cmd.siteHandler.List(opts)
				if sites != nil && err == nil {
					if sites[0].Spec.Edge {
						endPointType = common.EdgeRole
					}
				}
				for index, endpoint := range link.Spec.Endpoints {
					if endpoint.Name == endPointType {
						endpointName = link.Spec.Endpoints[index].Host + ":" + link.Spec.Endpoints[index].Port
					}
				}

				tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
				fmt.Fprintln(tw, fmt.Sprintf("Name:\t%s\nStatus:\t%s\nCost:\t%d\nTlsCredentials:\t%s\nEndpoint:\t%s\n",
					link.Name, status, link.Spec.Cost, link.Spec.TlsCredentials, endpointName))
				_ = tw.Flush()
			}
		}
	}
	return nil
}

func (cmd *CmdLinkStatus) InputToOptions()  {}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }
