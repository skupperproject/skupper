package listener

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	listenerCreateLong    = ""
	listenerCreateExample = ""
)

type CmdListenerCreate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdListenerCreate() *CmdListenerCreate {

	skupperCmd := CmdListenerCreate{}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "",
		Long:    listenerCreateLong,
		Example: listenerCreateExample,
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

func (cmd *CmdListenerCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdListenerCreate) AddFlags()                                            {}
func (cmd *CmdListenerCreate) ValidateFlags() []error                               { return nil }
func (cmd *CmdListenerCreate) FlagsToOptions() error                                { return nil }
func (cmd *CmdListenerCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdListenerCreate) WaitUntilReady() bool                                 { return true }
