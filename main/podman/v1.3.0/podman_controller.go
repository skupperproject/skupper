package v1_3_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/main/podman"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type PodmanController struct {
	Common *podman.PodmanTask
}

func (u *PodmanController) Version() string {
	return "1.3.0"
}
func (u *PodmanController) Info() string {
	return "Add service controller to podman sites"
}

func (m *PodmanController) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "1.3.0")
}

func (u *PodmanController) Priority() update.Priority {
	return update.PriorityHigh
}

func (u *PodmanController) Platforms() []types.Platform {
	return []types.Platform{types.PlatformPodman}
}

func (u *PodmanController) Run() update.Result {
	u.Common.RestartController = true
	return update.Result{}
}
