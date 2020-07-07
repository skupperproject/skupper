package kube

import (
	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"os"
)

// TODO - remove constants, get from spec
func ContainerForController(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image: ds.Image,
		Name:  types.ControllerContainerName,
		Env:   ds.EnvVar,
	}
	return container
}

func ContainerForTransport(ds types.DeploymentSpec) corev1.Container {
	container := corev1.Container{
		Image: ds.Image,
		Name:  types.TransportContainerName,
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
	return container
}

func ContainerForBridgeServer() corev1.Container {
	var imageName string
	if os.Getenv("SKUPPER_BRIDGE_SERVER_IMAGE") != "" {
		imageName = os.Getenv("SKUPPER_BRIDGE_SERVER_IMAGE")
	} else {
		imageName = types.DefaultBridgeServerImage
	}
	container := corev1.Container{
		Image: imageName,
		Name:  types.BridgeServerContainerName,
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "CONF_FILE",
				Value: "/etc/bridge-server/bridges.json",
			},
		},
	}
	return container
}
