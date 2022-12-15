package v1_3_0

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type AddVault struct{}

func (u *AddVault) Version() string {
	return "1.3.0"
}
func (u *AddVault) Info() string {
	return "Adds vault support to skupper sites"
}

func (u *AddVault) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "1.3.0")
}

func (u *AddVault) Priority() update.Priority {
	return update.PriotityLow
}

func (u *AddVault) Platforms() []types.Platform {
	return []types.Platform{types.PlatformPodman, types.PlatformKubernetes}
}

func (u *AddVault) Run() update.Result {
	fmt.Println("  -> Updating site support for vault")
	switch config.GetPlatform() {
	case types.PlatformKubernetes:
		fmt.Println("     -> Saving vault credentials as a kubernetes secret")
	case types.PlatformPodman:
		fmt.Println("     -> Saving vault credentials as a podman volume")
	}
	return update.Result{}
}
