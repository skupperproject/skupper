package non_kube

import (
	"fmt"
	"github.com/spf13/cobra"
)

type CmdSiteStatus struct {
	CobraCmd  *cobra.Command
	Namespace string
}

func NewCmdSiteStatus() *CmdSiteStatus {
	return &CmdSiteStatus{}
}

func (cmd *CmdSiteStatus) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdSiteStatus) AddFlags()                           {}
func (cmd *CmdSiteStatus) ValidateInput(args []string) []error { return nil }
func (cmd *CmdSiteStatus) InputToOptions()                     {}
func (cmd *CmdSiteStatus) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdSiteStatus) WaitUntil() error { return nil }
