package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func GetDeploymentLabel(name string, key string, namespace string, cli kubernetes.Interface) string {
	deployment, err := GetDeployment(name, namespace, cli)
	if err != nil {
		return ""
	}
	if value, ok := deployment.Spec.Template.Labels[key]; ok {
		return value
	}
	return ""
}

func GetPods(selector string, namespace string, cli kubernetes.Interface) ([]corev1.Pod, error) {
	options := metav1.ListOptions{LabelSelector: selector}
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

func CheckProxyStatefulSet(image types.ImageDetails, desired types.ServiceInterface, actual *appsv1.StatefulSet, desiredConfig string, namespace string, cli kubernetes.Interface) (*appsv1.StatefulSet, error) {
	if actual == nil {
		var err error
		actual, err = cli.AppsV1().StatefulSets(namespace).Get(getProxyStatefulSetName(desired), metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return NewProxyStatefulSet(image, desired, desiredConfig, namespace, cli)
		} else if err != nil {
			return nil, err
		}
	}
	change := false
	if desired.Headless.Size != int(*actual.Spec.Replicas) {
		change = true
		*actual.Spec.Replicas = int32(desired.Headless.Size)
	}
	actualConfig := FindEnvVar(actual.Spec.Template.Spec.Containers[0].Env, "QDROUTERD_CONF")
	if actualConfig == nil || actualConfig.Value != desiredConfig {
		SetEnvVarForStatefulSet(actual, "QDROUTERD_CONF", desiredConfig)
		change = true
	}
	if change {
		return cli.AppsV1().StatefulSets(namespace).Update(actual)
	} else {
		return actual, nil
	}
}

func NewProxyStatefulSet(image types.ImageDetails, serviceInterface types.ServiceInterface, config string, namespace string, cli kubernetes.Interface) (*appsv1.StatefulSet, error) {
	statefulSets := cli.AppsV1().StatefulSets(namespace)
	deployments := cli.AppsV1().Deployments(namespace)
	transportDep, err := deployments.Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	ownerRef := GetDeploymentOwnerReference(transportDep)

	replicas := int32(serviceInterface.Headless.Size)
	labels := map[string]string{
		"internal.skupper.io/type": "proxy",
	}
	if len(serviceInterface.Labels) > 0 {
		for k, v := range serviceInterface.Labels {
			labels[k] = v
		}
	}

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
			Labels: labels,
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
							Image:           image.Name,
							ImagePullPolicy: GetPullPolicy(image.PullPolicy),
							Name:            "proxy",
							Env: []corev1.EnvVar{
								{
									Name:  "QDROUTERD_CONF",
									Value: config,
								},
								{
									Name:  "QDROUTERD_CONF_TYPE",
									Value: "json",
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
									Name:      "uplink",
									MountPath: "/etc/qpid-dispatch-certs/skupper-internal/",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "uplink",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: types.SiteServerSecret,
								},
							},
						},
					},
				},
			},
		},
	}

	podspec := &proxyStatefulSet.Spec.Template.Spec
	if len(serviceInterface.Headless.NodeSelector) > 0 {
		podspec.NodeSelector = serviceInterface.Headless.NodeSelector
	}

	if len(serviceInterface.Headless.Affinity) > 0 || len(serviceInterface.Headless.AntiAffinity) > 0 {
		podspec.Affinity = &corev1.Affinity{}
		if len(serviceInterface.Headless.Affinity) > 0 {
			podspec.Affinity.PodAffinity = &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: serviceInterface.Headless.Affinity,
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
		}
		if len(serviceInterface.Headless.AntiAffinity) > 0 {
			podspec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: serviceInterface.Headless.AntiAffinity,
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
		}
	}
	setResourceRequests(&podspec.Containers[0], serviceInterface.Headless)

	created, err := statefulSets.Create(proxyStatefulSet)

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
					MatchLabels: van.Controller.LabelSelector,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels:      van.Controller.Labels,
						Annotations: van.Controller.Annotations,
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

		setAffinity(&van.Controller, &dep.Spec.Template.Spec)
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

func setAffinity(spec *types.DeploymentSpec, podspec *corev1.PodSpec) {
	if len(spec.NodeSelector) > 0 {
		podspec.NodeSelector = spec.NodeSelector
	}

	if len(spec.Affinity) > 0 || len(spec.AntiAffinity) > 0 {
		podspec.Affinity = &corev1.Affinity{}
		if len(spec.Affinity) > 0 {
			podspec.Affinity.PodAffinity = &corev1.PodAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: spec.Affinity,
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
		}
		if len(spec.AntiAffinity) > 0 {
			podspec.Affinity.PodAntiAffinity = &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
					{
						Weight: 100,
						PodAffinityTerm: corev1.PodAffinityTerm{
							LabelSelector: &metav1.LabelSelector{
								MatchLabels: spec.AntiAffinity,
							},
							TopologyKey: "kubernetes.io/hostname",
						},
					},
				},
			}
		}
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
					MatchLabels: van.Transport.LabelSelector,
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
						},
					},
				},
			},
		}

		setAffinity(&van.Transport, &dep.Spec.Template.Spec)
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

func GetContainerPort(deployment *appsv1.Deployment) map[int]int {
	if len(deployment.Spec.Template.Spec.Containers) > 0 && len(deployment.Spec.Template.Spec.Containers[0].Ports) > 0 {
		return GetAllContainerPorts(deployment.Spec.Template.Spec.Containers[0])
	} else {
		return map[int]int{}
	}
}

func GetContainerPortForStatefulSet(statefulSet *appsv1.StatefulSet) map[int]int {
	if len(statefulSet.Spec.Template.Spec.Containers) > 0 && len(statefulSet.Spec.Template.Spec.Containers[0].Ports) > 0 {
		return GetAllContainerPorts(statefulSet.Spec.Template.Spec.Containers[0])
	} else {
		return map[int]int{}
	}
}

func GetContainerPortForDaemonSet(daemonSet *appsv1.DaemonSet) map[int]int {
	if len(daemonSet.Spec.Template.Spec.Containers) > 0 && len(daemonSet.Spec.Template.Spec.Containers[0].Ports) > 0 {
		return GetAllContainerPorts(daemonSet.Spec.Template.Spec.Containers[0])
	} else {
		return map[int]int{}
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

func WaitStatefulSetReadyReplicas(name string, namespace string, readyReplicas int, cli kubernetes.Interface, timeout, interval time.Duration) (*appsv1.StatefulSet, error) {
	var ss *appsv1.StatefulSet
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		ss, err = cli.AppsV1().StatefulSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			// ss does not exist yet
			return false, nil
		}
		return ss.Status.ReadyReplicas == int32(readyReplicas), nil
	})

	return ss, err
}

// WaitDeploymentReady waits till given deployment contains at least one ReadyReplicas, or until it times out
func WaitDeploymentReady(name string, namespace string, cli kubernetes.Interface, timeout, interval time.Duration) (*appsv1.Deployment, error) {
	var dep *appsv1.Deployment
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		dep, err = cli.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			// dep does not exist yet
			return false, nil
		} else if err != nil {
			return false, err
		}
		return dep.Status.ReadyReplicas > 0, nil
	})

	return dep, err
}

func WaitDaemonSetReady(name string, namespace string, cli kubernetes.Interface, timeout, interval time.Duration) (*appsv1.DaemonSet, error) {
	var dep *appsv1.DaemonSet
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		dep, err = cli.AppsV1().DaemonSets(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return dep.Status.NumberReady > 0, nil
	})

	return dep, err
}
