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

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
)

type HostPort struct {
	Host string `json:"host,omitempty"`
	Port int32  `json:"port,omitempty"`
}

type HostPorts struct {
	InterRouter HostPort `json:"inter-router,omitempty"`
	Edge        HostPort `json:"edge,omitempty"`
	Claims      HostPort `json:"claims,omitempty"`
}

type Resolver interface {
	IsLocalAccessOnly() bool
	GetAllHosts() ([]string, error)
	GetHostPortForInterRouter() (HostPort, error)
	GetHostPortForEdge() (HostPort, error)
	GetHostPortForClaims() (HostPort, error)
}

type IngressConfig interface {
	IsIngressRoute() bool
	IsIngressLoadBalancer() bool
	IsIngressNodePort() bool
	IsIngressNginxIngress() bool
	IsIngressContourHttpProxy() bool
	IsIngressKubernetes() bool
	IsIngressPodmanHost() bool
	IsIngressNone() bool
	GetRouterIngressHost() string
}

func NewResolver(clients internalclient.Clients, namespace string, ingress IngressConfig) (Resolver, error) {
	client := clients.GetKubeClient()
	if ingress.IsIngressRoute() {
		return NewRouteResolver(clients, namespace)
	}
	if ingress.IsIngressLoadBalancer() {
		return NewLoadBalancerResolver(client, namespace), nil
	}
	if ingress.IsIngressNodePort() {
		return NewNodePortResolver(client, namespace, ingress.GetRouterIngressHost()), nil
	}
	if ingress.IsIngressNginxIngress() || ingress.IsIngressKubernetes() {
		return NewIngressResolver(clients, namespace), nil
	}
	if ingress.IsIngressContourHttpProxy() {
		return NewContourHttpProxyResolver(client, namespace, ingress.GetRouterIngressHost()), nil
	}
	if ingress.IsIngressNone() {
		return NewLocalServiceResolver(client, namespace), nil
	}
	return nil, fmt.Errorf("Could not determine ingress type")
}
