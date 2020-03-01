package kube

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/ajssmith/skupper/api/types"
)

func GetOwnerReference(dep *appsv1.Deployment) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       dep.ObjectMeta.Name,
		UID:        dep.ObjectMeta.UID,
	}
}

// todo, pass full client object with namespace and clientset
func GetDeployment(name string, namespace string, cli *kubernetes.Clientset) (*appsv1.Deployment, error) {
	existing, err := cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return existing, err
	}
}

func NewControllerDeployment(van *types.VanRouterSpec, ownerRef metav1.OwnerReference, cli *kubernetes.Clientset) *appsv1.Deployment {
	deployments := cli.AppsV1().Deployments(van.Namespace)
	existing, err := deployments.Get(types.ControllerDeploymentName, metav1.GetOptions{})
	if err == nil {
		fmt.Println("VAN site controller already exists")
		return existing
	} else if errors.IsNotFound(err) {
		dep := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            types.ControllerDeploymentName,
				Namespace:       van.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &van.Controller.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: van.Controller.Labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: van.Controller.Labels,
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: types.ControllerServiceAccountName,
						Containers:         []corev1.Container{ContainerForController(van.Controller)},
					},
				},
			},
		}

		dep.Spec.Template.Spec.Volumes = van.Controller.Volumes
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = van.Controller.VolumeMounts

		created, err := deployments.Create(dep)
		if err != nil {
			fmt.Println("Failed to create controller deployment: ", err.Error())
			return nil
		} else {
			return created
		}

	} else {
		dep := &appsv1.Deployment{}
		fmt.Println("Failed to check controller deployment: ", err.Error())
		return dep
	}

	return nil
}

func NewQdrDeployment(van *types.VanRouterSpec, cli *kubernetes.Clientset) *appsv1.Deployment {
	deployments := cli.AppsV1().Deployments(van.Namespace)
	existing, err := deployments.Get(types.QdrDeploymentName, metav1.GetOptions{})
	if err == nil {
		fmt.Println("VAN site qdr already exists")
		return existing
	} else if errors.IsNotFound(err) {
		dep := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      types.QdrDeploymentName,
				Namespace: van.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &van.Qdr.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: van.Qdr.Labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      van.Qdr.Labels,
						Annotations: van.Qdr.Annotations,
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: types.QdrServiceAccountName,
						Containers:         []corev1.Container{ContainerForQdr(van.Qdr)},
					},
				},
			},
		}

		dep.Spec.Template.Spec.Volumes = van.Qdr.Volumes
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = van.Qdr.VolumeMounts

		created, err := deployments.Create(dep)
		if err != nil {
			fmt.Println("Failed to create qdr deployment: ", err.Error())
			return nil
		} else {
			return created
		}

	} else {
		dep := &appsv1.Deployment{}
		fmt.Println("Failed to check qdr deployment: ", err.Error())
		return dep
	}

	return nil
}
