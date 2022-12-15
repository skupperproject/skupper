package v1_3_0

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/main/kube"
	"github.com/skupperproject/skupper/pkg/update"
	"github.com/skupperproject/skupper/pkg/utils"
)

type UpdateAddVflowCollectorKube struct {
	Common *kube.KubeTask
}

func (u *UpdateAddVflowCollectorKube) Version() string {
	return "1.3.0"
}

func (u *UpdateAddVflowCollectorKube) Info() string {
	return "Add vflow collector to kubernetes sites"
}

func (m *UpdateAddVflowCollectorKube) AppliesTo(siteVersion string) bool {
	return utils.LessRecentThanVersion(siteVersion, "1.3.0")
}

func (u *UpdateAddVflowCollectorKube) Priority() update.Priority {
	return update.PriorityHigh
}

func (u *UpdateAddVflowCollectorKube) Platforms() []types.Platform {
	return []types.Platform{types.PlatformKubernetes}
}

func (u *UpdateAddVflowCollectorKube) Run() update.Result {
	u.Common.RestartController = true
	return update.Result{}
}
