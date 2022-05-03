package constants

import (
	"time"

	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	// until this issue: https://github.com/skupperproject/skupper/issues/163
	// is fixed, this is the best we can do
	SkupperServiceReadyPeriod              time.Duration = 10 * time.Minute
	DefaultTick                                          = 5 * time.Second
	ImagePullingAndResourceCreationTimeout               = 10 * time.Minute
	TestSuiteTimeout                                     = 20 * time.Minute
	NamespaceDeleteTimeout                               = 2 * time.Minute
)

var (
	DefaultRetry wait.Backoff = wait.Backoff{
		Steps:    int(ImagePullingAndResourceCreationTimeout / DefaultTick),
		Duration: DefaultTick,
	}
)

func DefaultRouterOptions(spec *types.RouterOptions) types.RouterOptions {
	if spec == nil {
		spec = &types.RouterOptions{}
	}

	spec.DebugMode = "gdb"
	if spec.Logging == nil {
		spec.Logging = []types.RouterLogConfig{}
	}
	spec.Logging = append(spec.Logging, types.RouterLogConfig{Module: "DEFAULT", Level: "trace+"})

	return *spec
}
