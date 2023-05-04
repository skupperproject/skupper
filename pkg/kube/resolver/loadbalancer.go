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
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"k8s.io/client-go/kubernetes"
)

type LoadBalancerResolver struct {
	client    kubernetes.Interface
	namespace string
}

func NewLoadBalancerResolver(client kubernetes.Interface, namespace string) Resolver {
	return &LoadBalancerResolver{
		client:    client,
		namespace: namespace,
	}
}

func (*LoadBalancerResolver) IsLocalAccessOnly() bool {
	return false
}

func (r *LoadBalancerResolver) getHost() (string, error) {
	service, err := kube.GetService(types.TransportServiceName, r.namespace, r.client)
	if err != nil {
		return "", err
	}
	return kube.GetLoadBalancerHostOrIP(service), nil
}

func (r *LoadBalancerResolver) GetAllHosts() ([]string, error) {
	host, err := r.getHost()
	if err != nil {
		return nil, err
	}
	if host == "" {
		return []string{}, nil
	}
	return []string{host}, nil
}

func (r *LoadBalancerResolver) getHostPort(port int32) (HostPort, error) {
	host, err := r.getHost()
	if err != nil {
		return HostPort{}, err
	}
	return HostPort{
		Host: host,
		Port: port,
	}, nil
}

func (r *LoadBalancerResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterListenerPort)
}

func (r *LoadBalancerResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeListenerPort)
}

func (r *LoadBalancerResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimRedemptionPort)
}
