package token

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

var (
	tokenCreateLong    = ""
	tokenCreateExample = ""
)

type CmdTokenCreate struct {
	client   *client.KubeClient
	CobraCmd cobra.Command
}

func NewCmdTokenCreate() *CmdTokenCreate {

	skupperCmd := CmdTokenCreate{}

	cmd := cobra.Command{
		Use:     "create",
		Short:   "Create a token",
		Long:    tokenCreateLong,
		Example: tokenCreateExample,
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

func (cmd *CmdTokenCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdTokenCreate) AddFlags()                                            {}
func (cmd *CmdTokenCreate) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdTokenCreate) InputToOptions()                                      {}
func (cmd *CmdTokenCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdTokenCreate) WaitUntilReady() error                                { return nil }
