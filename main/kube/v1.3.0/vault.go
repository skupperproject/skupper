package v1_3_0

import (
	"fmt"

	"github.com/skupperproject/skupper/main/kube"
	"github.com/skupperproject/skupper/pkg/update"
	v1_3_0 "github.com/skupperproject/skupper/pkg/update/v1.3.0"
)

type AddVaultKube struct {
	Common *kube.KubeTask
	v1_3_0.AddVault
}

func (u *AddVaultKube) Run() update.Result {
	fmt.Println("  -> Updating site support for vault")
	fmt.Println("     -> Saving vault credentials as a kubernetes secret")
	return update.Result{}
}
