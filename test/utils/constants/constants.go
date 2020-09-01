package constants

import (
	"k8s.io/apimachinery/pkg/util/wait"
	"time"
)

const (
	//until this issue: https://github.com/skupperproject/skupper/issues/163
	//is fixed, this is the best we can do
	SkupperServiceReadyPeriod              time.Duration = time.Minute
	DefaultTick                                          = time.Second * 5
	TestJobBackOffLimit                                  = 3
	ImagePullingAndResourceCreationTimeout               = 10 * time.Minute
	NamespaceDeleteTimeout                               = 1 * time.Minute
)

var (
	DefaultRetry wait.Backoff = wait.Backoff{
		Steps:    int(ImagePullingAndResourceCreationTimeout / DefaultTick),
		Duration: DefaultTick,
	}
)
