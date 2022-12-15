package podman

import "github.com/skupperproject/skupper/client/podman"

type PodmanTask struct {
	Cli               *podman.PodmanRestClient
	RestartRouter     bool
	RestartController bool
}
