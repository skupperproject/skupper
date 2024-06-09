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
	openshiftroute "github.com/openshift/client-go/route/clientset/versioned"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	skupperclient "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned"
)

type Clients interface {
	GetKubeClient() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
	GetDiscoveryClient() discovery.DiscoveryInterface
	GetRouteInterface() openshiftroute.Interface
	GetRouteClient() routev1client.RouteV1Interface
	GetSkupperClient() skupperclient.Interface
}
