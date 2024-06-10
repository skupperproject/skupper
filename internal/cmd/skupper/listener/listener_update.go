package listener

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	listenerUpdateLong    = ""
	listenerUpdateExample = ""
)

type CmdListenerUpdate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdListenerUpdate() *CmdListenerUpdate {

	skupperCmd := CmdListenerUpdate{}

	cmd := cobra.Command{
		Use:     "update",
		Short:   "",
		Long:    listenerUpdateLong,
		Example: listenerUpdateExample,
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

func (cmd *CmdListenerUpdate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdListenerUpdate) AddFlags()                                            {}
func (cmd *CmdListenerUpdate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdListenerUpdate) InputToOptions()                                      {}
func (cmd *CmdListenerUpdate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdListenerUpdate) WaitUntilReady() error                                { return nil }
