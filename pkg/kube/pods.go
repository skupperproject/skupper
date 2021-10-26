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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/pkg/utils"
)

func GetPullPolicy(policy string) corev1.PullPolicy {
	switch policy {
	case string(corev1.PullAlways):
		return corev1.PullAlways
	case string(corev1.PullNever):
		return corev1.PullNever
	case string(corev1.PullIfNotPresent):
		return corev1.PullIfNotPresent
	default:
		return ""
	}
}

func IsPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func IsPodRunning(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodRunning
}

func FirstReadyPod(list []corev1.Pod) *corev1.Pod {
	for _, p := range list {
		if IsPodReady(&p) {
			return &p
		}
	}
	return nil
}

func GetReadyPod(namespace string, clientset kubernetes.Interface, component string) (*corev1.Pod, error) {
	selector := "skupper.io/component=" + component
	pods, err := clientset.CoreV1().Pods(namespace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	} else if len(pods.Items) == 0 {
		return nil, errors.New("Not found")
	}
	pod := FirstReadyPod(pods.Items)
	if pod == nil {
		return nil, errors.New("Not ready")
	} else {
		return pod, nil
	}
}

func GetImageVersion(pod *corev1.Pod, container string) string {
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == container {
			parts := strings.Split(c.ImageID, "@")
			if len(parts) > 1 && len(parts[1]) >= 19 {
				return fmt.Sprintf("%s (%s)", c.Image, parts[1][:19])
			} else {
				return fmt.Sprintf("%s", c.Image)
			}
		}
	}
	return "not-found"
}

func GetComponentVersion(namespace string, clientset kubernetes.Interface, component string, container string) string {
	pod, err := GetReadyPod(namespace, clientset, component)
	if err == nil {
		return GetImageVersion(pod, container)
	} else {
		return "not-found"
	}
}

func WaitForPodStatus(namespace string, clientset kubernetes.Interface, podName string, status corev1.PodPhase, timeout time.Duration, interval time.Duration) (*corev1.Pod, error) {
	var pod *corev1.Pod
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		pod, err = clientset.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
		if err != nil {
			// pod does not exist yet
			return false, nil
		}
		return pod.Status.Phase == status, nil
	})

	return pod, err
}

func WaitForPodsStatus(namespace string, clientset kubernetes.Interface, selector string, status corev1.PodPhase, timeout time.Duration, interval time.Duration) error {
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		pods, err := GetPods(selector, namespace, clientset)
		if err != nil {
			return false, nil
		}
		if len(pods) == 0 {
			return false, nil
		}
		for _, pod := range pods {
			if pod.Status.Phase != status {
				return false, nil
			}
		}
		return true, nil
	})

	return err
}

func GetPodContainerLogs(podName string, containerName string, namespace string, clientset kubernetes.Interface) (string, error) {
	podLogOpts := corev1.PodLogOptions{}
	return GetPodContainerLogsWithOpts(podName, containerName, namespace, clientset, podLogOpts)
}

func GetPodContainerLogsWithOpts(podName string, containerName string, namespace string, clientset kubernetes.Interface, podLogOpts corev1.PodLogOptions) (string, error) {
	if containerName != "" {
		podLogOpts.Container = containerName
	}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	podLogs, err := req.Stream()
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	str := buf.String()

	return str, nil
}

func GetContainerPorts(spec *corev1.PodSpec) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{}
	for _, container := range spec.Containers {
		for _, port := range container.Ports {
			ports = append(ports, port)
		}
	}
	return ports
}
