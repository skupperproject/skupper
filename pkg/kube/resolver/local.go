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
	"k8s.io/client-go/kubernetes"
)

type LocalServiceResolver struct {
	namespace string
}

func NewLocalServiceResolver(client kubernetes.Interface, namespace string) Resolver {
	return &LocalServiceResolver{
		namespace: namespace,
	}
}

func (*LocalServiceResolver) IsLocalAccessOnly() bool {
	return true
}

func (r *LocalServiceResolver) GetAllHosts() ([]string, error) {
	return []string{r.getHostPort(0).Host}, nil
}

func (r *LocalServiceResolver) getHostPort(port int32) HostPort {
	return HostPort{
		Host: fmt.Sprintf("%s.%s", types.TransportServiceName, r.namespace),
		Port: port,
	}
}

func (r *LocalServiceResolver) GetHostPortForInterRouter() (HostPort, error) {
	return r.getHostPort(types.InterRouterListenerPort), nil
}

func (r *LocalServiceResolver) GetHostPortForEdge() (HostPort, error) {
	return r.getHostPort(types.EdgeListenerPort), nil
}

func (r *LocalServiceResolver) GetHostPortForClaims() (HostPort, error) {
	return r.getHostPort(types.ClaimRedemptionPort), nil
}
