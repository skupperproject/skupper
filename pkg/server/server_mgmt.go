package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

func GetSiteInfo(namespace string, clientset kubernetes.Interface, config *restclient.Config) ([]types.SiteInfo, error) {
	command := getQueryServiceController("sites")
	buffer, err := serviceControllerExec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		var results []types.SiteInfo
		err = json.Unmarshal(buffer.Bytes(), &results)

		if err != nil {
			fmt.Println("Failed to parse JSON:", err, buffer.String())
			return nil, err
		} else {
			return results, nil
		}
	}
}

func getQueryServiceController(typename string) []string {
	return []string{
		"get",
		typename,
		"-o",
		"json",
	}
}

func serviceControllerExec(command []string, namespace string, clientset kubernetes.Interface, config *restclient.Config) (*bytes.Buffer, error) {
	pod, err := kube.GetReadyPod(namespace, clientset, "service-controller")
	if err != nil {
		return nil, err
	}

	return kube.ExecCommandInContainer(command, pod.Name, "service-controller", namespace, clientset, config)
}
