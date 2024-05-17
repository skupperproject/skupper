package connector

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	connectorGetLong    = ""
	connectorGetExample = ""
)

type CmdConnectorGet struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdConnectorGet() *CmdConnectorGet {

	skupperCmd := CmdConnectorGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "",
		Long:    connectorGetLong,
		Example: connectorGetExample,
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

func (cmd *CmdConnectorGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdConnectorGet) AddFlags()                                            {}
func (cmd *CmdConnectorGet) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdConnectorGet) InputToOptions(args []string) error                   { return nil }
func (cmd *CmdConnectorGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdConnectorGet) WaitUntilReady() error                                { return nil }
