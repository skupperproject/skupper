package securedaccess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	skuppertypes "github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/kube/resource"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

const accessLabel = "internal.skupper.io/secured-access"

type accessSnapshot struct {
	namespace, name    string
	access             *skupperv2alpha1.SecuredAccess
	service            *corev1.Service
	ingress            *networkingv1.Ingress
	routes             map[string]*routev1.Route
	proxies, tlsRoutes map[string]*unstructured.Unstructured
	certificate        accessCertificateSnapshot
	policy             *labels.LabelsAndAnnotations
	config             Config
	accessType         string
	routeAvailable     bool
	gatewayAvailable   bool
	gateway            *unstructured.Unstructured
	gatewayNamespace   string
}

// Actions contain final objects, never closures over mutable manager state.
// Status timestamps and API results are execution facts, not desired state.
type accessAction struct {
	verb, kind string
	object     runtime.Object
	observed   *unstructured.Unstructured // Copied snapshot for SSA success receipts.
}
type accessPlan struct {
	actions       []accessAction
	access        *skupperv2alpha1.SecuredAccess
	endpoints     []skupperv2alpha1.Endpoint
	validationErr error
	retryErr      error
}
type accessResult struct {
	verb, kind, namespace, name string
	duration                    time.Duration
	err                         error
	conditions                  []metav1.Condition
	unchanged                   bool
}

func (m *SecuredAccessManager) Kind() string { return "AccessScope" }
func (m *SecuredAccessManager) Describe(e watchers.ResourceChange) string {
	return m.Kind() + " " + e.Key
}
func (m *SecuredAccessManager) Handle(e watchers.ResourceChange) (err error) {
	phase := "key"
	var results []accessResult
	defer func() { m.logResults("SecuredAccess", e.Key, phase, results, err) }()
	ns, _, err := cache.SplitMetaNamespaceKey(e.Key)
	if err != nil || !m.isControlled(ns) {
		return err
	}
	phase = "snapshot"
	s, err := m.snapshot(e.Key)
	if err != nil {
		return err
	}
	phase = "plan"
	p := planAccess(s)
	phase = "execute"
	results, err = m.executeAccess(p)
	return err
}

func (m *SecuredAccessManager) snapshot(key string) (accessSnapshot, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	s := accessSnapshot{namespace: ns, name: name, config: m.config, routes: map[string]*routev1.Route{}, proxies: map[string]*unstructured.Unstructured{}, tlsRoutes: map[string]*unstructured.Unstructured{}}
	if err != nil {
		return s, err
	}
	w := m.watcher
	s.routeAvailable = w.Routes != nil
	s.gatewayAvailable = w.Gateways != nil && w.TLSRoutes != nil
	sa, err := w.Accesses.Get(key)
	if err != nil {
		return s, err
	}
	s.access = sa.DeepCopy()
	if sa != nil {
		s.accessType = m.actualAccessType(sa)
		if target := CertificateTarget(sa); target != "" {
			s.certificate, err = m.certificates.snapshot(ns + "/" + target)
			if err != nil {
				return s, err
			}
		}
	}
	if w.Services != nil {
		observed, err := w.Services.Get(key)
		if err != nil {
			return s, err
		}
		s.service = observed.DeepCopy()
	}
	if w.Ingresses != nil {
		observed, err := w.Ingresses.Get(key)
		if err != nil {
			return s, err
		}
		s.ingress = observed.DeepCopy()
	}
	portNames := map[string]bool{}
	if sa != nil {
		for _, port := range sa.Spec.Ports {
			portNames[name+"-"+port.Name] = true
		}
	}
	if w.Routes != nil {
		for _, o := range w.Routes.List() {
			if o.Namespace == ns && ((s.accessType == ACCESS_TYPE_ROUTE && portNames[o.Name]) || accessOwner(o, name) != "") {
				s.routes[o.Name] = o.DeepCopy()
			}
		}
	}
	if w.HTTPProxies != nil {
		for _, o := range w.HTTPProxies.List() {
			if o.GetNamespace() == ns && ((s.accessType == ACCESS_TYPE_CONTOUR_HTTP_PROXY && portNames[o.GetName()]) || accessOwner(o, name) != "") {
				s.proxies[o.GetName()] = o.DeepCopy()
			}
		}
	}
	if w.TLSRoutes != nil {
		for _, o := range w.TLSRoutes.List() {
			if o.GetNamespace() == ns && ((s.accessType == ACCESS_TYPE_GATEWAY && portNames[o.GetName()]) || accessOwner(o, name) != "") {
				s.tlsRoutes[o.GetName()] = o.DeepCopy()
			}
		}
	}
	if m.context != nil {
		s.policy = m.context.SnapshotMetadata(ns)
		s.gatewayNamespace = m.context.Namespace()
	}
	if w.Gateways != nil {
		g, err := w.Gateways.Get(s.gatewayNamespace + "/skupper")
		if err != nil {
			return s, err
		}
		s.gateway = g.DeepCopy()
	}
	return s, nil
}

func exposureMetadata(sa *skupperv2alpha1.SecuredAccess, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Namespace: sa.Namespace, Name: name, OwnerReferences: ownerReferences(sa), Labels: map[string]string{accessLabel: "true"}, Annotations: map[string]string{skuppertypes.ControlledQualifier: "true"}}
}
func accessReference(ref metav1.OwnerReference) bool {
	return ref.APIVersion == "skupper.io/v2alpha1" && ref.Kind == "SecuredAccess"
}

// accessOwner returns this scope's owner UID, or "" if another SecuredAccess or
// a foreign controller claims obj. Other non-controller owners are allowed.
func accessOwner(obj metav1.Object, name string) types.UID {
	var uid types.UID
	for _, ref := range obj.GetOwnerReferences() {
		if accessReference(ref) && ref.Name == name && uid == "" {
			uid = ref.UID
		} else if accessReference(ref) || ptr.Deref(ref.Controller, false) {
			return ""
		}
	}
	return uid
}

func applyPolicy(policy *labels.LabelsAndAnnotations, kind string, obj metav1.Object) metav1.ObjectMeta {
	if policy == nil {
		return metav1.ObjectMeta{}
	}
	// keep old capitalization of TLSRoute kind for evaluating metadata
	// templates
	if kind == "TLSRoute" {
		kind = "TlsRoute"
	}
	metadata := metav1.ObjectMeta{Labels: obj.GetLabels(), Annotations: obj.GetAnnotations()}
	authored := policy.ApplyObjectMetadata(obj.GetNamespace(), obj.GetName(), kind, &metadata)
	if len(metadata.Labels) > 0 || obj.GetLabels() != nil {
		obj.SetLabels(metadata.Labels)
	}
	if len(metadata.Annotations) > 0 || obj.GetAnnotations() != nil {
		obj.SetAnnotations(metadata.Annotations)
	}
	return authored
}

// Build the apply request independently of the full observed policy input.
// Policy may read external labels, but only its outputs belong in the request.
func applyConfiguration(observed, desired *unstructured.Unstructured, policy *labels.LabelsAndAnnotations, kind string) (*unstructured.Unstructured, error) {
	context := desired.DeepCopy()
	if observed != nil {
		context = observed.DeepCopy()
	}
	values := context.GetLabels()
	if values == nil {
		values = map[string]string{}
	}
	maps.Copy(values, desired.GetLabels())
	context.SetLabels(values)
	values = context.GetAnnotations()
	if values == nil {
		values = map[string]string{}
	}
	maps.Copy(values, desired.GetAnnotations())
	context.SetAnnotations(values)
	authored := applyPolicy(policy, kind, context)
	fields, err := resource.AppliedFields(context, applyFieldManager, desired.GetAPIVersion())
	if err != nil {
		return nil, err
	}
	result := desired.DeepCopy()
	result.SetResourceVersion(context.GetResourceVersion())
	result.SetLabels(applyMetadataValues(context.GetLabels(), desired.GetLabels(), authored.Labels, fields, "labels"))
	result.SetAnnotations(applyMetadataValues(context.GetAnnotations(), desired.GetAnnotations(), authored.Annotations, fields, "annotations"))
	return result, nil
}

func applyMetadataValues(observed, builtins, policy map[string]string, fields *fieldpath.Set, section string) map[string]string {
	var result map[string]string
	for key, value := range observed {
		_, builtin := builtins[key]
		_, authored := policy[key]
		if builtin || authored || fields.Has(fieldpath.MakePathOrDie("metadata", section, key)) {
			if result == nil {
				result = map[string]string{}
			}
			result[key] = value
		}
	}
	return result
}

// planOutput guards ownership before deriving a mutation. Metadata is evaluated
// from the observed object so selectors chain, then compared in its final form.
func (p *accessPlan) planOutput(s accessSnapshot, kind string, observed, desired runtime.Object) {
	dm, _ := meta.Accessor(desired)
	if observed != nil && !reflect.ValueOf(observed).IsNil() {
		om, _ := meta.Accessor(observed)
		if uid := accessOwner(om, s.name); uid == "" || uid != s.access.UID {
			p.validationErr = errors.Join(p.validationErr, fmt.Errorf("%s %q has conflicting ownership", kind, dm.GetName()))
			return
		}
		if om.GetDeletionTimestamp() != nil {
			return
		}
		if kind == "TLSRoute" {
			p.planApply(s, kind, observed.(*unstructured.Unstructured), desired.(*unstructured.Unstructured))
			return
		}
		// Preserve external metadata and API identity; only built-ins and policy
		// values belong to this writer. Removing a template retains old values,
		// matching the existing metadata registry contract.
		merged := observed.DeepCopyObject()
		mm, _ := meta.Accessor(merged)
		ls, as := mm.GetLabels(), mm.GetAnnotations()
		if ls == nil {
			ls = map[string]string{}
		}
		if as == nil {
			as = map[string]string{}
		}
		for k, v := range dm.GetLabels() {
			ls[k] = v
		}
		for k, v := range dm.GetAnnotations() {
			as[k] = v
		}
		mm.SetLabels(ls)
		mm.SetAnnotations(as)
		switch d := desired.(type) {
		case *corev1.Service:
			o := merged.(*corev1.Service)
			o.Spec.Selector = d.Spec.Selector
			o.Spec.Type = d.Spec.Type
			updatePorts(&o.Spec, s.access.Spec.Ports)
			if o.Spec.Type == corev1.ServiceTypeClusterIP {
				for i := range o.Spec.Ports {
					o.Spec.Ports[i].NodePort = 0
				}
				o.Spec.HealthCheckNodePort = 0
				o.Spec.ExternalTrafficPolicy = ""
			}
			if o.Spec.Type != corev1.ServiceTypeLoadBalancer {
				o.Spec.HealthCheckNodePort = 0
				o.Spec.AllocateLoadBalancerNodePorts = nil
				o.Spec.LoadBalancerClass = nil
			}
		case *networkingv1.Ingress:
			merged.(*networkingv1.Ingress).Spec = d.Spec
		case *routev1.Route:
			o := merged.(*routev1.Route)
			if !equivalentRoute(o, d) {
				host := o.Spec.Host
				o.Spec = d.Spec
				if d.Spec.Host == "" {
					o.Spec.Host = host
				}
			}
		case *unstructured.Unstructured:
			merged.(*unstructured.Unstructured).Object["spec"] = runtime.DeepCopyJSONValue(d.Object["spec"])
		}
		applyPolicy(s.policy, kind, mm)
		if !reflect.DeepEqual(observed, merged) {
			p.actions = append(p.actions, accessAction{verb: "update", kind: kind, object: merged})
		}
		return
	}
	if kind == "TLSRoute" {
		p.planApply(s, kind, nil, desired.(*unstructured.Unstructured))
		return
	}
	applyPolicy(s.policy, kind, dm)
	p.actions = append(p.actions, accessAction{verb: "create", kind: kind, object: desired})
}

func (p *accessPlan) planApply(s accessSnapshot, kind string, observed, desired *unstructured.Unstructured) {
	configuration, err := applyConfiguration(observed, desired, s.policy, kind)
	if err != nil {
		p.retryErr = errors.Join(p.retryErr, err)
		return
	}
	verb := "create"
	if observed != nil {
		verb = "update"
	}
	p.actions = append(p.actions, accessAction{verb: verb, kind: kind, object: configuration, observed: observed})
}

func planAccess(s accessSnapshot) accessPlan {
	p := accessPlan{access: s.access.DeepCopy(), validationErr: validateAccessCertificate(s.certificate)}
	sa := s.access
	if sa != nil && sa.DeletionTimestamp != nil {
		return accessPlan{}
	}
	desiredNames := map[string]bool{}
	if sa != nil {
		if !s.config.isValidAccessType(s.accessType) || (s.accessType == ACCESS_TYPE_ROUTE && !s.routeAvailable) {
			p.validationErr = errors.Join(p.validationErr, errors.New("unsupported access type"))
			// Do not write outputs or clean up with invalid intent - keep
			// existing exposure in place until fixed.
			return p
		}
		svc := &corev1.Service{ObjectMeta: exposureMetadata(sa, sa.Name), Spec: corev1.ServiceSpec{Selector: sa.Spec.Selector, Type: serviceType(s.accessType)}}
		updatePorts(&svc.Spec, sa.Spec.Ports)
		p.planOutput(s, "Service", s.service, svc)
		// Endpoints requiring allocations/status wait for ordinary observations.
		switch s.accessType {
		case ACCESS_TYPE_LOCAL:
			for _, port := range sa.Spec.Ports {
				p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: port.Name, Host: sa.Name + "." + sa.Namespace, Port: strconv.Itoa(port.Port)})
			}
		case ACCESS_TYPE_NODEPORT:
			if s.service != nil {
				for _, port := range s.service.Spec.Ports {
					if hasPort(sa, port.Name) && port.NodePort != 0 {
						p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: port.Name, Host: s.config.ClusterHost, Port: strconv.Itoa(int(port.NodePort))})
					}
				}
				// Publish only once every spec port's allocation is observed.
				if len(p.endpoints) != len(sa.Spec.Ports) {
					p.endpoints = nil
				}
			}
		case ACCESS_TYPE_LOADBALANCER:
			if s.service != nil {
				for _, addr := range s.service.Status.LoadBalancer.Ingress {
					host := addr.IP
					if host == "" {
						host = addr.Hostname
					}
					if host == "" {
						continue
					}
					for _, port := range sa.Spec.Ports {
						p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: port.Name, Host: host, Port: strconv.Itoa(port.Port)})
					}
				}
			}
		case ACCESS_TYPE_INGRESS, ACCESS_TYPE_INGRESS_NGINX:
			domain := s.config.IngressDomain
			qualified := domain != ""
			if domain == "" && s.ingress != nil {
				domain = deduceDomainForIngressHosts(s.ingress)
				qualified = domain != ""
			}
			domain = qualify(sa.Namespace, domain)
			d := toIngress(domain, sa)
			if class := ingressClassNameForDesiredIngress(s.accessType == ACCESS_TYPE_INGRESS_NGINX, s.config.IngressClassName, sa.Spec.Settings); class != "" {
				d.Spec.IngressClassName = &class
			}
			if s.accessType == ACCESS_TYPE_INGRESS_NGINX {
				addNginxIngressAnnotations(d.Annotations)
			}
			p.planOutput(s, "Ingress", s.ingress, d)
			desiredNames["Ingress/"+d.Name] = true
			if qualified {
				for _, r := range d.Spec.Rules {
					p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: prefix(r.Host), Host: r.Host, Port: "443"})
				}
			}
		case ACCESS_TYPE_ROUTE:
			for _, port := range sa.Spec.Ports {
				d := desiredRouteForPort(sa, port)
				p.planOutput(s, "Route", s.routes[d.Name], d)
				desiredNames["Route/"+d.Name] = true
				if o := s.routes[d.Name]; o != nil {
					for _, r := range o.Status.Ingress {
						p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: port.Name, Host: r.Host, Port: "443"})
					}
				}
			}
		case ACCESS_TYPE_CONTOUR_HTTP_PROXY:
			for _, proxy := range desiredHttpProxies(qualify(sa.Namespace, s.config.HttpProxyDomain), sa) {
				d := &unstructured.Unstructured{}
				d.SetGroupVersionKind(httpProxyGVK)
				d.SetName(proxy.Name)
				d.SetNamespace(sa.Namespace)
				d.SetOwnerReferences(ownerReferences(sa))
				d.SetLabels(map[string]string{accessLabel: "true"})
				d.SetAnnotations(map[string]string{skuppertypes.ControlledQualifier: "true"})
				_ = proxy.writeToContourProxy(d)
				p.planOutput(s, "HTTPProxy", s.proxies[proxy.Name], d)
				desiredNames["HTTPProxy/"+proxy.Name] = true
				p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: strings.TrimPrefix(proxy.Name, sa.Name+"-"), Host: proxy.Host, Port: "443"})
			}
		case ACCESS_TYPE_GATEWAY:
			for _, port := range sa.Spec.Ports {
				desiredNames["TLSRoute/"+sa.Name+"-"+port.Name] = true
			}
			// Nil caches mean startup discovery found no usable API (absent, denied or
			// failed); watches are not re-registered without a restart.
			if !s.gatewayAvailable {
				p.validationErr = errors.Join(p.validationErr, errors.New("Gateway API (Gateway and TLSRoute) is unavailable; restart the controller after making both resources available"))
				break
			}
			if s.gateway == nil || s.gateway.GetDeletionTimestamp() != nil {
				p.validationErr = errors.Join(p.validationErr, errors.New("Gateway not yet available"))
				break
			}
			domain := s.config.GatewayDomain
			if domain == "" && s.gateway != nil {
				domain = getBaseDomain(s.gateway)
			}
			if domain == "" {
				p.validationErr = errors.Join(p.validationErr, errors.New("Gateway base domain not yet resolved"))
			} else {
				for _, port := range sa.Spec.Ports {
					d := desiredTLSRoute(sa, port, s.gatewayNamespace, domain)
					p.planOutput(s, "TLSRoute", s.tlsRoutes[d.GetName()], d)
					desiredNames["TLSRoute/"+d.GetName()] = true
					p.endpoints = append(p.endpoints, skupperv2alpha1.Endpoint{Name: port.Name, Host: qualify(d.GetName()+"."+sa.Namespace, domain), Port: strconv.Itoa(s.config.GatewayPort)})
				}
			}
		default:
			p.validationErr = errors.Join(p.validationErr, errors.New("unsupported access type"))
		}
		if p.validationErr != nil || p.retryErr != nil {
			p.endpoints = nil
		}
	}
	// Discovery is independent of a definition's existence and resource labels.
	var outputs []accessAction
	if s.service != nil && sa == nil {
		outputs = append(outputs, accessAction{kind: "Service", object: s.service})
	}
	if s.ingress != nil {
		outputs = append(outputs, accessAction{kind: "Ingress", object: s.ingress})
	}
	for _, o := range s.routes {
		outputs = append(outputs, accessAction{kind: "Route", object: o})
	}
	for _, o := range s.proxies {
		outputs = append(outputs, accessAction{kind: "HTTPProxy", object: o})
	}
	for _, o := range s.tlsRoutes {
		outputs = append(outputs, accessAction{kind: "TLSRoute", object: o})
	}
	cleanup := outputs[:0]
	for _, o := range outputs {
		om, _ := meta.Accessor(o.object)
		if !desiredNames[o.kind+"/"+om.GetName()] && accessOwner(om, s.name) != "" && om.GetDeletionTimestamp() == nil {
			o.verb = "delete"
			cleanup = append(cleanup, o)
		}
	}
	sort.Slice(cleanup, func(i, j int) bool {
		a, _ := meta.Accessor(cleanup[i].object)
		b, _ := meta.Accessor(cleanup[j].object)
		return cleanup[i].kind+"/"+a.GetName() < cleanup[j].kind+"/"+b.GetName()
	})
	p.actions = append(p.actions, cleanup...)
	if sa != nil {
		d := sa.DeepCopy()
		applyPolicy(s.policy, "SecuredAccess", d)
		if !reflect.DeepEqual(d, sa) {
			p.actions = append(p.actions, accessAction{verb: "update", kind: "SecuredAccess", object: d})
		}
	}
	if p.retryErr != nil {
		// An invalid apply configuration cannot authorize partial output writes.
		p.actions = nil
	}
	return p
}

// staleObservation reports writes refused because the plan used an older cache
// than the server; status from that plan is not published and the retry replans.
func staleObservation(err error) bool {
	if apierrors.IsAlreadyExists(err) {
		return true
	}
	return apierrors.IsConflict(err) && !apierrors.HasStatusCause(err, metav1.CauseTypeFieldManagerConflict)
}

func (m *SecuredAccessManager) executeAccess(p accessPlan) ([]accessResult, error) {
	var results []accessResult
	executionErr := p.retryErr
	// Cleanup deletes and Access metadata follow every exposure write, so their
	// failure leaves the planned endpoints realized.
	var exposureErr error
	stale := false
	for _, a := range p.actions {
		var result *accessResult
		var err error
		if a.kind == "TLSRoute" && a.verb != "delete" {
			result, err = m.executeCachedApply(a)
		} else {
			start := time.Now()
			unchanged, actionErr := m.executeAction(a)
			err = actionErr
			om, _ := meta.Accessor(a.object)
			result = &accessResult{verb: a.verb, kind: a.kind, namespace: om.GetNamespace(), name: om.GetName(), duration: time.Since(start), err: err, unchanged: unchanged}
		}
		if result != nil {
			results = append(results, *result)
		}
		executionErr = errors.Join(executionErr, err)
		if err != nil {
			if staleObservation(err) {
				stale = true
			} else if a.verb != "delete" && a.kind != "SecuredAccess" {
				exposureErr = err
			}
			break // A failed action invalidates downstream assumptions.
		}
		if sa, ok := a.object.(*skupperv2alpha1.SecuredAccess); ok {
			p.access = sa.DeepCopy()
		}
	}
	if p.access != nil && !stale {
		sa := p.access.DeepCopy()
		endpoints := p.endpoints
		if p.retryErr != nil || exposureErr != nil {
			endpoints = nil
		}
		changed := sa.SetResolved(endpoints)
		changed = sa.SetConfigured(errors.Join(p.validationErr, p.retryErr, exposureErr)) || changed
		if changed {
			start := time.Now()
			written, err := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(sa.Namespace).UpdateStatus(context.Background(), sa, metav1.UpdateOptions{})
			result := accessResult{verb: "status", kind: "SecuredAccess", namespace: sa.Namespace, name: sa.Name, duration: time.Since(start), err: err}
			if err == nil {
				result.conditions = append([]metav1.Condition(nil), written.Status.Conditions...)
			}
			results = append(results, result)
			executionErr = errors.Join(executionErr, err)
		}
	}
	return results, executionErr
}

// unchanged reports deletion of an already absent output.
func (m *SecuredAccessManager) executeAction(a accessAction) (unchanged bool, err error) {
	ctx := context.Background()
	om, _ := meta.Accessor(a.object)
	ns, name := om.GetNamespace(), om.GetName()
	uid, rv := om.GetUID(), om.GetResourceVersion()
	del := metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid, ResourceVersion: &rv}}
	switch o := a.object.(type) {
	case *corev1.Service:
		c := m.clients.GetKubeClient().CoreV1().Services(ns)
		switch a.verb {
		case "create":
			_, err = c.Create(ctx, o, metav1.CreateOptions{})
		case "update":
			_, err = c.Update(ctx, o, metav1.UpdateOptions{})
		case "delete":
			err = c.Delete(ctx, name, del)
		}
	case *networkingv1.Ingress:
		c := m.clients.GetKubeClient().NetworkingV1().Ingresses(ns)
		switch a.verb {
		case "create":
			_, err = c.Create(ctx, o, metav1.CreateOptions{})
		case "update":
			_, err = c.Update(ctx, o, metav1.UpdateOptions{})
		case "delete":
			err = c.Delete(ctx, name, del)
		}
	case *routev1.Route:
		c := m.clients.GetRouteClient().Routes(ns)
		switch a.verb {
		case "create":
			_, err = c.Create(ctx, o, metav1.CreateOptions{})
		case "update":
			_, err = c.Update(ctx, o, metav1.UpdateOptions{})
		case "delete":
			err = c.Delete(ctx, name, del)
		}
	case *skupperv2alpha1.Certificate:
		c := m.clients.GetSkupperClient().SkupperV2alpha1().Certificates(ns)
		switch a.verb {
		case "create":
			_, err = c.Create(ctx, o, metav1.CreateOptions{})
		case "update":
			_, err = c.Update(ctx, o, metav1.UpdateOptions{})
		case "delete":
			err = c.Delete(ctx, name, del)
		}
	case *skupperv2alpha1.SecuredAccess:
		updated, e := m.clients.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(ns).Update(ctx, o, metav1.UpdateOptions{})
		err = e
		if err == nil {
			*o = *updated
		}
	case *unstructured.Unstructured:
		gvr := resource.TlsRouteResource()
		if a.kind == "HTTPProxy" {
			gvr = httpProxyResource
		}
		c := m.clients.GetDynamicClient().Resource(gvr).Namespace(ns)
		if a.verb == "delete" {
			err = c.Delete(ctx, name, del)
		} else if a.kind == "HTTPProxy" {
			if a.verb == "create" {
				_, err = c.Create(ctx, o, metav1.CreateOptions{})
			} else {
				_, err = c.Update(ctx, o, metav1.UpdateOptions{})
			}
		} else {
			return false, fmt.Errorf("unsupported access action %s for %s", a.verb, a.kind)
		}
	default:
		return false, fmt.Errorf("unknown access action object %T", a.object)
	}
	if a.verb == "delete" && apierrors.IsNotFound(err) {
		return true, nil
	}
	return unchanged, err
}

const applyFieldManager = "skupper-controller"

// executeCachedApply returns no action result for a receipt hit: no API request
// was attempted. Only successful real responses can establish another receipt.
func (m *SecuredAccessManager) executeCachedApply(a accessAction) (*accessResult, error) {
	intent := a.object.(*unstructured.Unstructured).DeepCopy()
	intent.SetResourceVersion("") // Execution guard, not authored intent.
	applied, err := m.applyCache.Applied(a.observed, intent)
	if err != nil || applied {
		return nil, err
	}
	start := time.Now()
	written, err := m.applyOutput(a)
	unchanged := err == nil && a.verb == "update" && a.observed.GetResourceVersion() == written.GetResourceVersion()
	if err == nil {
		err = m.applyCache.Remember(written, intent)
	}
	result := &accessResult{verb: a.verb, kind: a.kind, namespace: intent.GetNamespace(), name: intent.GetName(), duration: time.Since(start), err: err, unchanged: unchanged}
	return result, err
}

// applyOutput returns the real SSA response so callers can record its revision.
// Authorization and the authored projection are established during planning.
func (m *SecuredAccessManager) applyOutput(a accessAction) (*unstructured.Unstructured, error) {
	o := a.object.(*unstructured.Unstructured)
	gvr := resource.TlsRouteResource()
	if a.kind == "Gateway" {
		gvr = resource.GatewayResource()
	}
	// Owned, resourceVersion-pinned updates force conflicts; creates never do.
	d := &unstructured.Unstructured{Object: map[string]interface{}{"apiVersion": o.GetAPIVersion(), "kind": o.GetKind(), "spec": runtime.DeepCopyJSONValue(o.Object["spec"])}}
	d.SetName(o.GetName())
	d.SetNamespace(o.GetNamespace())
	d.SetResourceVersion(o.GetResourceVersion())
	d.SetLabels(o.GetLabels())
	d.SetAnnotations(o.GetAnnotations())
	d.SetOwnerReferences(o.GetOwnerReferences())
	data, err := json.Marshal(d)
	if err != nil {
		return nil, err
	}
	return m.clients.GetDynamicClient().Resource(gvr).Namespace(o.GetNamespace()).Patch(context.Background(), o.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{FieldManager: applyFieldManager, Force: ptr.To(a.verb == "update")})
}

func (m *SecuredAccessManager) logResults(scope, key, phase string, results []accessResult, err error) {
	failed, unchanged := 0, 0
	for _, r := range results {
		outcome := "succeeded"
		level := slog.LevelInfo
		if r.err != nil {
			outcome = "failed"
			level = slog.LevelError
			failed++
		} else if r.unchanged {
			outcome = "unchanged"
			level = slog.LevelDebug
			unchanged++
		}
		attributes := []any{"scope", scope, "key", key, "action", r.verb, "kind", r.kind, "namespace", r.namespace, "name", r.name, "outcome", outcome, "duration", r.duration}
		if r.err != nil {
			attributes = append(attributes, "error", r.err)
		}
		if r.verb == "status" && r.err == nil {
			conditions := make([]map[string]any, 0, len(r.conditions))
			for _, c := range r.conditions {
				conditions = append(conditions, map[string]any{"type": c.Type, "status": string(c.Status), "reason": c.Reason, "message": c.Message, "observed_generation": c.ObservedGeneration})
			}
			attributes = append(attributes, "conditions", conditions)
		}
		m.logger.Log(context.Background(), level, r.logMessage(), attributes...)
	}
	outcome := "succeeded"
	if err != nil {
		outcome = "failed"
	}
	m.logger.Debug("Reconciled "+scope+" scope", "scope", scope, "key", key, "phase", phase, "actions", len(results), "failed_actions", failed, "unchanged_actions", unchanged, "outcome", outcome)
}

func (r accessResult) logMessage() string {
	target := r.kind + " " + r.namespace + "/" + r.name
	operation, completed := r.verb, ""
	switch r.verb {
	case "create":
		completed = "Created"
	case "update":
		completed = "Updated"
	case "delete":
		completed = "Requested deletion of"
	case "status":
		operation, completed = "update", "Updated"
		target += " status"
	}
	if (r.kind == "TLSRoute" || r.kind == "Gateway") && (r.verb == "create" || r.verb == "update") {
		operation, completed = "apply", "Applied"
	}
	if r.err != nil {
		return "Failed to " + operation + " " + target
	}
	if r.unchanged {
		return "Unchanged " + target
	}
	message := completed + " " + target
	if r.verb == "status" {
		var conditions []string
		for _, c := range r.conditions {
			condition := c.Type + "=" + string(c.Status) + " (" + c.Reason + ")"
			if c.Message != "" {
				condition += ": " + c.Message
			}
			conditions = append(conditions, condition)
		}
		if len(conditions) > 0 {
			message += ": " + strings.Join(conditions, "; ")
		}
	}
	return message
}
