package connector

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	connectorDeleteLong    = ""
	connectorDeleteExample = ""
)

type CmdConnectorDelete struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdConnectorDelete() *CmdConnectorDelete {

	skupperCmd := CmdConnectorDelete{}

	cmd := cobra.Command{
		Use:     "delete",
		Short:   "",
		Long:    connectorDeleteLong,
		Example: connectorDeleteExample,
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

func (cmd *CmdConnectorDelete) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdConnectorDelete) AddFlags()                                            {}
func (cmd *CmdConnectorDelete) ValidateFlags() []error                               { return nil }
func (cmd *CmdConnectorDelete) FlagsToOptions() error                                { return nil }
func (cmd *CmdConnectorDelete) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdConnectorDelete) WaitUntilReady() bool                                 { return true }
