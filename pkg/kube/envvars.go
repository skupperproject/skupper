package kube

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func FindEnvVar(env []corev1.EnvVar, name string) *corev1.EnvVar {
	for _, v := range env {
		if v.Name == name {
			return &v
		}
	}
	return nil
}

func SetEnvVarForDeployment(dep *appsv1.Deployment, name string, value string) {
	original := dep.Spec.Template.Spec.Containers[0].Env
	updated := []corev1.EnvVar{}
	for _, v := range original {
		if v.Name == name {
			v.Value = value
			updated = append(updated, corev1.EnvVar{Name: v.Name, Value: value})
		} else {
			updated = append(updated, v)
		}
	}
	dep.Spec.Template.Spec.Containers[0].Env = updated
}
