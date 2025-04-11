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

type CmdSystemDelete struct {
	Client       skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient   kubernetes.Interface
	CobraCmd     *cobra.Command
	Namespace    string
	Flags        *common.CommandSystemDeleteFlags
	SystemDelete func(string) error
	file         string
}

func NewCmdSystemDelete() *CmdSystemDelete {

	skupperCmd := CmdSystemDelete{}

	return &skupperCmd
}

func (cmd *CmdSystemDelete) NewClient(cobraCommand *cobra.Command, args []string) {
	cmd.SystemDelete = bootstrap.Delete
}

func (cmd *CmdSystemDelete) ValidateInput(args []string) error {
	var validationErrors []error

	if len(args) > 0 {
		validationErrors = append(validationErrors, fmt.Errorf("this command does not accept arguments"))
	}

	//TODO: CHECK IF THE FILE EXISTS

	return errors.Join(validationErrors...)
}

func (cmd *CmdSystemDelete) InputToOptions() {
	cmd.file = cmd.Flags.Filename
}

func (cmd *CmdSystemDelete) Run() error {
	err := cmd.SystemDelete(cmd.file)

	if err != nil {
		return fmt.Errorf("failed parsing the custom resources: %s", err)
	}

	fmt.Println("Custom resources deleted successfully. You can now run `skupper reload` to make effective the changes.")

	return nil
}

func (cmd *CmdSystemDelete) WaitUntil() error { return nil }
