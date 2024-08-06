package link

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	"github.com/spf13/cobra"
)

var (
	linkGetLong    = ""
	linkGetExample = ""
)

type CmdLinkGet struct {
	client   *client.KubeClient
	CobraCmd cobra.Command
}

func NewCmdLinkGet() *CmdLinkGet {

	skupperCmd := CmdLinkGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "",
		Long:    linkGetLong,
		Example: linkGetExample,
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

func (cmd *CmdLinkGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdLinkGet) AddFlags()                                            {}
func (cmd *CmdLinkGet) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdLinkGet) InputToOptions()                                      {}
func (cmd *CmdLinkGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdLinkGet) WaitUntil() error                                     { return nil }
