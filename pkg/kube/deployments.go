package kube

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/pkg/utils"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func GetDeploymentPods(name string, namespace string, cli kubernetes.Interface) ([]corev1.Pod, error) {
	deployment, err := GetDeployment(name, namespace, cli)
	if err != nil {
		return nil, err
	}
	options := metav1.ListOptions{LabelSelector: "application=" + deployment.Name}
	podList, err := cli.CoreV1().Pods(namespace).List(options)
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

func GetDeploymentOwnerReference(dep *appsv1.Deployment) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       dep.ObjectMeta.Name,
		UID:        dep.ObjectMeta.UID,
	}
}

func DeleteDeployment(name string, namespace string, cli kubernetes.Interface) error {
	_, err := cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err == nil {
		err = cli.AppsV1().Deployments(namespace).Delete(name, &metav1.DeleteOptions{})
	}
	return err
}

// TODO, pass full client object with namespace and clientset
func GetDeployment(name string, namespace string, cli kubernetes.Interface) (*appsv1.Deployment, error) {
	existing, err := cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return existing, err
	}
}

func getProxyStatefulSetName(definition types.ServiceInterface) string {
	if definition.Origin == "" {
		//in the originating site, the name cannot clash with
		//the statefulset being exposed
		return definition.Address + "-proxy"
	} else {
		//in all other sites, the name must match the
		//statefulset that was exposed in the originating site
		return definition.Headless.Name
	}
}

func CheckProxyStatefulSet(desired types.ServiceInterface, actual *appsv1.StatefulSet, namespace string, cli kubernetes.Interface) (*appsv1.StatefulSet, error) {
	encoded, err := jsonencoding.Marshal(desired)
	if err != nil {
		return nil, err
	}
	if actual == nil {
		actual, err = cli.AppsV1().StatefulSets(namespace).Get(getProxyStatefulSetName(desired), metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return NewProxyStatefulSet(desired, namespace, cli)
		} else if err != nil {
			return nil, err
		}
	}
	change := false
	if desired.Headless.Size != int(*actual.Spec.Replicas) {
		change = true
		*actual.Spec.Replicas = int32(desired.Headless.Size)
	}
	config := FindEnvVar(actual.Spec.Template.Spec.Containers[0].Env, "SKUPPER_PROXY_CONFIG")
	if config == nil || config.Value != string(encoded) {
		SetEnvVarForStatefulSet(actual, "SKUPPER_PROXY_CONFIG", string(encoded))
	}
	if change {
		return cli.AppsV1().StatefulSets(namespace).Update(actual)
	} else {
		return actual, nil
	}
}

func NewProxyStatefulSet(serviceInterface types.ServiceInterface, namespace string, cli kubernetes.Interface) (*appsv1.StatefulSet, error) {
	statefulSets := cli.AppsV1().StatefulSets(namespace)
	deployments := cli.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	encoded, err := jsonencoding.Marshal(serviceInterface)
	if err != nil {
		return nil, err
	}

	ownerRef := GetDeploymentOwnerReference(transportDep)

	var imageName string
	if os.Getenv("PROXY_IMAGE") != "" {
		imageName = os.Getenv("PROXY_IMAGE")
	} else {
		imageName = types.DefaultProxyImage
	}

	replicas := int32(serviceInterface.Headless.Size)
	proxyStatefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            getProxyStatefulSetName(serviceInterface),
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
			Replicas:    &replicas,
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

func NewProxyDeployment(serviceInterface types.ServiceInterface, namespace string, cli kubernetes.Interface) (*appsv1.Deployment, error) {
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

	ownerRef := GetDeploymentOwnerReference(transportDep)

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

func NewControllerDeployment(van *types.RouterSpec, ownerRef *metav1.OwnerReference, cli kubernetes.Interface) (*appsv1.Deployment, error) {
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
				Name:      types.ControllerDeploymentName,
				Namespace: van.Namespace,
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
		if ownerRef != nil {
			dep.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
		}

		for _, sc := range van.Controller.Sidecars {
			dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, *sc)
		}

		dep.Spec.Template.Spec.Volumes = van.Controller.Volumes
		for i, _ := range van.Controller.VolumeMounts {
			dep.Spec.Template.Spec.Containers[i].VolumeMounts = van.Controller.VolumeMounts[i]
		}

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

func NewTransportDeployment(van *types.RouterSpec, ownerRef *metav1.OwnerReference, cli kubernetes.Interface) (*appsv1.Deployment, error) {
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
						Containers: []corev1.Container{
							ContainerForTransport(van.Transport),
							ContainerForBridgeServer(),
						},
					},
				},
			},
		}

		for _, sc := range van.Transport.Sidecars {
			dep.Spec.Template.Spec.Containers = append(dep.Spec.Template.Spec.Containers, *sc)
		}

		if ownerRef != nil {
			dep.ObjectMeta.OwnerReferences = []metav1.OwnerReference{
				*ownerRef,
			}
		}
		dep.Spec.Template.Spec.Volumes = van.Transport.Volumes
		for i, _ := range van.Transport.VolumeMounts {
			dep.Spec.Template.Spec.Containers[i].VolumeMounts = van.Transport.VolumeMounts[i]
		}

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

func GetContainerPort(deployment *appsv1.Deployment) int32 {
	if len(deployment.Spec.Template.Spec.Containers) > 0 && len(deployment.Spec.Template.Spec.Containers[0].Ports) > 0 {
		return deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
	} else {
		return 0
	}
}

// WaitDeploymentReadyReplicas waits till given deployment contains the expected
// number of readyReplicas, or until it times out
func WaitDeploymentReadyReplicas(name string, namespace string, readyReplicas int, cli kubernetes.Interface, timeout, interval time.Duration) (*appsv1.Deployment, error) {
	var dep *appsv1.Deployment
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		dep, err = cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			// dep does not exist yet
			return false, nil
		}
		return dep.Status.ReadyReplicas == int32(readyReplicas), nil
	})

	return dep, err
}
