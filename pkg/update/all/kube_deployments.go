package all

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/update/shared"
	"github.com/skupperproject/skupper/pkg/utils"
)

type UpdateDeployments struct{}

func (u *UpdateDeployments) Version() string {
	return "*"
}

func (u *UpdateDeployments) Info() string {
	return "Updates skupper deployments to current version"
}

func (u *UpdateDeployments) AppliesTo(siteVersion string) bool {
	// THIS IS THE CORRECT return utils.LessRecentThanVersion(siteVersion, version.Version)

	// Simulating CLI version to be 1.3.0
	return utils.LessRecentThanVersion(siteVersion, "1.3.0")
}

func (u *UpdateDeployments) Priority() update.Priority {
	return update.PriotityLow
}

func (u *UpdateDeployments) Platforms() []types.Platform {
	return []types.Platform{types.PlatformKubernetes}
}

func (u *UpdateDeployments) Run() update.Result {
	if shared.RestartController {
		fmt.Println("Restarting controller")
	}
	if shared.RestartRouter {
		fmt.Println("Restarting router")
	}
	return update.Result{}
}
