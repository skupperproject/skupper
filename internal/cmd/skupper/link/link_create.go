package link

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

var (
	linkCreateLong    = ""
	linkCreateExample = ""
)

type CmdLinkCreate struct {
	client   *client.KubeClient
	CobraCmd cobra.Command
}

func NewCmdLinkCreate() *CmdLinkCreate {

	skupperCmd := CmdLinkCreate{}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "",
		Long:    linkCreateLong,
		Example: linkCreateExample,
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

func (cmd *CmdLinkCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkCreate) AddFlags()                                            {}
func (cmd *CmdLinkCreate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdLinkCreate) InputToOptions()                                      {}
func (cmd *CmdLinkCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkCreate) WaitUntilReady() error                                { return nil }
