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

func UpdateEnvVar(original []corev1.EnvVar, name string, value string) []corev1.EnvVar {
	updated := []corev1.EnvVar{}
	found := false
	for _, v := range original {
		if v.Name == name {
			found = true
			v.Value = value
			updated = append(updated, corev1.EnvVar{Name: v.Name, Value: value})
		} else {
			updated = append(updated, v)
		}
	}
	if !found {
		updated = append(updated, corev1.EnvVar{Name: name, Value: value})
	}
	return updated
}

func SetEnvVarForDeployment(dep *appsv1.Deployment, name string, value string) {
	dep.Spec.Template.Spec.Containers[0].Env = UpdateEnvVar(dep.Spec.Template.Spec.Containers[0].Env, name, value)
}

func SetEnvVarForStatefulSet(dep *appsv1.StatefulSet, name string, value string) {
	dep.Spec.Template.Spec.Containers[0].Env = UpdateEnvVar(dep.Spec.Template.Spec.Containers[0].Env, name, value)
}
