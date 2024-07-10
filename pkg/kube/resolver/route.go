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
	"context"
	"fmt"

	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RouteResolver struct {
	client    routev1client.RouteV1Interface
	namespace string
}

func NewRouteResolver(clients internalclient.Clients, namespace string) (Resolver, error) {
	client := clients.GetRouteClient()
	if client == nil {
		return nil, fmt.Errorf("Route client not configured, but ingress set to route")
	}
	return &RouteResolver{
		client:    client,
		namespace: namespace,
	}, nil
}

func (*RouteResolver) IsLocalAccessOnly() bool {
	return false
}

func (r *RouteResolver) GetAllHosts() ([]string, error) {
	var hosts []string
	for _, name := range []string{types.InterRouterRouteName, types.EdgeRouteName, types.ClaimRedemptionRouteName} {
		hostport, err := r.getHostPort(name)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, hostport.Host)
	}
	return hosts, nil
}

func (r *RouteResolver) getHostPort(name string) (HostPort, error) {
	route, err := r.client.Routes(r.namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return HostPort{}, err
	}
	return HostPort{
		Host: route.Spec.Host,
		Port: 443,
	}, nil
}

func (r *RouteResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterRouteName)
}

func (r *RouteResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeRouteName)
}

func (r *RouteResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimRedemptionRouteName)
}
