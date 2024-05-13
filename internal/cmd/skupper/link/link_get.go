package link

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	linkGetLong    = ""
	linkGetExample = ""
)

type CmdLinkGet struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdLinkGet() *CmdLinkGet {

	skupperCmd := CmdLinkGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "",
		Long:    linkGetLong,
		Example: linkGetExample,
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

func (cmd *CmdLinkGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkGet) AddFlags()                                            {}
func (cmd *CmdLinkGet) ValidateFlags() []error                               { return nil }
func (cmd *CmdLinkGet) FlagsToOptions() error                                { return nil }
func (cmd *CmdLinkGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkGet) WaitUntilReady() bool                                 { return true }
