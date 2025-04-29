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

package client

import (
	"bytes"
	"context"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/internal/utils"
)

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

func getPods(selector string, namespace string, cli kubernetes.Interface) ([]corev1.Pod, error) {
	options := metav1.ListOptions{LabelSelector: selector}
	podList, err := cli.CoreV1().Pods(namespace).List(context.TODO(), options)
	if err != nil {
		return nil, err
	}
	return podList.Items, err
}

func WaitForPodsSelectorStatus(namespace string, clientset kubernetes.Interface, selector string, status corev1.PodPhase, timeout time.Duration, interval time.Duration) ([]corev1.Pod, error) {
	var pods []corev1.Pod
	var pod corev1.Pod
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		pods, err = getPods(selector, namespace, clientset)
		if err != nil {
			// pod does not exist yet
			return false, nil
		}
		for _, pod = range pods {
			if pod.Status.Phase != status {
				return false, nil
			}
		}
		return true, nil
	})

	return pods, err
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
	podLogs, err := req.Stream(context.TODO())
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
