package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdListenerCreate struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandListenerCreateFlags
	Namespace string
	siteName  string
}

func NewCmdListenerCreate() *CmdListenerCreate {
	return &CmdListenerCreate{}
}

func (cmd *CmdListenerCreate) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdListenerCreate) AddFlags()                           {}
func (cmd *CmdListenerCreate) ValidateInput(args []string) []error { return nil }
func (cmd *CmdListenerCreate) InputToOptions()                     {}
func (cmd *CmdListenerCreate) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdListenerCreate) WaitUntil() error { return nil }
