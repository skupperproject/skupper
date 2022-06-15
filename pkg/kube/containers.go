package kube

import (
	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func setResourceRequests(container *corev1.Container, resources types.Resources) {
	requests := corev1.ResourceList{}
	if resources.HasCpuRequest() {
		requests[corev1.ResourceCPU] = resources.GetCpuRequest()
	}
	if resources.HasMemoryRequest() {
		requests[corev1.ResourceMemory] = resources.GetMemoryRequest()
	}
	limits := corev1.ResourceList{}
	if resources.HasCpuLimit() {
		limits[corev1.ResourceCPU] = resources.GetCpuLimit()
	}
	if resources.HasMemoryLimit() {
		limits[corev1.ResourceMemory] = resources.GetMemoryLimit()
	}
	if len(requests) > 0 || len(limits) > 0 {
		container.Resources = corev1.ResourceRequirements{
			Requests: requests,
			Limits:   limits,
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

func ContainerForConfigSync(ds types.DeploymentSpec) *corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.ConfigSyncContainerName,
	}

	setResourceRequests(&container, &ds)
	return &container
}
