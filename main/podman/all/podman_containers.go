package all

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/main/podman"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type UpdatePodmanContainers struct {
	Common *podman.PodmanTask
}

func (u *UpdatePodmanContainers) Version() string {
	return "*"
}

func (u *UpdatePodmanContainers) Info() string {
	return "Updates skupper podman containers to current version"
}

func (u *UpdatePodmanContainers) AppliesTo(siteVersion string) bool {
	// THIS IS THE CORRECT return utils.LessRecentThanVersion(siteVersion, version.Version)

	// Simulating CLI version to be 1.3.0
	return utils.LessRecentThanVersion(siteVersion, "1.3.0")
}

func (u *UpdatePodmanContainers) Priority() update.Priority {
	return update.PriotityLow
}

func (u *UpdatePodmanContainers) Platforms() []types.Platform {
	return []types.Platform{types.PlatformPodman}
}

func (u *UpdatePodmanContainers) Run() update.Result {
	if u.Common.RestartController {
		fmt.Println("Restarting controller")
	}
	if u.Common.RestartRouter {
		fmt.Println("Restarting router")
	}
	return update.Result{}
}
