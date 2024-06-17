package kube

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/internal/cmd/skupper/utils"
	"github.com/spf13/cobra"
)

type CmdSiteDelete struct {
	client   *client.VanClient
	CobraCmd cobra.Command
}

func NewCmdSiteDelete() *CmdSiteDelete {

	skupperCmd := CmdSiteDelete{}

	cmd := cobra.Command{
		Use:     "delete",
		Short:   "delete the site",
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

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {}
func (cmd *CmdSiteDelete) AddFlags()                                            {}
func (cmd *CmdSiteDelete) ValidateInput(args []string) []error                  { return nil }
func (cmd *CmdSiteDelete) InputToOptions()                                      {}
func (cmd *CmdSiteDelete) Run() error                                           { fmt.Println("Not implemented yet."); return nil }
func (cmd *CmdSiteDelete) WaitUntilReady() error                                { return nil }
