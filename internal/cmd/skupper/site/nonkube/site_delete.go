package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"

	"github.com/spf13/cobra"
)

type CmdSiteDelete struct {
	CobraCmd  *cobra.Command
	Namespace string
	siteName  string
	Flags     *common.CommandSiteDeleteFlags
}

func NewCmdSiteDelete() *CmdSiteDelete {
	return &CmdSiteDelete{}
}

func (cmd *CmdSiteDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdSiteDelete) ValidateInput(args []string) []error { return nil }
func (cmd *CmdSiteDelete) InputToOptions()                     {}
func (cmd *CmdSiteDelete) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdSiteDelete) WaitUntil() error { return nil }
