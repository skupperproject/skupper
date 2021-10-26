package kube

import (
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var httpProxyResource = schema.GroupVersionResource{
	Group:    "projectcontour.io",
	Version:  "v1",
	Resource: "httpproxies",
}
var httpProxyGVK = schema.GroupVersionKind{
	Group:   httpProxyResource.Group,
	Version: httpProxyResource.Version,
	Kind:    "HTTPProxy",
}

func (p *IngressRoute) readFromContourProxy(obj *unstructured.Unstructured) error {
	host, _, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "virtualhost", "fqdn")
	if err != nil {
		return err
	}
	p.Host = host
	services, _, err := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "tcpproxy", "services")
	if err != nil {
		return err
	}
	if len(services) > 0 {
		service, ok := services[0].(map[string]interface{})
		if ok {
			name, _, err := unstructured.NestedString(service, "name")
			if err != nil {
				return err
			}
			port, _, err := unstructured.NestedInt64(service, "port")
			if err != nil {
				return err
			}
			p.ServiceName = name
			p.ServicePort = int(port)
		}
	}
	return nil
}

func (p *IngressRoute) writeToContourProxy(obj *unstructured.Unstructured) error {
	err := unstructured.SetNestedField(obj.UnstructuredContent(), p.Host, "spec", "virtualhost", "fqdn")
	if err != nil {
		return err
	}
	err = unstructured.SetNestedField(obj.UnstructuredContent(), true, "spec", "virtualhost", "tls", "passthrough")
	if err != nil {
		return err
	}
	services := []interface{}{
		map[string]interface{}{
			"name": p.ServiceName,
			"port": int64(p.ServicePort),
		},
	}
	err = unstructured.SetNestedSlice(obj.UnstructuredContent(), services, "spec", "tcpproxy", "services")
	if err != nil {
		return err
	}
	return nil
}

func GetContourProxy(client dynamic.Interface, namespace string, name string) (*IngressRoute, error) {
	obj, err := client.Resource(httpProxyResource).Namespace(namespace).Get(name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	proxy := IngressRoute{}
	err = proxy.readFromContourProxy(obj)
	if err != nil {
		return nil, err
	}
	return &proxy, nil
}

func CreateContourProxy(client dynamic.Interface, namespace string, name string, def IngressRoute, owner *metav1.OwnerReference) error {
	obj := unstructured.Unstructured{}
	obj.SetGroupVersionKind(httpProxyGVK)
	obj.SetName(name)
	obj.SetOwnerReferences([]metav1.OwnerReference{*owner})
	err := def.writeToContourProxy(&obj)
	if err != nil {
		return err
	}
	_, err = client.Resource(httpProxyResource).Namespace(namespace).Create(&obj, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func CreateContourProxies(routes []IngressRoute, owner *metav1.OwnerReference, client dynamic.Interface, namespace string) error {
	for _, route := range routes {
		err := CreateContourProxy(client, namespace, route.Name, route, owner)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}
	return nil
}
