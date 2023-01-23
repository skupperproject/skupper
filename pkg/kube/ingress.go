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
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
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

func addNginxIngressAnnotations(sslPassthrough bool, annotations map[string]string) {
	annotations["kubernetes.io/ingress.class"] = "nginx"
	if sslPassthrough {
		annotations["nginx.ingress.kubernetes.io/ssl-passthrough"] = "true"
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
	}
}

type Clients interface {
	GetKubeClient() kubernetes.Interface
	GetDynamicClient() dynamic.Interface
	GetDiscoveryClient() *discovery.DiscoveryClient
}

func CreateIngress(name string, routes []IngressRoute, isNginx bool, sslPassthrough bool, ownerRefs []metav1.OwnerReference, namespace string, annotations map[string]string, clients Clients) error {
	if useV1API(clients.GetDiscoveryClient()) {
		if isNginx {
			if annotations == nil {
				annotations = map[string]string{}
			}
			addNginxIngressAnnotations(sslPassthrough, annotations)
		}
		resolve := false
		for _, route := range routes {
			if route.Resolve {
				resolve = true
			}
		}
		return ensureIngressRoutesV1(clients.GetDynamicClient(), namespace, name, routes, annotations, ownerRefs, resolve)
	}
	cli := clients.GetKubeClient()
	ingress := networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{},
		},
	}
	if isNginx {
		addNginxIngressAnnotations(sslPassthrough, ingress.ObjectMeta.Annotations)
	}

	for key, value := range annotations {
		ingress.ObjectMeta.Annotations[key] = value
	}

	if ownerRefs != nil {
		ingress.ObjectMeta.OwnerReferences = ownerRefs
	}
	resolve := false
	for _, route := range routes {
		ingress.Spec.Rules = append(ingress.Spec.Rules, route.toRule())
		if route.Resolve {
			resolve = true
		}
	}
	updated, err := cli.NetworkingV1beta1().Ingresses(namespace).Create(context.TODO(), &ingress, metav1.CreateOptions{})
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
			updated, err = cli.NetworkingV1beta1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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
		_, err = cli.NetworkingV1beta1().Ingresses(namespace).Update(context.TODO(), updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func GetIngressRoutes(name string, namespace string, clients Clients) ([]IngressRoute, error) {
	if useV1API(clients.GetDiscoveryClient()) {
		return getIngressRoutesV1(clients.GetDynamicClient(), namespace, name)
	}
	cli := clients.GetKubeClient()
	var routes []IngressRoute
	ingress, err := cli.NetworkingV1beta1().Ingresses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
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

var ingressResource = schema.GroupVersionResource{
	Group:    "networking.k8s.io",
	Version:  "v1",
	Resource: "ingresses",
}
var ingressGVK = schema.GroupVersionKind{
	Group:   ingressResource.Group,
	Version: ingressResource.Version,
	Kind:    "Ingress",
}

func getIngressRoutesV1(client dynamic.Interface, namespace string, name string) ([]IngressRoute, error) {
	obj, err := client.Resource(ingressResource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return readIngressRules(obj)
}

func appendIngressHostSuffix(desired []IngressRoute, suffix string) []IngressRoute {
	var updated []IngressRoute
	for _, r := range desired {
		r.Host = r.Host + suffix
		updated = append(updated, r)
	}
	return updated
}

func ensureIngressRoutesV1(client dynamic.Interface, namespace string, name string, routes []IngressRoute, annotations map[string]string, ownerRefs []metav1.OwnerReference, resolve bool) error {
	obj, err := client.Resource(ingressResource).Namespace(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		obj = &unstructured.Unstructured{}
		ingressClassName, ok := annotations["kubernetes.io/ingress.class"]
		if ok {
			delete(annotations, "kubernetes.io/ingress.class")
		}
		obj.SetGroupVersionKind(ingressGVK)
		obj.SetName(name)
		obj.SetOwnerReferences(ownerRefs)
		obj.SetAnnotations(annotations)
		err := writeIngressRules(routes, obj)
		if err != nil {
			return err
		}
		err = unstructured.SetNestedField(obj.UnstructuredContent(), ingressClassName, "spec", "ingressClassName")
		if err != nil {
			return err
		}
		_, err = client.Resource(ingressResource).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		return nil
	} else if err != nil {
		return err
	}
	update := false
	if !reflect.DeepEqual(obj.GetOwnerReferences(), ownerRefs) {
		obj.SetOwnerReferences(ownerRefs)
		update = true
	}
	if !reflect.DeepEqual(obj.GetAnnotations(), annotations) {
		obj.SetAnnotations(annotations)
		update = true
	}
	existing, err := readIngressRules(obj)
	base := ""
	if resolve {
		base = resolveBaseHostOrIp(obj)
		if base != "" {
			base = "." + namespace + "." + base
			routes = appendIngressHostSuffix(routes, base)
		}
	}
	if err != nil || !reflect.DeepEqual(existing, routes) {
		err = writeIngressRules(routes, obj)
		if err != nil {
			return err
		}
		update = true
	}
	if update {
		_, err = client.Resource(ingressResource).Namespace(namespace).Update(context.TODO(), obj, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func writeIngressRules(routes []IngressRoute, obj *unstructured.Unstructured) error {
	var rules []interface{}
	for _, route := range routes {
		rules = append(rules, route.toUntypedRule())
	}
	err := unstructured.SetNestedSlice(obj.UnstructuredContent(), rules, "spec", "rules")
	if err != nil {
		return err
	}
	return nil
}

func readIngressRules(obj *unstructured.Unstructured) ([]IngressRoute, error) {
	rules, _, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "rules")
	if err != nil {
		return nil, err
	}
	var routes []IngressRoute
	for _, r := range rules {
		route := IngressRoute{}
		err := route.fromUntypedRule(r)
		if err != nil {
			return routes, err
		}
		routes = append(routes, route)
	}
	return routes, nil
}

func resolveBaseHostOrIp(obj *unstructured.Unstructured) string {
	ingresses, _, err := unstructured.NestedSlice(obj.UnstructuredContent(), "status", "loadBalancer", "ingress")
	if err != nil || len(ingresses) == 0 {
		return ""
	}
	ingress, ok := ingresses[0].(map[string]interface{})
	if !ok {
		return ""
	}
	host, _, _ := unstructured.NestedString(ingress, "host")
	if host != "" {
		return host
	}
	ip, _, _ := unstructured.NestedString(ingress, "ip")
	if ip != "" {
		return ip + ".nip.io"
	}
	return ""
}

func (r *IngressRoute) fromUntypedRule(u interface{}) error {
	rule, ok := u.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Could not parse rule")
	}
	host, _, err := unstructured.NestedString(rule, "host")
	if err != nil {
		return err
	}
	paths, _, err := unstructured.NestedSlice(rule, "http", "paths")
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("No paths in rule")
	}
	path, ok := paths[0].(map[string]interface{})
	serviceName, _, err := unstructured.NestedString(path, "backend", "service", "name")
	if err != nil {
		return err
	}
	servicePort, _, err := unstructured.NestedInt64(path, "backend", "service", "port", "number")
	if err != nil {
		return err
	}

	r.Name = strings.Split(host, ".")[0]
	r.Host = host
	r.ServiceName = serviceName
	r.ServicePort = int(servicePort)
	return nil
}

func (r *IngressRoute) toUntypedRule() interface{} {
	return map[string]interface{}{
		"host": r.Host,
		"http": map[string]interface{}{
			"paths": []interface{}{
				map[string]interface{}{
					"path":     "/",
					"pathType": "Prefix",
					"backend": map[string]interface{}{
						"service": map[string]interface{}{
							"name": r.ServiceName,
							"port": map[string]interface{}{
								"number": int64(r.ServicePort),
							},
						},
					},
				},
			},
		},
	}
}

func useV1API(dc *discovery.DiscoveryClient) bool {
	if dc == nil {
		return false
	}
	_, err := dc.ServerResourcesForGroupVersion("networking.k8s.io/v1beta1")
	return err != nil
}
