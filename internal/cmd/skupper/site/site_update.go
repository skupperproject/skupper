package site

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
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
		Long:    "",
		Example: "",
		PreRun:  skupperCmd.NewClient,
		Run: func(cmd *cobra.Command, args []string) {
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			skupperCmd.InputToOptions()
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdSiteUpdate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdSiteUpdate) AddFlags()                                            {}
func (cmd *CmdSiteUpdate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdSiteUpdate) InputToOptions()                                      {}
func (cmd *CmdSiteUpdate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdSiteUpdate) WaitUntilReady() error                                { return nil }
