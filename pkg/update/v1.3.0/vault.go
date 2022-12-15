package v1_3_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

// AddVault is an abstract implementation
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
