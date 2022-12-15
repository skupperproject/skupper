package v0_7_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/main/kube"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type Claims struct {
	Common *kube.KubeTask
}

func (m *Claims) Version() string {
	return "0.7.0"
}

func (m *Claims) Info() string {
	return "Adds claims support"
}

func (m *Claims) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "0.7.0")
}

func (m *Claims) Priority() update.Priority {
	return update.PriorityHigh
}

func (m *Claims) Platforms() []types.Platform {
	return []types.Platform{types.PlatformKubernetes}
}

func (m *Claims) Run() update.Result {
	m.Common.RestartController = true
	m.Common.RestartRouter = true
	return update.Result{}
}
