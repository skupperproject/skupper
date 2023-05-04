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

package resolver

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

type NodePortResolver struct {
	client      kubernetes.Interface
	namespace   string
	ingressHost string
}

func NewNodePortResolver(client kubernetes.Interface, namespace string, ingressHost string) Resolver {
	return &NodePortResolver{
		client:      client,
		namespace:   namespace,
		ingressHost: ingressHost,
	}
}

func (*NodePortResolver) IsLocalAccessOnly() bool {
	return false
}

func (r *NodePortResolver) GetAllHosts() ([]string, error) {
	return []string{r.ingressHost}, nil
}

func (r *NodePortResolver) getHostPort(portName string) (HostPort, error) {
	result := HostPort{
		Host: r.ingressHost,
	}
	service, err := kube.GetService(types.TransportServiceName, r.namespace, r.client)
	if err != nil {
		return result, err
	}
	port := findPortByName(service, portName)
	if port == nil {
		return result, fmt.Errorf("NodePort for %s not found.", portName)
	}
	result.Port = port.NodePort
	return result, nil
}

func (r *NodePortResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterRole)
}

func (r *NodePortResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeRole)
}

func (r *NodePortResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimRedemptionPortName)
}

func findPortByName(service *corev1.Service, name string) *corev1.ServicePort {
	for _, port := range service.Spec.Ports {
		if port.Name == name {
			return &port
		}
	}
	return nil
}
