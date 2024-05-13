package connector

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	connectorCreateLong    = ""
	connectorCreateExample = ""
)

type CmdConnectorCreate struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdConnectorCreate() *CmdConnectorCreate {

	skupperCmd := CmdConnectorCreate{}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "",
		Long:    connectorCreateLong,
		Example: connectorCreateExample,
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

func (cmd *CmdConnectorCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdConnectorCreate) AddFlags()                                            {}
func (cmd *CmdConnectorCreate) ValidateFlags() []error                               { return nil }
func (cmd *CmdConnectorCreate) FlagsToOptions() error                                { return nil }
func (cmd *CmdConnectorCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdConnectorCreate) WaitUntilReady() bool                                 { return true }
