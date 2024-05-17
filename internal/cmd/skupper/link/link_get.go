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
			utils.HandleErrorList(skupperCmd.ValidateInput(args))
			utils.HandleError(skupperCmd.InputToOptions(args))
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdLinkGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkGet) AddFlags()                                            {}
func (cmd *CmdLinkGet) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdLinkGet) InputToOptions(args []string) error                   { return nil }
func (cmd *CmdLinkGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkGet) WaitUntilReady() error                                { return nil }
