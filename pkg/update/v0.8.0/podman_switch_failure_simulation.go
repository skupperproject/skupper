package v0_8_0

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type PodmanSwitch struct{}

func (p *PodmanSwitch) Info() string {
	return "Simulates a blocking failure while adding support for switch command"
}

func (p *PodmanSwitch) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "0.8.0")
}

func (p *PodmanSwitch) Version() string {
	return "0.8.0"
}

func (p *PodmanSwitch) Priority() update.Priority {
	return update.PriorityHigh
}

func (p *PodmanSwitch) Platforms() []types.Platform {
	return []types.Platform{types.PlatformPodman}
}

func (p *PodmanSwitch) Run() update.Result {
	return update.Result{
		Err:  fmt.Errorf("simulated error for podman switch update to 0.8.0"),
		Stop: true,
	}
}
