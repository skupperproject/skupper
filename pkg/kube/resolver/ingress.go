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
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

type IngressResolver struct {
	clients   internalclient.Clients
	namespace string
}

func NewIngressResolver(clients internalclient.Clients, namespace string) Resolver {
	return &IngressResolver{
		clients:   clients,
		namespace: namespace,
	}
}

func (*IngressResolver) IsLocalAccessOnly() bool {
	return false
}

func (r *IngressResolver) GetAllHosts() ([]string, error) {
	ingressRoutes, err := kube.GetIngressRoutes(types.IngressName, r.namespace, r.clients)
	if err != nil {
		return nil, err
	}
	var hosts []string
	for _, route := range ingressRoutes {
		hosts = append(hosts, route.Host)
	}
	return hosts, nil
}

func (r *IngressResolver) getHostPort(port int32) (HostPort, error) {
	ingressRoutes, err := kube.GetIngressRoutes(types.IngressName, r.namespace, r.clients)
	if err != nil {
		return HostPort{}, err
	}
	if len(ingressRoutes) > 0 {
		for _, route := range ingressRoutes {
			if route.ServicePort == int(port) {
				return HostPort{
					Host: route.Host,
					Port: 443,
				}, nil
			}
		}
	}
	return HostPort{}, fmt.Errorf("Could not find ingress rule for port %d", port)
}

func (r *IngressResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterListenerPort)
}

func (r *IngressResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeListenerPort)
}

func (r *IngressResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimRedemptionPort)
}
