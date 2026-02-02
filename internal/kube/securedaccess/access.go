package securedaccess

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/skupperproject/skupper/internal/kube/certificates"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/resource"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type AccessType interface {
	RealiseAndResolve(access *skupperv2alpha1.SecuredAccess, service *corev1.Service) ([]skupperv2alpha1.Endpoint, error)
}

type ControllerContext interface {
	IsControlled(namespace string) bool
	SetLabels(namespace string, name string, kind string, labels map[string]string) bool
	SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool
	Namespace() string
	Name() string
	UID() string
}

type SecuredAccessManager struct {
	definitions        map[string]*skupperv2alpha1.SecuredAccess
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
	context            ControllerContext
	logger             *slog.Logger
}

func NewSecuredAccessManager(clients internalclient.Clients, certMgr certificates.CertificateManager, config *Config, context ControllerContext) *SecuredAccessManager {
	mgr := &SecuredAccessManager{
		definitions:        map[string]*skupperv2alpha1.SecuredAccess{},
		services:           map[string]*corev1.Service{},
		routes:             map[string]*routev1.Route{},
		ingresses:          map[string]*networkingv1.Ingress{},
		httpProxies:        map[string]*unstructured.Unstructured{},
		tlsRoutes:          map[string]*unstructured.Unstructured{},
		clients:            clients,
		certMgr:            certMgr,
		enabledAccessTypes: map[string]AccessType{},
		defaultAccessType:  config.getDefaultAccessType(clients),
		context:            context,
		logger:             slog.New(slog.Default().Handler()).With(slog.String("component", "kube.securedaccess.manager")),
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
				mgr.logger.Error("Failed to create gateway, gateway access type will not be enabled", slog.Any("error", err))
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

func (m *SecuredAccessManager) IsValidAccessType(accessType string) bool {
	_, ok := m.enabledAccessTypes[accessType]
	return ok
}

func (m *SecuredAccessManager) Ensure(namespace string, name string, spec skupperv2alpha1.SecuredAccessSpec, annotations map[string]string, refs []metav1.OwnerReference) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if current, ok := m.definitions[key]; ok {
		if current.ObjectMeta.Labels == nil {
			current.ObjectMeta.Labels = map[string]string{}
		}
		if current.ObjectMeta.Annotations == nil {
			current.ObjectMeta.Annotations = map[string]string{}
		}
		update := false
		if !reflect.DeepEqual(spec, current.Spec) {
			current.Spec = spec
			update = true
		}
		for k, v := range annotations {
			if current.ObjectMeta.Annotations[k] != v {
				current.ObjectMeta.Annotations[k] = v
				update = true
			}
		}
		if m.context != nil {
			if m.context.SetLabels(namespace, name, "SecuredAccess", current.ObjectMeta.Labels) {
				update = true
			}
			if m.context.SetAnnotations(namespace, name, "SecuredAccess", current.ObjectMeta.Annotations) {
				update = true
			}
		}

		if ensureOwnerReferences(&current.ObjectMeta, refs) {
			update = true
		}
		if !update {
			return nil
		}
		updated, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = updated
		return nil
	} else {
		sa := &skupperv2alpha1.SecuredAccess{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "SecuredAccess",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: refs,
				Annotations:     annotations,
				Labels:          map[string]string{},
			},
			Spec: spec,
		}
		if m.context != nil {
			if sa.ObjectMeta.Annotations == nil {
				sa.ObjectMeta.Annotations = map[string]string{}
			}
			m.context.SetLabels(namespace, name, "SecuredAccess", sa.ObjectMeta.Labels)
			m.context.SetAnnotations(namespace, name, "SecuredAccess", sa.ObjectMeta.Annotations)
		}
		created, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace).Create(context.Background(), sa, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = created
		return nil
	}

}

func (m *SecuredAccessManager) Delete(namespace string, name string) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if _, ok := m.definitions[key]; ok {
		if err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace).Delete(context.Background(), name, metav1.DeleteOptions{}); err != nil {
			return err
		}
		delete(m.definitions, key)
	}
	return nil
}

func (m *SecuredAccessManager) checkLabelsAndAnnotations(key string, current *skupperv2alpha1.SecuredAccess) *skupperv2alpha1.SecuredAccess {
	if m.context != nil {
		update := false
		if current.ObjectMeta.Labels == nil {
			current.ObjectMeta.Labels = map[string]string{}
		}
		if m.context.SetLabels(current.Namespace, current.Name, "SecuredAccess", current.ObjectMeta.Labels) {
			update = true
		}
		if current.ObjectMeta.Annotations == nil {
			current.ObjectMeta.Annotations = map[string]string{}
		}
		if m.context.SetAnnotations(current.Namespace, current.Name, "SecuredAccess", current.ObjectMeta.Annotations) {
			update = true
		}
		if update {
			updated, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(current.Namespace).Update(context.Background(), current, metav1.UpdateOptions{})
			if err != nil {
				m.logger.Error("Error updating labels/annotations for SecuredAccess", slog.String("key", key), slog.Any("error", err))
			} else {
				return updated
			}
		}
	}
	return current
}

func (m *SecuredAccessManager) SecuredAccessChanged(key string, current *skupperv2alpha1.SecuredAccess) error {
	current = m.checkLabelsAndAnnotations(key, current)
	m.definitions[key] = current
	return m.reconcile(current)
}

func (m *SecuredAccessManager) actualAccessType(sa *skupperv2alpha1.SecuredAccess) string {
	if sa.Spec.AccessType == "" {
		return m.defaultAccessType
	}
	return sa.Spec.AccessType
}

func (m *SecuredAccessManager) accessType(sa *skupperv2alpha1.SecuredAccess) AccessType {
	accessType := sa.Spec.AccessType
	if accessType == "" {
		accessType = m.defaultAccessType
	}
	if at, ok := m.enabledAccessTypes[accessType]; ok {
		return at
	}
	return newUnsupportedAccess(m)
}

func (m *SecuredAccessManager) reconcile(sa *skupperv2alpha1.SecuredAccess) error {
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
		if len(endpoints) > 0 {
			m.logger.Info("Resolved endpoints", slog.String("key", sa.Key()), slog.Any("endpoints", endpoints))
		}
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
func (m *SecuredAccessManager) updateStatus(sa *skupperv2alpha1.SecuredAccess) error {
	latest, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(sa.Namespace).UpdateStatus(context.TODO(), sa, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.definitions[latest.Key()] = latest
	return nil
}

func (m *SecuredAccessManager) checkCertificate(sa *skupperv2alpha1.SecuredAccess) error {
	if sa.Spec.Issuer == "" {
		return nil
	}
	name := sa.Spec.Certificate
	if name == "" {
		name = sa.Name
	}
	return m.certMgr.Ensure(sa.Namespace, name, sa.Spec.Issuer, sa.Name, getHosts(sa), false, true, ownerReferences(sa))
}

func (m *SecuredAccessManager) checkService(sa *skupperv2alpha1.SecuredAccess) (*corev1.Service, error) {
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
		if m.context != nil {
			if svc.ObjectMeta.Labels == nil {
				svc.ObjectMeta.Labels = map[string]string{}
			}
			if svc.ObjectMeta.Annotations == nil {
				svc.ObjectMeta.Annotations = map[string]string{}
			}
			if m.context.SetLabels(svc.Namespace, svc.Name, "Service", svc.ObjectMeta.Labels) {
				update = true
			}
			if m.context.SetAnnotations(svc.Namespace, svc.Name, "Service", svc.ObjectMeta.Annotations) {
				update = true
			}
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

func (m *SecuredAccessManager) createService(sa *skupperv2alpha1.SecuredAccess) (*corev1.Service, error) {
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
	updatePorts(&service.Spec, sa.Spec.Ports)
	if m.context != nil {
		m.context.SetLabels(sa.Namespace, sa.Name, "Service", service.ObjectMeta.Labels)
		m.context.SetAnnotations(sa.Namespace, sa.Name, "Service", service.ObjectMeta.Annotations)
	}
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

func (m *SecuredAccessManager) getDefinitionForPortQualifiedResourceKey(qualifiedKey string, expectedAccessType string) *skupperv2alpha1.SecuredAccess {
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
			if !canDelete(&route.ObjectMeta) {
				return nil
			}
			m.logger.Info("Deleting redundant route as no matching ServiceAccess definition found",
				slog.String("namespace", route.Namespace),
				slog.String("name", route.Name))
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
			m.logger.Info("Deleting redundant HttpProxy", slog.String("namespace", o.GetNamespace()), slog.String("name", o.GetName()))
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
			m.logger.Info("Deleting redundant TLSRoute", slog.String("namespace", o.GetNamespace()), slog.String("name", o.GetName()))
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
			if !canDelete(&ingress.ObjectMeta) {
				return nil
			}
			// delete this ingress as there is no corresponding securedaccess resource
			m.logger.Info("Deleting redundant Ingress", slog.String("namespace", ingress.Namespace), slog.String("name", ingress.Name))
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
		if !canDelete(&svc.ObjectMeta) {
			return nil
		}
		// delete this service as there is no corresponding securedaccess resource
		m.logger.Info("Deleting redundant service", slog.String("namespace", svc.Namespace), slog.String("name", svc.Name))
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

func portsAsMap(ports []skupperv2alpha1.SecuredAccessPort) map[string]skupperv2alpha1.SecuredAccessPort {
	desired := map[string]skupperv2alpha1.SecuredAccessPort{}
	for _, port := range ports {
		desired[port.Name] = port
	}
	return desired
}

func updatePorts(spec *corev1.ServiceSpec, desired []skupperv2alpha1.SecuredAccessPort) bool {
	expected := toServicePorts(portsAsMap(desired))
	changed := false
	var ports []corev1.ServicePort
	for _, actual := range spec.Ports {
		if port, ok := expected[actual.Name]; ok {
			if equivalentPorts(port, actual) {
				ports = append(ports, actual)
			} else {
				ports = append(ports, port)
				changed = true
			}
			delete(expected, actual.Name)
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

func equivalentPorts(desired corev1.ServicePort, actual corev1.ServicePort) bool {
	return desired.Name == actual.Name && desired.Port == actual.Port && equivalentTargetPorts(desired, actual) && equivalentProtocols(desired.Protocol, actual.Protocol)
}

func equivalentTargetPorts(desired corev1.ServicePort, actual corev1.ServicePort) bool {
	return desired.TargetPort == actual.TargetPort || (desired.TargetPort.IntVal == 0 && actual.TargetPort.IntVal == desired.Port)
}

func equivalentProtocols(desired corev1.Protocol, actual corev1.Protocol) bool {
	return desired == actual || (desired == "" && actual == corev1.ProtocolTCP)
}

func toServicePorts(desired map[string]skupperv2alpha1.SecuredAccessPort) map[string]corev1.ServicePort {
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

func ownerReferences(sa *skupperv2alpha1.SecuredAccess) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			Kind:       "SecuredAccess",
			APIVersion: "skupper.io/v2alpha1",
			Name:       sa.Name,
			UID:        sa.ObjectMeta.UID,
		},
	}
}

func getHosts(sa *skupperv2alpha1.SecuredAccess) []string {
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
	for _, h := range []string{sa.Name, sa.Name + "." + sa.Namespace} {
		if _, ok := hosts[h]; !ok {
			results = append(results, h)
		}
	}
	return results
}

func canDelete(obj *metav1.ObjectMeta) bool {
	return isOwned(obj) && hasSecuredAccessLabel(obj)
}

func isOwned(obj *metav1.ObjectMeta) bool {
	if obj.Annotations == nil {
		return false
	}
	_, ok := obj.Annotations["internal.skupper.io/controlled"]
	return ok
}

func hasSecuredAccessLabel(obj *metav1.ObjectMeta) bool {
	if obj.Labels == nil {
		return false
	}
	_, ok := obj.Labels["internal.skupper.io/secured-access"]
	return ok
}

func ensureOwnerReferences(meta *metav1.ObjectMeta, owners []metav1.OwnerReference) bool {
	byUID := make(map[string]int, len(owners))
	for iRef, ref := range owners {
		byUID[string(ref.UID)] = iRef
	}

	changed := false
	i := 0
	for _, ref := range meta.OwnerReferences {
		uid := string(ref.UID)
		if _, ok := byUID[uid]; !ok {
			changed = true
			continue
		}
		delete(byUID, uid)
		meta.OwnerReferences[i] = ref
		i++
	}
	if changed {
		meta.OwnerReferences = meta.OwnerReferences[:i]
	}
	for _, iRef := range byUID {
		meta.OwnerReferences = append(meta.OwnerReferences, owners[iRef])
		changed = true
	}

	return changed
}
