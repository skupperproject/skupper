package site

import (
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/utils"
	"github.com/spf13/cobra"
)

type CmdSiteUpdate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdSiteUpdate() *CmdSiteUpdate {

	skupperCmd := CmdSiteUpdate{}

	cmd := cobra.Command{
		Use:     "update",
		Short:   "update the site",
		Long:    siteCreateLong,
		Example: siteCreateExample,
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateFlags())
			utils.HandleError(skupperCmd.FlagsToOptions())
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdSiteUpdate) AddFlags()                                            {}
func (cmd *CmdSiteUpdate) ValidateFlags() []error                               { return nil }
func (cmd *CmdSiteUpdate) FlagsToOptions() error                                { return nil }
func (cmd *CmdSiteUpdate) Run() error                                           { return nil }
func (cmd *CmdSiteUpdate) WaitUntilReady() bool                                 { return true }
