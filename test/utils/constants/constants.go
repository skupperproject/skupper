package constants

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

const (
	//until this issue: https://github.com/skupperproject/skupper/issues/163
	//is fixed, this is the best we can do
	SkupperServiceReadyPeriod              time.Duration = 10 * time.Minute
	DefaultTick                                          = 5 * time.Second
	ImagePullingAndResourceCreationTimeout               = 10 * time.Minute
	NamespaceDeleteTimeout                               = 2 * time.Minute
)

var (
	DefaultRetry wait.Backoff = wait.Backoff{
		Steps:    int(ImagePullingAndResourceCreationTimeout / DefaultTick),
		Duration: DefaultTick,
	}
)
