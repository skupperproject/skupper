package site

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

type CmdSiteGet struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdSiteGet() *CmdSiteGet {

	skupperCmd := CmdSiteGet{}

	cmd := cobra.Command{
		Use:     "get",
		Short:   "get the site",
		Long:    "",
		Example: "",
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

func (cmd *CmdSiteGet) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdSiteGet) AddFlags()                                            {}
func (cmd *CmdSiteGet) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdSiteGet) InputToOptions()                                      {}
func (cmd *CmdSiteGet) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdSiteGet) WaitUntilReady() error                                { return nil }
