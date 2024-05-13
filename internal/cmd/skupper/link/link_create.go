package link

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	linkCreateLong    = ""
	linkCreateExample = ""
)

type CmdLinkCreate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdLinkCreate() *CmdLinkCreate {

	skupperCmd := CmdLinkCreate{}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "",
		Long:    linkCreateLong,
		Example: linkCreateExample,
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

func (cmd *CmdLinkCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkCreate) AddFlags()                                            {}
func (cmd *CmdLinkCreate) ValidateFlags() []error                               { return nil }
func (cmd *CmdLinkCreate) FlagsToOptions() error                                { return nil }
func (cmd *CmdLinkCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkCreate) WaitUntilReady() bool                                 { return true }
