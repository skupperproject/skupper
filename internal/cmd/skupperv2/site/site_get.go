package site

import (
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupperv2/utils"
	"github.com/spf13/cobra"
)

type CmdSiteGet struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdSiteGet() *CmdSiteGet {

	skupperCmd := CmdSiteGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "get the site",
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

func (cmd *CmdSiteGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdSiteGet) AddFlags()                                            {}
func (cmd *CmdSiteGet) ValidateFlags() []error                               { return nil }
func (cmd *CmdSiteGet) FlagsToOptions() error                                { return nil }
func (cmd *CmdSiteGet) Run() error                                           { return nil }
func (cmd *CmdSiteGet) WaitUntilReady() bool                                 { return true }
