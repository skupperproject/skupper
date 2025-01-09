package nonkube

import (
	"fmt"
<<<<<<< HEAD
=======
	"os"
	"text/tabwriter"
>>>>>>> 5122e05 (add nonkube link status support)

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
)

type CmdLinkStatus struct {
	linkHandler *fs.LinkHandler
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
}

func (cmd *CmdLinkStatus) ValidateInput(args []string) []error {
	var validationErrors []error
	opts := fs.GetOptions{LogWarning: false}
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
	// Validate that there is a link with this name in the namespace
	if cmd.linkName != "" {
		link, err := cmd.linkHandler.Get(cmd.linkName, opts)
		if link == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("link %s does not exist in namespace %s", cmd.linkName, cmd.namespace))
		}
	}
	return validationErrors
}

func (cmd *CmdLinkStatus) Run() error {
	opts := fs.GetOptions{LogWarning: true}
	if cmd.linkName == "" {
		links, err := cmd.linkHandler.List()
		if links == nil || err != nil {
			fmt.Println("no links found:")
			return err
		}

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
		link, err := cmd.linkHandler.Get(cmd.linkName, opts)
		if link == nil || err != nil {
			fmt.Println("No links found:")
			return err
		}
		status := "Not Ready"
		if link.IsConfigured() {
			status = "Ok"
		}
		tw := tabwriter.NewWriter(os.Stdout, 8, 8, 1, '\t', tabwriter.TabIndent)
		fmt.Fprintln(tw, fmt.Sprintf("Name:\t%s\nStatus:\t%s\nCost:\t%d\nTlsCredentials:\t%s\n",
			link.Name, status, link.Spec.Cost, link.Spec.TlsCredentials))
		_ = tw.Flush()
	}
	return nil
}

func (cmd *CmdLinkStatus) InputToOptions()  {}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }
