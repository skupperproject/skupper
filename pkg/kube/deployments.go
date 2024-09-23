package kube

import (
	"context"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetDeploymentOwnerReference(dep *appsv1.Deployment) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       dep.ObjectMeta.Name,
		UID:        dep.ObjectMeta.UID,
	}
}

func DeleteDeployment(name string, namespace string, cli kubernetes.Interface) error {
	_, err := cli.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err == nil {
		err = cli.AppsV1().Deployments(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	}
	return err
}

// TODO, pass full client object with namespace and clientset
func GetDeployment(name string, namespace string, cli kubernetes.Interface) (*appsv1.Deployment, error) {
	existing, err := cli.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	} else {
		return existing, err
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
		dep, err = cli.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		ss, err = cli.AppsV1().StatefulSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		dep, err = cli.AppsV1().Deployments(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		dep, err = cli.AppsV1().DaemonSets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return dep.Status.NumberReady > 0, nil
	})

	return dep, err
}
