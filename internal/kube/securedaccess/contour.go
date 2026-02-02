package securedaccess

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type ContourHttpProxyAccessType struct {
	manager *SecuredAccessManager
	domain  string
	logger  *slog.Logger
}

func newContourHttpProxyAccess(manager *SecuredAccessManager, domain string) AccessType {
	return &ContourHttpProxyAccessType{
		manager: manager,
		domain:  domain,
		logger:  slog.New(slog.Default().Handler()).With(slog.String("component", "kube.securedaccess.contourHttpProxy.accessType")),
	}
}

func (o *ContourHttpProxyAccessType) RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, svc *corev1.Service) ([]skupperv2alpha1.Endpoint, error) {
	desired := desiredHttpProxies(qualify(access.Namespace, o.domain), access)

	var endpoints []skupperv2alpha1.Endpoint
	for _, proxy := range desired {
		obj, err := o.ensureHttpProxy(access.Namespace, proxy, ownerReferences(access))
		if err != nil {
			return nil, err
		}
		if endpoint := extractEndpoint(obj); endpoint.Host != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints, nil
}

func extractEndpoint(obj *unstructured.Unstructured) skupperv2alpha1.Endpoint {
	_, portName := getKeyAndPortNameForHttpProxy(obj)
	return skupperv2alpha1.Endpoint{
		Name: portName,
		Host: extractHost(obj),
		Port: "443",
	}
}

func getKeyAndPortNameForHttpProxy(o *unstructured.Unstructured) (string, string) {
	svcName := extractServiceName(o)
	portName := strings.TrimPrefix(o.GetName(), svcName+"-")
	return o.GetNamespace() + "/" + svcName, portName
}

func httpProxyName(svcName string, portName string) string {
	return svcName + "-" + portName
}

func (o *ContourHttpProxyAccessType) ensureHttpProxy(namespace string, desired HttpProxy, ownerRefs []metav1.OwnerReference) (*unstructured.Unstructured, error) {
	key := namespace + "/" + desired.Name
	if existing, ok := o.manager.httpProxies[key]; ok {
		actual := HttpProxy{
			Name: desired.Name,
		}
		changed := false
		if err := actual.readFromContourProxy(existing); err != nil {
			return nil, errors.New("Unexpected structure for HTTPProxy")
		}
		modified := existing.DeepCopy()
		if desired != actual {
			changed = true
		}
		if o.manager.context != nil {
			if o.manager.context.SetLabels(namespace, modified.GetName(), "HttpProxy", modified.GetLabels()) {
				changed = true
			}
			if o.manager.context.SetAnnotations(namespace, modified.GetName(), "HttpProxy", modified.GetAnnotations()) {
				changed = true
			}
		}
		if !changed {
			return existing, nil
		}
		if err := desired.writeToContourProxy(modified); err != nil {
			return nil, errors.New("Unexpected structure for HTTPProxy")
		}
		updated, err := updateContourProxy(o.manager.clients.GetDynamicClient(), modified)
		if err != nil {
			return nil, err
		}
		o.manager.httpProxies[key] = updated
		return updated, nil
	}
	labels := map[string]string{
		"internal.skupper.io/secured-access": "true",
	}
	annotations := map[string]string{
		"internal.skupper.io/controlled": "true",
	}
	o.logger.Info("Creating contour httpproxy")
	if o.manager.context != nil {
		o.manager.context.SetLabels(namespace, desired.Name, "HTTPProxy", labels)
		o.manager.context.SetAnnotations(namespace, desired.Name, "HTTPProxy", annotations)
	}
	created, err := createContourProxy(o.manager.clients.GetDynamicClient(), namespace, desired, labels, annotations, ownerRefs)
	if err != nil {
		return nil, err
	}
	o.manager.httpProxies[key] = created
	return created, nil
}

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

type HttpProxy struct {
	Name        string
	Host        string
	ServiceName string
	ServicePort int
}

func desiredHttpProxies(domain string, access *skupperv2alpha1.SecuredAccess) []HttpProxy {
	var proxies []HttpProxy
	for _, port := range access.Spec.Ports {
		name := httpProxyName(access.Name, port.Name)
		proxies = append(proxies, HttpProxy{
			Name:        name,
			Host:        qualify(name, domain),
			ServiceName: access.Name,
			ServicePort: port.Port,
		})
	}
	return proxies
}

func extractHost(obj *unstructured.Unstructured) string {
	host, _, _ := unstructured.NestedString(obj.UnstructuredContent(), "spec", "virtualhost", "fqdn")
	return host
}

func extractServiceName(obj *unstructured.Unstructured) string {
	proxy := &HttpProxy{}
	proxy.readFromContourProxy(obj)
	return proxy.ServiceName
}

func (p *HttpProxy) readFromContourProxy(obj *unstructured.Unstructured) error {
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

func (p *HttpProxy) writeToContourProxy(obj *unstructured.Unstructured) error {
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

func createContourProxy(client dynamic.Interface, namespace string, def HttpProxy, labels map[string]string, annotations map[string]string, ownerRefs []metav1.OwnerReference) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(httpProxyGVK)
	obj.SetName(def.Name)
	obj.SetOwnerReferences(ownerRefs)
	obj.SetLabels(labels)
	obj.SetAnnotations(annotations)
	def.writeToContourProxy(obj)
	return client.Resource(httpProxyResource).Namespace(namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
}

func updateContourProxy(client dynamic.Interface, proxy *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	return client.Resource(httpProxyResource).Namespace(proxy.GetNamespace()).Update(context.TODO(), proxy, metav1.UpdateOptions{})
}
