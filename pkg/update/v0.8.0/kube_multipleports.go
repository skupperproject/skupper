package v0_8_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/update/shared"
	"github.com/skupperproject/skupper/pkg/utils"
)

type MultiplePorts struct{}

func (m *MultiplePorts) Version() string {
	return "0.8.0"
}

func (m *MultiplePorts) Info() string {
	return "Convert service definitions to use multiple ports"
}

func (m *MultiplePorts) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "0.8.0")
}

func (m *MultiplePorts) Priority() update.Priority {
	return update.PriorityHigh
}

func (m *MultiplePorts) Platforms() []types.Platform {
	return []types.Platform{types.PlatformKubernetes}
}

func (m *MultiplePorts) Run() update.Result {
	shared.RestartController = true
	shared.RestartRouter = true
	return update.Result{}
}
