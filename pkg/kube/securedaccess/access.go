package securedaccess

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/resource"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
)

type AccessType interface {
	RealiseAndResolve(access *skupperv1alpha1.SecuredAccess, service *corev1.Service) ([]skupperv1alpha1.Endpoint, error)
}

type ControllerContext struct {
	Namespace string
	Name      string
	UID       string
}

type SecuredAccessManager struct {
	definitions        map[string]*skupperv1alpha1.SecuredAccess
	services           map[string]*corev1.Service
	routes             map[string]*routev1.Route
	ingresses          map[string]*networkingv1.Ingress
	httpProxies        map[string]*unstructured.Unstructured
	tlsRoutes          map[string]*unstructured.Unstructured
	clients            internalclient.Clients
	certMgr            certificates.CertificateManager
	enabledAccessTypes map[string]AccessType
	defaultAccessType  string
	gatewayInit        func() error
}

func NewSecuredAccessManager(clients internalclient.Clients, certMgr certificates.CertificateManager, config *Config, context ControllerContext) *SecuredAccessManager {
	mgr := &SecuredAccessManager{
		definitions:        map[string]*skupperv1alpha1.SecuredAccess{},
		services:           map[string]*corev1.Service{},
		routes:             map[string]*routev1.Route{},
		ingresses:          map[string]*networkingv1.Ingress{},
		httpProxies:        map[string]*unstructured.Unstructured{},
		tlsRoutes:          map[string]*unstructured.Unstructured{},
		clients:            clients,
		certMgr:            certMgr,
		enabledAccessTypes: map[string]AccessType{},
		defaultAccessType:  config.getDefaultAccessType(clients),
	}
	for _, accessType := range config.EnabledAccessTypes {
		if accessType == ACCESS_TYPE_ROUTE {
			mgr.enabledAccessTypes[accessType] = newRouteAccess(mgr)
		} else if accessType == ACCESS_TYPE_LOADBALANCER {
			mgr.enabledAccessTypes[accessType] = newLoadbalancerAccess(mgr)
		} else if accessType == ACCESS_TYPE_INGRESS_NGINX {
			mgr.enabledAccessTypes[accessType] = newIngressAccess(mgr, true, config.IngressDomain)
		} else if accessType == ACCESS_TYPE_CONTOUR_HTTP_PROXY {
			mgr.enabledAccessTypes[accessType] = newContourHttpProxyAccess(mgr, config.HttpProxyDomain)
		} else if accessType == ACCESS_TYPE_GATEWAY {
			at, init, err := newGatewayAccess(mgr, config.GatewayClass, config.GatewayDomain, config.GatewayPort, context)
			if err != nil {
				log.Printf("Failed to create gateway, gateway access type will not be enabled: %s", err)
			} else {
				mgr.enabledAccessTypes[accessType] = at
				mgr.gatewayInit = init
			}
		} else if accessType == ACCESS_TYPE_NODEPORT {
			mgr.enabledAccessTypes[accessType] = newNodeportAccess(mgr, config.ClusterHost)
		} else if accessType == ACCESS_TYPE_LOCAL {
			mgr.enabledAccessTypes[accessType] = newLocalAccess(mgr)
		}
	}

	return mgr
}

func (m *SecuredAccessManager) Ensure(namespace string, name string, spec skupperv1alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if current, ok := m.definitions[key]; ok {
		if reflect.DeepEqual(spec, current.Spec) && reflect.DeepEqual(annotations, current.ObjectMeta.Annotations) {
			return nil
		}
		current.Spec = spec
		current.ObjectMeta.Annotations = annotations
		updated, err := m.clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = updated
		return nil
	} else {
		sa := &skupperv1alpha1.SecuredAccess{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "SecuredAccess",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: refs,
				Annotations:     annotations,
			},
			Spec: spec,
		}
		created, err := m.clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(namespace).Create(context.Background(), sa, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = created
		return nil
	}

}

func (m *SecuredAccessManager) SecuredAccessChanged(key string, current *skupperv1alpha1.SecuredAccess) error {
	m.definitions[key] = current
	return m.reconcile(current)
}

func (m *SecuredAccessManager) actualAccessType(sa *skupperv1alpha1.SecuredAccess) string {
	if sa.Spec.AccessType == "" {
		return m.defaultAccessType
	}
	return sa.Spec.AccessType
}

func (m *SecuredAccessManager) accessType(sa *skupperv1alpha1.SecuredAccess) AccessType {
	accessType := sa.Spec.AccessType
	if accessType == "" {
		accessType = m.defaultAccessType
	}
	if at, ok := m.enabledAccessTypes[accessType]; ok {
		return at
	}
	return newUnsupportedAccess(m)
}

func (m *SecuredAccessManager) reconcile(sa *skupperv1alpha1.SecuredAccess) error {
	svc, err := m.checkService(sa)
	if err != nil {
		if sa.SetConfigured(err) {
			return m.updateStatus(sa)
		}
		return nil
	}
	updated := false
	endpoints, resourceErr := m.accessType(sa).RealiseAndResolve(sa, svc)

	if sa.SetResolved(endpoints) {
		log.Printf("Resolved endpoints for %s: %v", sa.Key(), endpoints)
		updated = true
	}

	certErr := m.checkCertificate(sa)

	if sa.SetConfigured(errors.Join(resourceErr, certErr)) {
		updated = true
	}

	if !updated {
		return nil
	}
	return m.updateStatus(sa)
}
func (m *SecuredAccessManager) updateStatus(sa *skupperv1alpha1.SecuredAccess) error {
	latest, err := m.clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(sa.Namespace).UpdateStatus(context.TODO(), sa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.definitions[latest.Key()] = latest
	return nil
}

func (m *SecuredAccessManager) checkCertificate(sa *skupperv1alpha1.SecuredAccess) error {
	if sa.Spec.Issuer == "" {
		return nil
	}
	name := sa.Spec.Certificate
	if name == "" {
		name = sa.Name
	}
	return m.certMgr.Ensure(sa.Namespace, name, sa.Spec.Issuer, sa.Name, getHosts(sa), false, true, ownerReferences(sa))
}

func (m *SecuredAccessManager) checkService(sa *skupperv1alpha1.SecuredAccess) (*corev1.Service, error) {
	key := sa.Key()
	if svc, ok := m.services[key]; ok {
		update := false
		if updateSelector(&svc.Spec, sa.Spec.Selector) {
			update = true
		}
		if updatePorts(&svc.Spec, sa.Spec.Ports) {
			update = true
		}
		if updateType(&svc.Spec, m.actualAccessType(sa)) {
			update = true
		}
		if !update {
			return svc, nil
		}
		updated, err := m.clients.GetKubeClient().CoreV1().Services(sa.Namespace).Update(context.Background(), svc, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		m.services[key] = updated
		return updated, nil
	}
	return m.createService(sa)
}

func (m *SecuredAccessManager) createService(sa *skupperv1alpha1.SecuredAccess) (*corev1.Service, error) {
	key := sa.Key()
	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            sa.Name,
			OwnerReferences: ownerReferences(sa),
			Labels: map[string]string{
				"internal.skupper.io/secured-access": "true",
			},
			Annotations: map[string]string{
				"internal.skupper.io/controlled": "true",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: sa.Spec.Selector,
			Type:     serviceType(m.actualAccessType(sa)),
		},
	}
	//TODO: copy labels and annotations from SecuredAccess resource
	updatePorts(&service.Spec, sa.Spec.Ports)
	created, err := m.clients.GetKubeClient().CoreV1().Services(sa.Namespace).Create(context.Background(), service, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	m.services[key] = created
	return created, nil
}

func (m *SecuredAccessManager) SecuredAccessDeleted(key string) error {
	if _, ok := m.definitions[key]; ok {
		//any resources created for this secured access
		//instance should have owner references set to this
		//definition, so deleting this will cause them to be
		//deleted also
		delete(m.definitions, key)
	}
	return nil
}

func (m *SecuredAccessManager) RecoverRoute(route *routev1.Route) {
	key := fmt.Sprintf("%s/%s", route.Namespace, route.Name)
	m.routes[key] = route
}

func (m *SecuredAccessManager) RecoverHttpProxy(o *unstructured.Unstructured) {
	key := fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	m.httpProxies[key] = o
}

func (m *SecuredAccessManager) RecoverTlsRoute(o *unstructured.Unstructured) {
	key := fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	m.tlsRoutes[key] = o
}

func (m *SecuredAccessManager) RecoverIngress(ingress *networkingv1.Ingress) {
	key := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)
	m.ingresses[key] = ingress
}

func (m *SecuredAccessManager) RecoverService(svc *corev1.Service) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
	m.services[key] = svc
}

func (m *SecuredAccessManager) getDefinitionForPortQualifiedResourceKey(qualifiedKey string, expectedAccessType string) *skupperv1alpha1.SecuredAccess {
	for _, p := range possibleKeyPortNamePairs(qualifiedKey) {
		key, portName := p.get()
		if sa, ok := m.definitions[key]; ok {
			if hasPort(sa, portName) && m.actualAccessType(sa) == expectedAccessType {
				return sa
			}
		}
	}
	return nil
}

func (m *SecuredAccessManager) CheckRoute(key string, route *routev1.Route) error {
	sa := m.getDefinitionForPortQualifiedResourceKey(key, ACCESS_TYPE_ROUTE)
	if route == nil {
		delete(m.routes, key)
		if sa == nil {
			return nil
		}
	} else {
		m.routes[key] = route
		if sa == nil {
			log.Printf("Deleting route %s/%s as no matching ServiceAccess definition found", route.Namespace, route.Name)
			return m.clients.GetRouteClient().Routes(route.Namespace).Delete(context.Background(), route.Name, metav1.DeleteOptions{})
		}
	}
	return m.reconcile(sa)
}

func (m *SecuredAccessManager) CheckHttpProxy(key string, o *unstructured.Unstructured) error {
	sa := m.getDefinitionForPortQualifiedResourceKey(key, ACCESS_TYPE_CONTOUR_HTTP_PROXY)
	if o == nil {
		delete(m.httpProxies, key)
		if sa == nil {
			return nil
		}
	} else {
		m.httpProxies[key] = o
		if sa == nil {
			log.Printf("Deleting redundant HttpProxy %s/%s", o.GetNamespace(), o.GetName())
			return m.clients.GetDynamicClient().Resource(httpProxyResource).Namespace(o.GetNamespace()).Delete(context.Background(), o.GetName(), metav1.DeleteOptions{})
		}
	}
	return m.reconcile(sa)
}

func (m *SecuredAccessManager) CheckTlsRoute(key string, o *unstructured.Unstructured) error {
	sa := m.getDefinitionForPortQualifiedResourceKey(key, ACCESS_TYPE_GATEWAY)
	if o == nil {
		delete(m.tlsRoutes, key)
		if sa == nil {
			return nil
		}
	} else {
		m.tlsRoutes[key] = o
		if sa == nil {
			log.Printf("Deleting redundant TLSRoute %s/%s", o.GetNamespace(), o.GetName())
			return m.clients.GetDynamicClient().Resource(resource.TlsRouteResource()).Namespace(o.GetNamespace()).Delete(context.Background(), o.GetName(), metav1.DeleteOptions{})
		}
	}
	return m.reconcile(sa)
}

func (m *SecuredAccessManager) CheckIngress(key string, ingress *networkingv1.Ingress) error {
	sa, ok := m.definitions[key]
	if ingress == nil {
		delete(m.ingresses, key)
		if !ok || m.actualAccessType(sa) != ACCESS_TYPE_INGRESS_NGINX {
			return nil
		}
	} else {
		m.ingresses[key] = ingress
		if !ok || m.actualAccessType(sa) != ACCESS_TYPE_INGRESS_NGINX {
			// delete this ingress as there is no corresponding securedaccess resource
			log.Printf("Deleting redundant Ingress %s/%s", ingress.Namespace, ingress.Name)
			return m.clients.GetKubeClient().NetworkingV1().Ingresses(ingress.Namespace).Delete(context.Background(), ingress.Name, metav1.DeleteOptions{})
		}
	}
	return m.reconcile(sa)
}

func (m *SecuredAccessManager) CheckGateway(key string, o *unstructured.Unstructured) error {
	if m.gatewayInit == nil {
		return nil
	}
	return m.gatewayInit()
}

func (m *SecuredAccessManager) CheckService(key string, svc *corev1.Service) error {
	if svc == nil {
		delete(m.services, key)
		if sa, ok := m.definitions[key]; ok {
			// recreate the service
			_, err := m.createService(sa)
			return err
		}
		return nil
	}
	sa, ok := m.definitions[key]
	if !ok {
		// delete this service as there is no corresponding securedaccess resource
		return m.clients.GetKubeClient().CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
	}
	m.services[key] = svc
	return m.reconcile(sa)
}

func updateType(spec *corev1.ServiceSpec, accessType string) bool {
	desired := serviceType(accessType)
	if spec.Type == desired {
		return false
	}
	spec.Type = desired
	return true
}

func updateSelector(spec *corev1.ServiceSpec, desired map[string]string) bool {
	if reflect.DeepEqual(spec.Selector, desired) {
		return false
	}
	spec.Selector = desired
	return true
}

func portsAsMap(ports []skupperv1alpha1.SecuredAccessPort) map[string]skupperv1alpha1.SecuredAccessPort {
	desired := map[string]skupperv1alpha1.SecuredAccessPort{}
	for _, port := range ports {
		desired[port.Name] = port
	}
	return desired
}

func updatePorts(spec *corev1.ServiceSpec, desired []skupperv1alpha1.SecuredAccessPort) bool {
	expected := toServicePorts(portsAsMap(desired))
	changed := false
	var ports []corev1.ServicePort
	for _, actual := range spec.Ports {
		if port, ok := expected[actual.Name]; ok {
			ports = append(ports, port)
			port.NodePort = actual.NodePort
			delete(expected, actual.Name)
			if actual != port {
				changed = true
			}
		} else {
			changed = true
		}
	}
	for _, port := range expected {
		ports = append(ports, port)
		changed = true
	}
	if changed {
		spec.Ports = ports
	}
	return changed
}

func toServicePorts(desired map[string]skupperv1alpha1.SecuredAccessPort) map[string]corev1.ServicePort {
	results := map[string]corev1.ServicePort{}
	for name, details := range desired {
		results[name] = corev1.ServicePort{
			Name:       name,
			Port:       int32(details.Port),
			TargetPort: intstr.IntOrString{IntVal: int32(details.TargetPort)},
			Protocol:   corev1.Protocol(details.Protocol),
		}
	}
	return results
}

func serviceType(accessType string) corev1.ServiceType {
	if accessType == ACCESS_TYPE_LOADBALANCER {
		return corev1.ServiceTypeLoadBalancer
	}
	if accessType == ACCESS_TYPE_NODEPORT {
		return corev1.ServiceTypeNodePort
	}
	return ""
}

func ownerReferences(sa *skupperv1alpha1.SecuredAccess) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			Kind:       "SecuredAccess",
			APIVersion: "skupper.io/v1alpha1",
			Name:       sa.Name,
			UID:        sa.ObjectMeta.UID,
		},
	}
}

func getHosts(sa *skupperv1alpha1.SecuredAccess) []string {
	hosts := map[string]string{}
	for _, endpoint := range sa.Status.Endpoints {
		if endpoint.Host != "" {
			hosts[endpoint.Host] = endpoint.Host
		}
	}
	var results []string
	for key, _ := range hosts {
		results = append(results, key)
	}
	results = append(results, sa.Name)
	results = append(results, sa.Name+"."+sa.Namespace)
	return results
}
