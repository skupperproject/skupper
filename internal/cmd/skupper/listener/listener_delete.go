package listener

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	listenerDeleteLong    = ""
	listenerDeleteExample = ""
)

type CmdListenerDelete struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdListenerDelete() *CmdListenerDelete {

	skupperCmd := CmdListenerDelete{}

	cmd := cobra.Command{
		Use:     "delete",
		Short:   "",
		Long:    listenerDeleteLong,
		Example: listenerDeleteExample,
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

func (cmd *CmdListenerDelete) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdListenerDelete) AddFlags()                                            {}
func (cmd *CmdListenerDelete) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdListenerDelete) InputToOptions()                                      {}
func (cmd *CmdListenerDelete) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdListenerDelete) WaitUntilReady() error                                { return nil }
