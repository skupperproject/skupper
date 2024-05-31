package listener

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	listenerGetLong    = ""
	listenerGetExample = ""
)

type CmdListenerGet struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdListenerGet() *CmdListenerGet {

	skupperCmd := CmdListenerGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "",
		Long:    listenerGetLong,
		Example: listenerGetExample,
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

func (cmd *CmdListenerGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdListenerGet) AddFlags()                                            {}
func (cmd *CmdListenerGet) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdListenerGet) InputToOptions()                                      {}
func (cmd *CmdListenerGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdListenerGet) WaitUntilReady() error                                { return nil }
