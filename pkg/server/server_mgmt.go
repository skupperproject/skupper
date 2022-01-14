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

func GetSiteInfo(namespace string, clientset kubernetes.Interface, config *restclient.Config) (*[]types.SiteInfo, error) {

	command := getQueryServiceController("sites")
	buffer, err := serviceControllerExec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		var results []types.SiteInfo
		err = json.Unmarshal(buffer.Bytes(), &results)

		if err != nil {
			return nil, fmt.Errorf("error when unmarshalling response from service controller: %s", string(buffer.Bytes()))
		} else {
			return &results, nil
		}
	}
}

func GetServiceInfo(namespace string, clientset kubernetes.Interface, config *restclient.Config) (*[]types.ServiceInfo, error) {
	command := getQueryServiceController("services")
	buffer, err := serviceControllerExec(command, namespace, clientset, config)
	if err != nil {
		return nil, err
	} else {
		var results []types.ServiceInfo
		err = json.Unmarshal(buffer.Bytes(), &results)

		if err != nil {
			return nil, fmt.Errorf("error when unmarshalling response from service controller: %s", string(buffer.Bytes()))
		} else {
			return &results, nil
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
		return nil, fmt.Errorf("service controller pod is not ready yet")
	}

	results, err := kube.ExecCommandInContainer(command, pod.Name, "service-controller", namespace, clientset, config)

	if err != nil {
		return nil, fmt.Errorf("service controller not ready")
	}

	return results, nil
}
