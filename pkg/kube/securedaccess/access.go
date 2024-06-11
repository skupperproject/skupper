package securedaccess

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/certificates"
)

type Factory interface {
	Ensure(namespace string, name string, spec skupperv1alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error
}

type AccessType interface {
	Realise(access *skupperv1alpha1.SecuredAccess) bool
	Resolve(access *skupperv1alpha1.SecuredAccess) bool
}

type SecuredAccessManager struct {
	definitions       map[string]*skupperv1alpha1.SecuredAccess
	services          map[string]*corev1.Service
	routes            map[string]*routev1.Route
	ingresses         map[string]*networkingv1.Ingress
	httpProxies       map[string]*unstructured.Unstructured
	clients           kube.Clients
	certMgr           certificates.CertificateManager
	defaultAccessType string
}

func NewSecuredAccessManager(clients kube.Clients, certMgr certificates.CertificateManager, defaultAccessType string) *SecuredAccessManager {
	return &SecuredAccessManager{
		definitions:       map[string]*skupperv1alpha1.SecuredAccess{},
		services:          map[string]*corev1.Service{},
		routes:            map[string]*routev1.Route{},
		ingresses:         map[string]*networkingv1.Ingress{},
		httpProxies:       map[string]*unstructured.Unstructured{},
		clients:           clients,
		certMgr:           certMgr,
		defaultAccessType: getAccessType(defaultAccessType, clients),
	}
}

func (m *SecuredAccessManager) Ensure(namespace string, name string, spec skupperv1alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error {
	log.Printf("SecuredAccess.Ensure(%s, %s)", namespace, name)
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

func serviceKey(svc *corev1.Service) string {
	return fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
}

func (m *SecuredAccessManager) SecuredAccessChanged(key string, current *skupperv1alpha1.SecuredAccess) error {
	log.Printf("Checking SecuredAccess %s", key)
	if original, ok := m.definitions[key]; ok {
		if original.Spec.AccessType != current.Spec.AccessType {
			//TODO: access type changed, delete any resources that may exist from old access type
		}
	}
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
	if accessType == ACCESS_TYPE_ROUTE {
		return newRouteAccess(m)
	} else if accessType == ACCESS_TYPE_LOADBALANCER {
		return newLoadbalancerAccess(m)
	} else if accessType == ACCESS_TYPE_LOCAL {
		return newLocalAccess(m)
	} else {
		return newUnsupportedAccess(m)
	}
}

func (m *SecuredAccessManager) reconcile(sa *skupperv1alpha1.SecuredAccess) error {
	if err := m.checkService(sa); err != nil {
		return err
	}
	updated := false
	if m.accessType(sa).Realise(sa) {
		updated = true
	}
	if m.accessType(sa).Resolve(sa) {
		updated = true
	}
	if err := m.checkCertificate(sa); err != nil {
		return err
	}

	if !updated {
		return nil
	}
	log.Printf("Updating SecuredAccess status for %s", sa.Key())
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

func (m *SecuredAccessManager) checkService(sa *skupperv1alpha1.SecuredAccess) error {
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
			return nil
		}
		updated, err := m.clients.GetKubeClient().CoreV1().Services(sa.Namespace).Update(context.Background(), svc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.services[key] = updated
		return nil
	}
	return m.createService(sa)
}

func (m *SecuredAccessManager) createService(sa *skupperv1alpha1.SecuredAccess) error {
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
		return err
	}
	m.services[key] = created
	return nil
}

func (m *SecuredAccessManager) SecuredAccessDeleted(key string) error {
	if _, ok := m.definitions[key]; ok {
		//delete any resources created for this secured access instance (or rely on owner references for that?)
		delete(m.definitions, key)
		delete(m.services, key)
	}
	return nil
}

func (m *SecuredAccessManager) ensureRoute(namespace string, route *routev1.Route) (error, *routev1.Route) {
	key := fmt.Sprintf("%s/%s", namespace, route.Name)
	if existing, ok := m.routes[key]; ok {
		if reflect.DeepEqual(existing.Spec, route.Spec) {
			return nil, existing
		}
		existing.Spec = route.Spec
		updated, err := m.clients.GetRouteClient().Routes(namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
		if err != nil {
			return err, nil
		}
		m.routes[key] = updated
		return nil, updated
	}
	created, err := m.clients.GetRouteClient().Routes(namespace).Create(context.Background(), route, metav1.CreateOptions{})
	if err != nil {
		return err, nil
	}
	m.routes[key] = created
	return nil, created
}

func (m *SecuredAccessManager) RecoverRoute(route *routev1.Route) {
	key := fmt.Sprintf("%s/%s", route.Namespace, route.Name)
	m.routes[key] = route
}

func (m *SecuredAccessManager) RecoverHttpProxy(o *unstructured.Unstructured) {
	key := fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())
	m.httpProxies[key] = o
}

func (m *SecuredAccessManager) RecoverIngress(ingress *networkingv1.Ingress) {
	key := fmt.Sprintf("%s/%s", ingress.Namespace, ingress.Name)
	m.ingresses[key] = ingress
}

func (m *SecuredAccessManager) RecoverService(svc *corev1.Service) {
	key := fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
	m.services[key] = svc
}

func (m *SecuredAccessManager) CheckRoute(routeKey string, route *routev1.Route) error {
	if route == nil {
		delete(m.routes, routeKey)
		// TODO: should it be recreated?
		return nil
	}
	port := route.Spec.Port.TargetPort.String()
	key, matched := strings.CutSuffix(routeKey, port)
	if !matched {
		log.Printf("Malformed Route name %s for SecuredAccess, expected suffix of %s", routeKey, port)
		return nil
	}
	sa, ok := m.definitions[key]
	var latest *routev1.Route
	if ok && sa.Spec.AccessType == "route" {
		for _, p := range sa.Spec.Ports {
			if p.Name == port {
				latest = desiredRouteForPort(sa, p)
			}
		}
	}
	if latest == nil {
		// delete this route instance
		name := strings.Split(routeKey, "/")[0]
		return m.clients.GetRouteClient().Routes(route.Namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	}

	err, latest := m.ensureRoute(sa.Namespace, latest)
	if err != nil {
		return err
	}

	// ensure status of access object has correct urls
	update := false
	if latest.Spec.Host != "" {
		desiredEndpoint := &skupperv1alpha1.Endpoint{
			Name:  port,
			Host:  latest.Spec.Host,
			Port:  "443",
			Group: latest.Name,
		}
		if sa.Status.UpdateEndpoint(desiredEndpoint) {
			update = true
		}
	}
	if !update {
		return nil
	}
	updated, err := m.clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(sa.Namespace).Update(context.Background(), sa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.definitions[key] = updated
	return nil
}

func (m *SecuredAccessManager) CheckHttpProxy(key string, o *unstructured.Unstructured) error {
	return nil
}

func (m *SecuredAccessManager) CheckIngress(key string, ingress *networkingv1.Ingress) error {
	// there will be one ingress resource for each securedaccess resource, so the key can be assumed to be the same
	return nil
}

func (m *SecuredAccessManager) CheckService(key string, svc *corev1.Service) error {
	if svc == nil {
		delete(m.services, key)
		if sa, ok := m.definitions[key]; ok {
			// recreate the service
			return m.createService(sa)
		}
		return nil
	}
	sa, ok := m.definitions[key]
	if !ok {
		// delete this service as there is no corresponding securedaccess resource
		return m.clients.GetKubeClient().CoreV1().Services(svc.Namespace).Delete(context.Background(), svc.Name, metav1.DeleteOptions{})
	}
	m.services[key] = svc
	return m.checkService(sa)
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

const ACCESS_TYPE_LOADBALANCER = "loadbalancer"
const ACCESS_TYPE_ROUTE = "route"
const ACCESS_TYPE_NODEPORT = "nodeport"
const ACCESS_TYPE_LOCAL = "local"

func DefaultAccessType(clients kube.Clients) string {
	if clients.GetRouteClient() != nil {
		return ACCESS_TYPE_ROUTE
	}
	return ACCESS_TYPE_LOADBALANCER
}

func GetAccessTypeFromEnv() string {
	return os.Getenv("SKUPPER_DEFAULT_ACCESS_TYPE")
}

func getAccessType(accessType string, clients kube.Clients) string {
	if accessType == "" {
		return DefaultAccessType(clients)
	}
	return accessType
}
