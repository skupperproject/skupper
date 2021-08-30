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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/utils"
)

func GetServiceInterfaceTarget(targetType string, targetName string, deducePort bool, namespace string, cli kubernetes.Interface) (*types.ServiceInterfaceTarget, map[string]string, error) {
	var labels map[string]string
	if targetType == "deployment" {
		deployment, err := cli.AppsV1().Deployments(namespace).Get(targetName, metav1.GetOptions{})
		if err == nil {
			target := types.ServiceInterfaceTarget{
				Name:     deployment.ObjectMeta.Name,
				Selector: utils.StringifySelector(deployment.Spec.Selector.MatchLabels),
			}
			if deducePort {
				//TODO: handle case where there is more than one container (need --container option?)
				if deployment.Spec.Template.Spec.Containers[0].Ports != nil {
					target.TargetPort = int(deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
				}
			}
			labels = deployment.GetLabels()
			return &target, labels, nil
		} else {
			return nil, nil, fmt.Errorf("Could not read deployment %s: %s", targetName, err)
		}
	} else if targetType == "statefulset" {
		statefulset, err := cli.AppsV1().StatefulSets(namespace).Get(targetName, metav1.GetOptions{})
		if err == nil {
			target := types.ServiceInterfaceTarget{
				Name:     statefulset.ObjectMeta.Name,
				Selector: utils.StringifySelector(statefulset.Spec.Selector.MatchLabels),
			}
			if deducePort {
				//TODO: handle case where there is more than one container (need --container option?)
				if statefulset.Spec.Template.Spec.Containers[0].Ports != nil {
					target.TargetPort = int(statefulset.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort)
				}
			}
			labels = statefulset.GetLabels()
			return &target, labels, nil
		} else {
			return nil, nil, fmt.Errorf("Could not read statefulset %s: %s", targetName, err)
		}
	} else if targetType == "pods" {
		return nil, nil, fmt.Errorf("VAN service interfaces for pods not yet implemented")
	} else if targetType == "service" {
		target := types.ServiceInterfaceTarget{
			Name:    targetName,
			Service: targetName,
		}
		if deducePort {
			port, err := GetPortForServiceTarget(targetName, namespace, cli)
			if err != nil {
				return nil, nil, err
			}
			if port != 0 {
				target.TargetPort = port
			}
		}
		targetSvc, err := GetService(targetName, namespace, cli)
		if err != nil {
			return nil, nil, err
		}
		labels = targetSvc.GetLabels()
		return &target, labels, nil
	} else {
		return nil, nil, fmt.Errorf("VAN service interface unsupported target type")
	}
}
