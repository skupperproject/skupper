package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdLinkDelete struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandLinkDeleteFlags
	Namespace string
	siteName  string
}

func NewCmdLinkDelete() *CmdLinkDelete {
	return &CmdLinkDelete{}
}

func (cmd *CmdLinkDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdLinkDelete) ValidateInput(args []string) []error { return nil }
func (cmd *CmdLinkDelete) InputToOptions()                     {}
func (cmd *CmdLinkDelete) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdLinkDelete) WaitUntil() error { return nil }
