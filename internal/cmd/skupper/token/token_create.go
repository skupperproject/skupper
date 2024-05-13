package token

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

var (
	tokenCreateLong    = ""
	tokenCreateExample = ""
)

type CmdTokenCreate struct {
	client   *client.VanClient
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
			utils.HandleErrorList(skupperCmd.ValidateFlags())
			utils.HandleError(skupperCmd.FlagsToOptions())
			utils.HandleError(skupperCmd.Run())
		},
	}

	skupperCmd.CobraCmd = cmd

	return &skupperCmd
}

func (cmd *CmdTokenCreate) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdTokenCreate) AddFlags()                                            {}
func (cmd *CmdTokenCreate) ValidateFlags() []error                               { return nil }
func (cmd *CmdTokenCreate) FlagsToOptions() error                                { return nil }
func (cmd *CmdTokenCreate) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdTokenCreate) WaitUntilReady() bool                                 { return true }
