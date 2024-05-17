package connector

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	connectorUpdateLong    = ""
	connectorUpdateExample = ""
)

type CmdConnectorUpdate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdConnectorUpdate() *CmdConnectorUpdate {

	skupperCmd := CmdConnectorUpdate{}

	cmd := cobra.Command{
		Use:     "update",
		Short:   "",
		Long:    connectorUpdateLong,
		Example: connectorUpdateExample,
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

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdConnectorUpdate) AddFlags()                                            {}
func (cmd *CmdConnectorUpdate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdConnectorUpdate) InputToOptions(args []string) error                   { return nil }
func (cmd *CmdConnectorUpdate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdConnectorUpdate) WaitUntilReady() error                                { return nil }
