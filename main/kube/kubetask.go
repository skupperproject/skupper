package kube

import (
	"github.com/skupperproject/skupper/client"
	appsv1 "k8s.io/api/apps/v1"
)

type KubeTask struct {
	Cli               *client.VanClient
	RestartRouter     bool
	RestartController bool
	Router            *appsv1.Deployment
	Controller        *appsv1.Deployment
}
