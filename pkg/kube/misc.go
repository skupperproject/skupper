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
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllContainerPorts(container corev1.Container) map[int]int {
	ports := map[int]int{}
	for _, cp := range container.Ports {
		ports[int(cp.ContainerPort)] = int(cp.ContainerPort)
	}
	return ports
}

func PortMapToLabelStr(portMap map[int]int) string {
	portStr := ""
	for iPort, tPort := range portMap {
		if len(portStr) > 0 {
			portStr += ","
		}
		portStr += fmt.Sprintf("%d:%d", iPort, tPort)
	}
	return portStr
}

func PortLabelStrToMap(portsStr string) map[int]int {
	var err error
	portMap := map[int]int{}
	if portsStr == "" {
		return portMap
	}
	for _, port := range strings.Split(portsStr, ",") {
		ports := strings.Split(port, ":")
		var iPort, tPort int
		iPort, err = strconv.Atoi(ports[0])
		if err != nil {
			return map[int]int{}
		}
		tPort = iPort
		if len(ports) > 1 {
			tPort, err = strconv.Atoi(ports[1])
			if err != nil {
				return map[int]int{}
			}
		}
		portMap[iPort] = tPort
	}
	return portMap
}

func SetAnnotation(o *metav1.ObjectMeta, key string, value string) {
	if o.Annotations == nil {
		o.Annotations = map[string]string{}
	}
	o.Annotations[key] = value
}

func SetLabel(o *metav1.ObjectMeta, key string, value string) {
	if o.Labels == nil {
		o.Labels = map[string]string{}
	}
	o.Labels[key] = value
}

func UpdateLabels(o *metav1.ObjectMeta, desired map[string]string) bool {
	if reflect.DeepEqual(desired, o.Labels) {
		return false
	}
	if o.Labels == nil {
		o.Labels = desired
	} else {
		//note this only adds new labels, it never removes any (is that what is wanted?)
		for k, v := range desired {
			o.Labels[k] = v
		}
	}
	return true
}

func UpdateAnnotations(o *metav1.ObjectMeta, desired map[string]string) bool {
	if reflect.DeepEqual(desired, o.Annotations) {
		return false
	}
	if o.Annotations == nil {
		o.Annotations = desired
	} else {
		//note this only adds new annotations, it never removes any
		for k, v := range desired {
			o.Annotations[k] = v
		}
	}
	return true
}
