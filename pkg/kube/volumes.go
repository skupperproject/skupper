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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

func AppendSecretVolumeWithVolumeName(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, secretName string, volumeName string, path string) {
	*volumes = append(*volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
			},
		},
	})
	*mounts = append(*mounts, corev1.VolumeMount{
		Name:      volumeName,
		MountPath: path,
	})
}

func AppendSecretVolume(volumes *[]corev1.Volume, mounts *[]corev1.VolumeMount, name string, path string) {
	AppendSecretVolumeWithVolumeName(volumes, mounts, name, name, path)
}

func AppendSharedSecretVolume(volumes *[]corev1.Volume, mounts []*[]corev1.VolumeMount, name string, path string) {
	*volumes = append(*volumes, corev1.Volume{
		Name: name,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: name,
			},
		},
	})
	for _, mount := range mounts {
		*mount = append(*mount, corev1.VolumeMount{
			Name:      name,
			MountPath: path,
		})
	}
}

func AppendSharedVolume(volumes *[]corev1.Volume, mounts []*[]corev1.VolumeMount, volumename string, path string) {

	*volumes = append(*volumes, corev1.Volume{
		Name: volumename,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})

	for _, mount := range mounts {
		*mount = append(*mount, corev1.VolumeMount{
			Name:      volumename,
			MountPath: path,
			ReadOnly:  false,
		})
	}
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
