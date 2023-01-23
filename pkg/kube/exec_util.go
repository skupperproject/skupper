package kube

import (
	"bufio"
	"bytes"
	"context"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func ExecCommandInContainer(command []string, podName string, containerName string, namespace string, clientset kubernetes.Interface, config *restclient.Config) (*bytes.Buffer, error) {
	pod, err := clientset.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// TODO: check that pod is ready or wait for it to be
	targetContainer := pod.Spec.Containers[0].Name
	if containerName != "" {
		targetContainer = containerName
	}

	var stdout io.Writer

	buffer := bytes.Buffer{}
	stdout = bufio.NewWriter(&buffer)

	restClient, err := restclient.RESTClientFor(config)
	if err != nil {
		panic(err)
	}

	req := restClient.Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: targetContainer,
		Command:   command,
		Stdin:     false,
		Stdout:    true,
		Stderr:    false,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(config, "POST", req.URL())
	if err != nil {
		panic(err)
	}
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: stdout,
		Stderr: nil,
	})
	if err != nil {
		return nil, err
	} else {
		return &buffer, nil
	}
}
