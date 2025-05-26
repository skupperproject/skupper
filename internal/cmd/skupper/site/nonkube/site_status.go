package nonkube

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/spf13/cobra"
)

type CmdSiteStatus struct {
	siteHandler *fs.SiteHandler
	CobraCmd    *cobra.Command
	Flags       *common.CommandSiteStatusFlags
	namespace   string
	siteName    string
	output      string
}

func NewCmdSiteStatus() *CmdSiteStatus {
	return &CmdSiteStatus{}
}

func (cmd *CmdSiteStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
}

func (cmd *CmdSiteStatus) ValidateInput(args []string) error {
	var validationErrors []error
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	resourceStringValidator := validator.NewResourceStringValidator()
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)

	// Validate arguments name if specified
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("site name must not be empty"))
		} else {
			ok, err := resourceStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("site name is not valid: %s", err))
			} else {
				cmd.siteName = args[0]
			}
		}
	}
	// Validate that there is a site with this name in the namespace
	if cmd.siteName != "" {
		site, err := cmd.siteHandler.Get(cmd.siteName, opts)
		if site == nil || err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("site %s does not exist", cmd.siteName))
		}
	}

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		} else {
			cmd.output = cmd.Flags.Output
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdSiteStatus) Run() error {
	opts := fs.GetOptions{LogWarning: true}
	sites, err := cmd.siteHandler.List(opts)
	if sites == nil || err != nil {
		fmt.Println("There is no existing Skupper site resource")
		return nil
	}

	if cmd.output != "" {
		for _, site := range sites {
			encodedOutput, err := utils.Encode(cmd.output, site)
			if err != nil {
				return err
			}
			fmt.Println(encodedOutput)
		}
	} else {
		writer := tabwriter.NewWriter(os.Stdout, 0, 8, 1, '\t', tabwriter.AlignRight)
		fmt.Fprintln(writer, "NAME\tSTATUS\tMESSAGE")

		for _, site := range sites {
			fmt.Fprintf(writer, "%s\t%s\t%s", site.Name, site.Status.StatusType, site.Status.Message)
			fmt.Fprintln(writer)
		}

		writer.Flush()
	}

	return nil
}

func (cmd *CmdSiteStatus) InputToOptions()  {}
func (cmd *CmdSiteStatus) WaitUntil() error { return nil }
