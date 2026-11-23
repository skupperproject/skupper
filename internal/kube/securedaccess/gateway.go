package securedaccess

import (
	skuppertypes "github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

// gatewayScope is the sole shared-Gateway writer; Access scopes only consume
// its cached status.
type gatewayScope struct{ manager *SecuredAccessManager }

func (g *gatewayScope) Kind() string                              { return "GatewayScope" }
func (g *gatewayScope) Describe(e watchers.ResourceChange) string { return g.Kind() + " " + e.Key }
func (g *gatewayScope) Handle(e watchers.ResourceChange) (err error) {
	m := g.manager
	phase := "key"
	var results []accessResult
	defer func() { m.logResults("Gateway", e.Key, phase, results, err) }()
	ns, name, err := cache.SplitMetaNamespaceKey(e.Key)
	if err != nil {
		return err
	}
	if !m.config.isEnabled(ACCESS_TYPE_GATEWAY) || m.context == nil || ns != m.context.Namespace() || name != "skupper" {
		return nil
	}
	phase = "snapshot"
	if m.watcher.Gateways == nil {
		// Logged once at watch registration; Access status reports it.
		return nil
	}
	observed, err := m.watcher.Gateways.Get(e.Key)
	if err != nil {
		return err
	}
	observed = observed.DeepCopy()
	desired := desiredGateway(ns, m.config)
	policy := m.context.SnapshotMetadata(ns)
	phase = "plan"
	actions, err := planGateway(observed, desired, policy)
	if err != nil {
		return err
	}
	phase = "execute"
	for _, a := range actions {
		var result *accessResult
		result, err = m.executeCachedApply(a)
		if result != nil {
			results = append(results, *result)
		}
		if err != nil {
			break
		}
	}
	return err
}

func planGateway(observed, desired *unstructured.Unstructured, policy *labels.LabelsAndAnnotations) ([]accessAction, error) {
	// The named Gateway in the controller namespace is reserved for Skupper,
	// regardless of its creator.
	if observed != nil && observed.GetDeletionTimestamp() != nil {
		return nil, nil
	}
	configuration, err := applyConfiguration(observed, desired, policy, "Gateway")
	if err != nil {
		return nil, err
	}
	verb := "create"
	if observed != nil {
		verb = "update"
	}
	return []accessAction{{verb: verb, kind: "Gateway", object: configuration, observed: observed}}, nil
}

func desiredGateway(namespace string, c Config) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1", "kind": "Gateway",
		"metadata": map[string]interface{}{"name": "skupper", "namespace": namespace},
		"spec":     map[string]interface{}{"gatewayClassName": c.GatewayClass, "listeners": []interface{}{map[string]interface{}{"name": "tls", "protocol": "TLS", "port": int64(c.GatewayPort), "tls": map[string]interface{}{"mode": "Passthrough"}, "allowedRoutes": map[string]interface{}{"namespaces": map[string]interface{}{"from": "All"}}}}},
	}}
}

func desiredTLSRoute(sa *skupperv2alpha1.SecuredAccess, p skupperv2alpha1.SecuredAccessPort, gatewayNamespace, domain string) *unstructured.Unstructured {
	name := sa.Name + "-" + p.Name
	d := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "gateway.networking.k8s.io/v1alpha2", "kind": "TLSRoute",
		"spec": map[string]interface{}{
			"parentRefs": []interface{}{map[string]interface{}{"group": "gateway.networking.k8s.io", "kind": "Gateway", "name": "skupper", "namespace": gatewayNamespace, "sectionName": "tls"}},
			"hostnames":  []interface{}{qualify(name+"."+sa.Namespace, domain)},
			"rules":      []interface{}{map[string]interface{}{"backendRefs": []interface{}{map[string]interface{}{"group": "", "kind": "Service", "name": sa.Name, "namespace": sa.Namespace, "port": int64(p.Port), "weight": int64(1)}}}},
		},
	}}
	d.SetName(name)
	d.SetNamespace(sa.Namespace)
	d.SetOwnerReferences(ownerReferences(sa))
	d.SetLabels(map[string]string{accessLabel: "true"})
	d.SetAnnotations(map[string]string{skuppertypes.ControlledQualifier: "true"})
	return d
}

func getBaseDomain(obj *unstructured.Unstructured) string {
	addresses, _, _ := unstructured.NestedSlice(obj.Object, "status", "addresses")
	for _, kind := range []string{"Hostname", "IPAddress"} {
		for _, a := range addresses {
			if address, ok := a.(map[string]interface{}); ok {
				value, _, _ := unstructured.NestedString(address, "value")
				t, _, _ := unstructured.NestedString(address, "type")
				if value != "" && t == kind {
					if kind == "IPAddress" {
						return value + ".nip.io"
					}
					return value
				}
			}
		}
	}
	return ""
}
