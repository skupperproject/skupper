package nonkube

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type CmdSystemApply struct {
	Client      skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient  kubernetes.Interface
	CobraCmd    *cobra.Command
	Namespace   string
	Flags       *common.CommandSystemApplyFlags
	SystemApply func(string) error
	file        string
}

func NewCmdSystemApply() *CmdSystemApply {

	skupperCmd := CmdSystemApply{}

	return &skupperCmd
}

func (cmd *CmdSystemApply) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemApply = bootstrap.Apply
}

func (cmd *CmdSystemApply) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	//TODO: CHECK IF THE FILE EXISTS

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemApply) InputToOptions() {
	cmd.file = cmd.Flags.Filename
}

func (cmd *CmdSystemApply) Run() error {
	err := cmd.SystemApply(cmd.file)

	if err != nil {
		return fmt.Errorf("failed parsing the custom resources: %s", err)
	}

	fmt.Println("Custom resources applied successfully. You can now run `skupper reload` to make effective the changes.")

	return nil
}

func (cmd *CmdSystemApply) WaitUntil() error { return nil }
