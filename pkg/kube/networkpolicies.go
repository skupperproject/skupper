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
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/api/types"
)

func getPort(port int32) *intstr.IntOrString {
	value := intstr.FromInt(int(port))
	return &value
}

func CreateNetworkPolicy(ownerrefs []metav1.OwnerReference, namespace string, cli kubernetes.Interface) error {
	policy := networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "skupper",
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					types.ComponentAnnotation: types.RouterComponent,
				},
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					//all pods from the same network can access any port
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{},
						},
					},
				},
				{
					//everything can access specific TLS protected ports
					From: []networkingv1.NetworkPolicyPeer{},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Port: getPort(types.AmqpsDefaultPort),
						},
						{
							Port: getPort(types.InterRouterListenerPort),
						},
						{
							Port: getPort(types.EdgeListenerPort),
						},
					},
				},
			},
		},
	}
	if ownerrefs != nil {
		policy.ObjectMeta.OwnerReferences = ownerrefs
	}
	_, err := cli.NetworkingV1().NetworkPolicies(namespace).Create(&policy)
	if err != nil {
		return err
	}
	return nil
}
