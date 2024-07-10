package connector

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

var (
	connectorUpdateLong    = ""
	connectorUpdateExample = ""
)

type CmdConnectorUpdate struct {
	client   *client.KubeClient
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
			skupperCmd.InputToOptions()
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdConnectorUpdate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdConnectorUpdate) AddFlags()                                            {}
func (cmd *CmdConnectorUpdate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdConnectorUpdate) InputToOptions()                                      {}
func (cmd *CmdConnectorUpdate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdConnectorUpdate) WaitUntilReady() error                                { return nil }
