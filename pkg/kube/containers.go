package kube

import (
	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func setResourceRequests(container *corev1.Container, ds *types.DeploymentSpec) {
	if ds.CpuRequest != nil || ds.MemoryRequest != nil {
		container.Resources = corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
		}
		if ds.CpuRequest != nil {
			container.Resources.Requests[corev1.ResourceCPU] = *ds.CpuRequest
		}
		if ds.MemoryRequest != nil {
			container.Resources.Requests[corev1.ResourceMemory] = *ds.MemoryRequest
		}
	}
}

func ContainerForController(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.ControllerContainerName,
		Env:             ds.EnvVar,
	}
	setResourceRequests(&container, &ds)
	return container
}

func ContainerForTransport(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.TransportContainerName,
		LivenessProbe: &corev1.Probe{
			InitialDelaySeconds: 60,
			Handler: corev1.Handler{
				HTTPGet: &corev1.HTTPGetAction{
					Port: intstr.FromInt(int(ds.LivenessPort)),
					Path: "/healthz",
				},
			},
		},
		Env:   ds.EnvVar,
		Ports: ds.Ports,
	}
	setResourceRequests(&container, &ds)
	return container
}
