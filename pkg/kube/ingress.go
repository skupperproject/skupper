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
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
)

type IngressRoute struct {
	Name        string
	Host        string
	ServiceName string
	ServicePort int
	Resolve     bool
}

func (r *IngressRoute) toRule() networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host: r.Host,
		IngressRuleValue: networkingv1.IngressRuleValue{
			HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{
					{
						Path: "/",
						Backend: networkingv1.IngressBackend{
							ServiceName: r.ServiceName,
							ServicePort: intstr.FromInt(r.ServicePort),
						},
					},
				},
			},
		},
	}
}

func fromRule(rule *networkingv1.IngressRule) IngressRoute {
	return IngressRoute{
		Host:        rule.Host,
		ServiceName: rule.IngressRuleValue.HTTP.Paths[0].Backend.ServiceName,
		ServicePort: rule.IngressRuleValue.HTTP.Paths[0].Backend.ServicePort.IntValue(),
	}
}

func getStatus(ingress *networkingv1.Ingress) *corev1.LoadBalancerIngress {
	if len(ingress.Status.LoadBalancer.Ingress) > 0 {
		status := ingress.Status.LoadBalancer.Ingress[0]
		if status.IP != "" || status.Hostname != "" {
			return &status
		}
	}
	return nil
}

func CreateIngress(name string, routes []IngressRoute, sslPassthrough bool, owner *metav1.OwnerReference, namespace string, cli kubernetes.Interface) error {
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				"kubernetes.io/ingress.class": "nginx",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{},
		},
	}
	if sslPassthrough {
		ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
		ingress.ObjectMeta.Annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
	}
	if owner != nil {
		ingress.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}

	}
	resolve := false
	for _, route := range routes {
		ingress.Spec.Rules = append(ingress.Spec.Rules, route.toRule())
		if route.Resolve {
			resolve = true
		}
	}
	updated, err := cli.NetworkingV1beta1().Ingresses(namespace).Create(&ingress)
	if err != nil {
		return err
	}
	if resolve {
		hostOrIp := getStatus(updated)
		for i := 0; i < 60 && hostOrIp == nil; i++ {
			if i%10 == 0 {
				fmt.Printf("Waiting for ingress host/ip...")
				fmt.Println()
			}
			time.Sleep(time.Second)
			updated, err = cli.NetworkingV1beta1().Ingresses(namespace).Get(name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			hostOrIp = getStatus(updated)
		}
		if hostOrIp == nil {
			return fmt.Errorf("Could not resolve host or ip of ingress")
		}
		updated.Spec.Rules = nil
		for _, route := range routes {
			if route.Resolve {
				base := hostOrIp.Hostname
				if hostOrIp.IP != "" {
					base = hostOrIp.IP + ".nip.io"
				}
				route.Host = strings.Join([]string{route.Host, namespace, base}, ".")
			}
			updated.Spec.Rules = append(updated.Spec.Rules, route.toRule())
		}
		_, err = cli.NetworkingV1beta1().Ingresses(namespace).Update(updated)
		if err != nil {
			return err
		}
	}
	return nil
}

func GetIngressRoutes(name string, namespace string, cli kubernetes.Interface) ([]IngressRoute, error) {
	var routes []IngressRoute
	ingress, err := cli.NetworkingV1beta1().Ingresses(namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return routes, nil
	} else if err != nil {
		return routes, err
	}
	for _, rule := range ingress.Spec.Rules {
		routes = append(routes, fromRule(&rule))
	}
	return routes, nil
}
