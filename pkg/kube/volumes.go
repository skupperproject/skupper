/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"github.com/skupperproject/skupper/api/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"time"
)

func AppendConfigVolume(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, volName string, refName string, path string) {
	*volumes = append(*volumes, corev1.Volume{
		Name: volName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: refName,
				},
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      volName,
		MountPath: path,
	})
}

func AppendSecretVolume(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, name string, path string) {
	*volumes = append(*volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      name,
		MountPath: path,
	})
}

func RemoveSecretVolumeForDeployment(name string, dep *appsv1.Deployment, index int) {
	volumes := []corev1.Volume{}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		if v.Name != name {
			volumes = append(volumes, v)
		}
	}
	dep.Spec.Template.Spec.Volumes = volumes

	volumeMounts := []corev1.VolumeMount{}
	for _, vm := range dep.Spec.Template.Spec.Containers[index].VolumeMounts {
		if vm.Name != name {
			volumeMounts = append(volumeMounts, vm)
		}
	}
	dep.Spec.Template.Spec.Containers[index].VolumeMounts = volumeMounts
}

func UpdateSecretVolume(spec *corev1.PodSpec, oldname string, name string) {
	for i, volume := range spec.Volumes {
		if volume.Name == oldname {
			spec.Volumes[i] = corev1.Volume{
				Name: name,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: name,
					},
				},
			}
			break
		}
	}
	for i, _ := range spec.Containers {
		for j, mount := range spec.Containers[i].VolumeMounts {
			if mount.Name == oldname {
				spec.Containers[i].VolumeMounts[j] = corev1.VolumeMount{
					Name:      name,
					MountPath: mount.MountPath,
				}
				break
			}
		}
	}
}

func AppendSecretAndUpdateDeployment(secretName string, path string, deploymentName string, namespace string, cli kubernetes.Interface, waitForRestart bool) error {

	deployment, err := GetDeployment(deploymentName, namespace, cli)

	AppendSecretVolume(&deployment.Spec.Template.Spec.Volumes, &deployment.Spec.Template.Spec.Containers[0].VolumeMounts, secretName, path+secretName+"/")

	_, err = cli.AppsV1().Deployments(namespace).Update(deployment)
	if err != nil {
		return err
	}

	if waitForRestart {
		_, err = WaitDeploymentReady(types.TransportDeploymentName, namespace, cli, time.Second*180, time.Second*5)
		if err != nil {
			return err
		}
	}
	return nil
}

func RemoveSecretAndUpdateDeployment(secretName string, deploymentName string, namespace string, cli kubernetes.Interface) error {

	deployment, err := GetDeployment(deploymentName, namespace, cli)

	RemoveSecretVolumeForDeployment(secretName, deployment, 0)

	_, err = cli.AppsV1().Deployments(namespace).Update(deployment)
	if err != nil {
		return err
	}
	return err
}
