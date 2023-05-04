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
	"k8s.io/client-go/kubernetes"
	"strings"
)

type ContourHttpProxyResolver struct {
	client    kubernetes.Interface
	namespace string
	suffix    string
}

func NewContourHttpProxyResolver(client kubernetes.Interface, namespace string, suffix string) Resolver {
	return &ContourHttpProxyResolver{
		client:    client,
		namespace: namespace,
		suffix:    suffix,
	}
}

func (*ContourHttpProxyResolver) IsLocalAccessOnly() bool {
	return false
}

func (r *ContourHttpProxyResolver) GetAllHosts() ([]string, error) {
	var hosts []string
	for _, prefix := range []string{types.InterRouterIngressPrefix, types.EdgeIngressPrefix, types.ClaimsIngressPrefix} {
		hosts = append(hosts, r.getHost(prefix))
	}
	return hosts, nil
}

func (r *ContourHttpProxyResolver) getHost(prefix string) string {
	return strings.Join([]string{prefix, r.namespace, r.suffix}, ".")
}

func (r *ContourHttpProxyResolver) getHostPort(prefix string) HostPort {
	return HostPort{
		Host: r.getHost(prefix),
		Port: 443,
	}
}

func (r *ContourHttpProxyResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterIngressPrefix), nil
}

func (r *ContourHttpProxyResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeIngressPrefix), nil
}

func (r *ContourHttpProxyResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimsIngressPrefix), nil
}
