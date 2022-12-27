package k8s

import (
	"fmt"

	"github.com/skupperproject/skupper/test/utils"
	v1 "k8s.io/api/apps/v1"
	v12 "k8s.io/api/core/v1"
	v13 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SecretMount struct {
	Name      string
	Secret    string
	MountPath string
}

type DeploymentOpts struct {
	Image         string
	Labels        map[string]string
	RestartPolicy v12.RestartPolicy
	Command       []string
	Args          []string
	EnvVars       []v12.EnvVar
	ResourceReq   v12.ResourceRequirements
	SecretMounts  []SecretMount
}

func NewDeployment(name, namespace string, opts DeploymentOpts) (*v1.Deployment, error) {

	var err error

	// Validating mandatory fields
	if utils.StrEmpty(name) {
		err := fmt.Errorf("deployment name is required")
		return nil, err
	}
	if utils.StrEmpty(opts.Image) {
		err := fmt.Errorf("image is required")
		return nil, err
	}

	var volumeMounts []v12.VolumeMount
	var volumes []v12.Volume

	for _, v := range opts.SecretMounts {
		volumeMounts = append(volumeMounts, v12.VolumeMount{
			Name:      v.Name,
			MountPath: v.MountPath,
		})
		volumes = append(volumes, v12.Volume{
			Name: v.Name,
			VolumeSource: v12.VolumeSource{
				Secret: &v12.SecretVolumeSource{
					SecretName: v.Secret,
				},
			},
		})
	}

	// Container to use
	containers := []v12.Container{
		{
			Name:            name,
			Image:           opts.Image,
			ImagePullPolicy: v12.PullAlways,
			Env:             opts.EnvVars,
			Resources:       opts.ResourceReq,
			VolumeMounts:    volumeMounts,
		},
	}
	// Customize commands and arguments if any informed
	if len(opts.Command) > 0 {
		containers[0].Command = opts.Command
	}
	if len(opts.Args) > 0 {
		containers[0].Args = opts.Args
	}

	d := &v1.Deployment{
		ObjectMeta: v13.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    opts.Labels,
		},
		Spec: v1.DeploymentSpec{
			Selector: &v13.LabelSelector{
				MatchLabels: opts.Labels,
			},
			Template: v12.PodTemplateSpec{
				ObjectMeta: v13.ObjectMeta{
					Labels: opts.Labels,
				},
				Spec: v12.PodSpec{
					Volumes:       volumes,
					Containers:    containers,
					RestartPolicy: opts.RestartPolicy,
				},
			},
		},
	}

	return d, err
}
