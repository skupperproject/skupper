package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdLinkStatus struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandLinkStatusFlags
	Namespace string
	siteName  string
}

func NewCmdLinkStatus() *CmdLinkStatus {
	return &CmdLinkStatus{}
}

func (cmd *CmdLinkStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdLinkStatus) ValidateInput(args []string) []error { return nil }
func (cmd *CmdLinkStatus) InputToOptions()                     {}
func (cmd *CmdLinkStatus) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdLinkStatus) WaitUntil() error { return nil }
