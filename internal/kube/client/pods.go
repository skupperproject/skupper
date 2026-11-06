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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func getPods(selector string, namespace string, cli kubernetes.Interface) ([]corev1.Pod, error) {
	options := metav1.ListOptions{LabelSelector: selector}
	podList, err := cli.CoreV1().Pods(namespace).List(context.TODO(), options)
	if err != nil {
		return nil, err
	}
	return podList.Items, err
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
