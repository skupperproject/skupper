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

func controllerStartupProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 1,
		PeriodSeconds:       1,
		FailureThreshold:    60,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(8182),
				Path: "/healthz",
			},
		},
	}
}

func controllerLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 60,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(8182),
				Path: "/healthz",
			},
		},
	}
}

func controllerReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 5,
		PeriodSeconds:       5,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(8182),
				Path: "/healthz",
			},
		},
	}
}

func CheckProbesForControllerContainer(c *corev1.Container) bool {
	updated := false
	if c.StartupProbe == nil {
		c.StartupProbe = controllerStartupProbe()
		updated = true
	}
	if c.LivenessProbe == nil {
		c.LivenessProbe = controllerLivenessProbe()
		updated = true
	}
	if c.ReadinessProbe == nil {
		c.ReadinessProbe = controllerReadinessProbe()
		updated = true
	}
	return updated
}

func ContainerForController(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.ControllerContainerName,
		Env:             ds.EnvVar,
		Ports:           ds.Ports,
		StartupProbe:    controllerStartupProbe(),
		LivenessProbe:   controllerLivenessProbe(),
		ReadinessProbe:  controllerReadinessProbe(),
	}
	setResourceRequests(&container, &ds)
	return container
}

func transportLivenessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 60,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(int(types.TransportLivenessPort)),
				Path: "/healthz",
			},
		},
	}
}

func transportReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(int(types.TransportLivenessPort)),
				Path: "/healthz",
			},
		},
	}
}

func CheckProbesForTransportContainer(c *corev1.Container) bool {
	updated := false
	if c.LivenessProbe == nil {
		c.LivenessProbe = transportLivenessProbe()
		updated = true
	}
	if c.ReadinessProbe == nil {
		c.ReadinessProbe = transportReadinessProbe()
		updated = true
	}
	return updated
}

func ContainerForTransport(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.TransportContainerName,
		LivenessProbe:   transportLivenessProbe(),
		ReadinessProbe:  transportReadinessProbe(),
		Env:             ds.EnvVar,
		Ports:           ds.Ports,
	}
	setResourceRequests(&container, &ds)
	return container
}

func configSyncReadinessProbe() *corev1.Probe {
	return &corev1.Probe{
		InitialDelaySeconds: 1,
		Handler: corev1.Handler{
			HTTPGet: &corev1.HTTPGetAction{
				Port: intstr.FromInt(9191),
				Path: "/healthz",
			},
		},
	}
}

func CheckProbesForConfigSync(c *corev1.Container) bool {
	if c.ReadinessProbe == nil {
		c.ReadinessProbe = configSyncReadinessProbe()
		return true
	}
	return false
}

func ContainerForConfigSync(ds types.DeploymentSpec) *corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.ConfigSyncContainerName,
		ReadinessProbe:  configSyncReadinessProbe(),
	}

	setResourceRequests(&container, &ds)
	return &container
}

func ContainerForFlowCollector(ds types.DeploymentSpec) *corev1.Container {
	container := corev1.Container{
		Image:           ds.Image.Name,
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
		Name:            types.FlowCollectorContainerName,
		Env:             ds.EnvVar,
	}
	setResourceRequests(&container, &ds)
	return &container
}

func ContainerForPrometheusServer(ds types.DeploymentSpec) corev1.Container {
	// --web.config.file=/path_to/web-config.yaml for tls server config and basic user auth

	container := corev1.Container{
		Name:            types.PrometheusContainerName,
		Image:           ds.Image.Name,
		Args:            []string{"--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.path=/prometheus/", "--web.config.file=/etc/prometheus/web-config.yml"},
		Env:             ds.EnvVar,
		VolumeMounts:    []corev1.VolumeMount{},
		ImagePullPolicy: GetPullPolicy(ds.Image.PullPolicy),
	}
	setResourceRequests(&container, &ds)
	return container
}
