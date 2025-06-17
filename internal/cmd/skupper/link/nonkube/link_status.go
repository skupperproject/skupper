package nonkube

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"

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
	output      string
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
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

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

	if cmd.Flags.Output != "" {
		ok, _ := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("format bad-value not supported"))
		}
	}
	return errors.Join(validationErrors...)
}

func (cmd *CmdLinkStatus) Run() error {

	if cmd.linkName != "" {
		selectedLink, err := cmd.linkHandler.Get(cmd.linkName, fs.GetOptions{LogWarning: false, RuntimeFirst: true})
		if err != nil {
			return fmt.Errorf("There is no link resource in the namespace with the name %q", cmd.linkName)
		}

		if cmd.output != "" {
			return printEncodedOuptut(cmd.output, selectedLink)
		} else {
			displaySingleLink(selectedLink)
		}

	} else {

		linkList, err := cmd.linkHandler.List(fs.GetOptions{RuntimeFirst: true, LogWarning: false})
		if err != nil {
			return err
		}

		if linkList != nil && len(linkList) == 0 {
			fmt.Println("There are no link resources in the namespace")
			return nil
		}

		if cmd.output != "" {
			for _, link := range linkList {
				err := printEncodedOuptut(cmd.output, link)

				if err != nil {
					return err
				}
			}
		} else {
			displayLinkList(linkList)
		}

	}

	return nil
}

func (cmd *CmdLinkStatus) InputToOptions() {
	cmd.output = cmd.Flags.Output
}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }

func printEncodedOuptut(outputType string, link *v2alpha1.Link) error {
	encodedOutput, err := utils.Encode(outputType, link)
	fmt.Println(encodedOutput)
	return err
}

func displaySingleLink(link *v2alpha1.Link) {
	fmt.Printf("%s\t: %s\n", "Name", link.Name)
	fmt.Printf("%s\t: %s\n", "Status", link.Status.StatusType)
	fmt.Printf("%s\t: %d\n", "Cost", link.Spec.Cost)
	fmt.Printf("%s\t: %s\n", "Message", link.Status.Message)
}

func displayLinkList(linkList []*v2alpha1.Link) {
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
	fmt.Fprintln(writer, "NAME\tSTATUS\tCOST\tMESSAGE")

	for _, link := range linkList {
		fmt.Fprintf(writer, "%s\t%s\t%d\t%s", link.Name, link.Status.StatusType, link.Spec.Cost, link.Status.Message)
		fmt.Fprintln(writer)
	}

	writer.Flush()
}
