package v1_3_0

import (
	"fmt"

	"github.com/skupperproject/skupper/main/podman"
	"github.com/skupperproject/skupper/pkg/update"
	v1_3_0 "github.com/skupperproject/skupper/pkg/update/v1.3.0"
)

// AddVault is an abstract implementation
type AddVaultPodman struct {
	Common *podman.PodmanTask
	v1_3_0.AddVault
}

func (a *AddVaultPodman) Run() update.Result {
	fmt.Println("  -> Updating site support for vault")
	fmt.Println("     -> Saving vault credentials as a podman volume")
	return update.Result{}
}
