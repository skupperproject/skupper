package non_kube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

type CmdTokenRedeem struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandTokenRedeemFlags
	Namespace string
	siteName  string
}

func NewCmdTokenRedeem() *CmdTokenRedeem {
	return &CmdTokenRedeem{}
}

func (cmd *CmdTokenRedeem) NewClient(cobraCommand *cobra.Command, args []string) {
	//TODO
}

func (cmd *CmdTokenRedeem) ValidateInput(args []string) []error { return nil }
func (cmd *CmdTokenRedeem) InputToOptions()                     {}
func (cmd *CmdTokenRedeem) Run() error {
	return fmt.Errorf("command not supported by the selected platform")
}
func (cmd *CmdTokenRedeem) WaitUntil() error { return nil }
