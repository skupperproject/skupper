package v0_8_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/update/shared"
	"github.com/skupperproject/skupper/pkg/utils"
)

type AddConfigSync struct{}

func (m *AddConfigSync) Version() string {
	return "0.8.0"
}

func (m *AddConfigSync) Info() string {
	return "Adds the config-sync container to the router component"
}

func (m *AddConfigSync) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "0.8.0")
}

func (m *AddConfigSync) Priority() update.Priority {
	return update.PriorityCommon
}

func (m *AddConfigSync) Platforms() []types.Platform {
	return []types.Platform{types.PlatformKubernetes}
}

func (m *AddConfigSync) Run() update.Result {
	shared.RestartController = true
	shared.RestartRouter = true
	return update.Result{}
}
