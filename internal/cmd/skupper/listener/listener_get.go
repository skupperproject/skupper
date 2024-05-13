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
			utils.HandleErrorList(skupperCmd.ValidateFlags())
			utils.HandleError(skupperCmd.FlagsToOptions())
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdListenerGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdListenerGet) AddFlags()                                            {}
func (cmd *CmdListenerGet) ValidateFlags() []error                               { return nil }
func (cmd *CmdListenerGet) FlagsToOptions() error                                { return nil }
func (cmd *CmdListenerGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdListenerGet) WaitUntilReady() bool                                 { return true }
