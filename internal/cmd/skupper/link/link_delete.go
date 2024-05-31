package link

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	linkDeleteLong    = ""
	linkDeleteExample = ""
)

type CmdLinkDelete struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdLinkDelete() *CmdLinkDelete {

	skupperCmd := CmdLinkDelete{}

	cmd := cobra.Command{
		Use:     "delete",
		Short:   "",
		Long:    linkDeleteLong,
		Example: linkDeleteExample,
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

func (cmd *CmdLinkDelete) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkDelete) AddFlags()                                            {}
func (cmd *CmdLinkDelete) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdLinkDelete) InputToOptions()                                      {}
func (cmd *CmdLinkDelete) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkDelete) WaitUntilReady() error                                { return nil }
