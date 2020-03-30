package kube

import (
	jsonencoding "encoding/json"
	"fmt"
	"os"

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

func DeleteDeployment(name string, namespace string, cli *kubernetes.Clientset) error {
	_, err := cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		err = cli.AppsV1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}

// TODO, pass full client object with namespace and clientset
func GetDeployment(name string, namespace string, cli *kubernetes.Clientset) (*appsv1.Deployment, error) {
	existing, err := cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return existing, err
	}
}

func NewProxyStatefulSet(serviceInterface types.ServiceInterface, namespace string, cli *kubernetes.Clientset) (*appsv1.StatefulSet, error) {
	// Do stateful sets use a different name >>> config.origion ? config.headless.name
	proxyName := serviceInterface.Address + "-proxy"

	statefulSets := cli.AppsV1().StatefulSets(namespace)
	deployments := cli.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	serviceInterface.Origin = ""

	encoded, err := jsonencoding.Marshal(serviceInterface)
	if err != nil {
		return nil, err
	}

	ownerRef := GetOwnerReference(transportDep)

	var imageName string
	if os.Getenv("PROXY_IMAGE") != "" {
		imageName = os.Getenv("PROXY_IMAGE")
	} else {
		imageName = types.DefaultProxyImage
	}

	// TODO: Fix replicas

	proxyStatefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            proxyName,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations: map[string]string{
				types.ServiceQualifier: serviceInterface.Address,
			},
			Labels: map[string]string{
				"internal.skupper.io/type": "proxy",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: serviceInterface.Address,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"internal.skupper.io/service": serviceInterface.Address,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"internal.skupper.io/service": serviceInterface.Address,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: types.TransportServiceAccountName,
					Containers: []corev1.Container{
						{
							Image: imageName,
							Name:  "proxy",
							Env: []corev1.EnvVar{
								{
									Name:  "SKUPPER_PROXY_CONFIG",
									Value: string(encoded),
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "connect",
									MountPath: "/etc/messaging/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "connect",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "skupper",
								},
							},
						},
					},
				},
			},
		},
	}

	created, err := statefulSets.Create(proxyStatefulSet)

	if err != nil {
		return nil, err
	} else {
		return created, nil
	}

}

func NewProxyDeployment(serviceInterface types.ServiceInterface, namespace string, cli *kubernetes.Clientset) (*appsv1.Deployment, error) {
	proxyName := serviceInterface.Address + "-proxy"

	deployments := cli.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	serviceInterface.Origin = ""

	encoded, err := jsonencoding.Marshal(serviceInterface)
	if err != nil {
		return nil, err
	}

	ownerRef := GetOwnerReference(transportDep)

	var imageName string
	if os.Getenv("PROXY_IMAGE") != "" {
		imageName = os.Getenv("PROXY_IMAGE")
	} else {
		imageName = types.DefaultProxyImage
	}

	proxyDep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            proxyName,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Annotations: map[string]string{
				types.ServiceQualifier: serviceInterface.Address,
			},
			Labels: map[string]string{
				"internal.skupper.io/type": "proxy",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"internal.skupper.io/service": serviceInterface.Address,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"internal.skupper.io/service": serviceInterface.Address,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: types.TransportServiceAccountName,
					Containers: []corev1.Container{
						{
							Image: imageName,
							Name:  "proxy",
							Env: []corev1.EnvVar{
								{
									Name:  "SKUPPER_PROXY_CONFIG",
									Value: string(encoded),
								},
								{
									Name: "NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "connect",
									MountPath: "/etc/messaging/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "connect",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "skupper",
								},
							},
						},
					},
				},
			},
		},
	}

	created, err := deployments.Create(proxyDep)

	if err != nil {
		return nil, err
	} else {
		return created, nil
	}

}

func NewControllerDeployment(van *types.VanRouterSpec, ownerRef metav1.OwnerReference, cli *kubernetes.Clientset) (*appsv1.Deployment, error) {
	deployments := cli.AppsV1().Deployments(van.Namespace)
	existing, err := deployments.Get(types.ControllerDeploymentName, metav1.GetOptions{})
	if err == nil {
		return existing, nil
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
			return nil, fmt.Errorf("Failed to create controller deployment: %w", err)
		} else {
			return created, nil
		}

	} else {
		dep := &appsv1.Deployment{}
		return dep, fmt.Errorf("Failed to check controller deployment: %w", err)
	}
}

func NewTransportDeployment(van *types.VanRouterSpec, cli *kubernetes.Clientset) (*appsv1.Deployment, error) {
	deployments := cli.AppsV1().Deployments(van.Namespace)
	existing, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err == nil {
		return existing, nil
	} else if errors.IsNotFound(err) {
		dep := &appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      types.TransportDeploymentName,
				Namespace: van.Namespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &van.Transport.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: van.Transport.Labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      van.Transport.Labels,
						Annotations: van.Transport.Annotations,
					},
					Spec: corev1.PodSpec{
						ServiceAccountName: types.TransportServiceAccountName,
						Containers:         []corev1.Container{ContainerForTransport(van.Transport)},
					},
				},
			},
		}

		dep.Spec.Template.Spec.Volumes = van.Transport.Volumes
		dep.Spec.Template.Spec.Containers[0].VolumeMounts = van.Transport.VolumeMounts

		created, err := deployments.Create(dep)
		if err != nil {
			return nil, fmt.Errorf("Failed to create transport deployment: %w", err)
		} else {
			return created, nil
		}

	} else {
		dep := &appsv1.Deployment{}
		return dep, fmt.Errorf("Failed to check transport deployment: %w", err)
	}
}
