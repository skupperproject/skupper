package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"time"
)

func GetSiteInfo(namespace string, clientset kubernetes.Interface, config *restclient.Config) (*[]types.SiteInfo, error) {

	command := getQueryServiceController("sites")
	execResult, err := utils.TryUntil(3*time.Second, func() utils.Result {
		res, err := serviceControllerExec(command, namespace, clientset, config)
		return utils.Result{
			Value: res,
			Error: err,
		}
	})

	if err != nil {
		return nil, err
	} else {
		bufferResult := execResult.(*bytes.Buffer)
		var results []types.SiteInfo
		err = json.Unmarshal(bufferResult.Bytes(), &results)

		if err != nil {
			return nil, fmt.Errorf("error when unmarshalling response from service controller: %s", string(bufferResult.Bytes()))
		} else {
			return &results, nil
		}
	}
}

func GetServiceInfo(namespace string, clientset kubernetes.Interface, config *restclient.Config) (*[]types.ServiceInfo, error) {
	command := getQueryServiceController("services")
	execResult, err := utils.TryUntil(3*time.Second, func() utils.Result {
		res, err := serviceControllerExec(command, namespace, clientset, config)
		return utils.Result{
			Value: res,
			Error: err,
		}
	})

	if err != nil {
		return nil, err
	} else {
		bufferResult := execResult.(*bytes.Buffer)
		var results []types.ServiceInfo
		err = json.Unmarshal(bufferResult.Bytes(), &results)

		if err != nil {
			return nil, fmt.Errorf("error when unmarshalling response from service controller: %s", string(bufferResult.Bytes()))
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
		return nil, fmt.Errorf("service controller is not ready yet")
	}

	results, err := kube.ExecCommandInContainer(command, pod.Name, "service-controller", namespace, clientset, config)

	if err != nil {
		return nil, fmt.Errorf("service controller is not ready yet")
	}

	return results, nil
}
