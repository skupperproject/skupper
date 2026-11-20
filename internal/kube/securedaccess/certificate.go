package securedaccess

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	skuppertypes "github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/kube/certificates"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

// One stable handler, keyed by Certificate namespace/name, owns the aggregate.
// Individual Access scopes publish exposure status and never write Certificates.
type accessCertificateScope struct{ manager *SecuredAccessManager }

type accessCertificateSnapshot struct {
	namespace, name string
	accesses        []*skupperv2alpha1.SecuredAccess
	certificate     *skupperv2alpha1.Certificate
	policy          *labels.LabelsAndAnnotations
}

func (c *accessCertificateScope) Kind() string { return "SecuredAccessCertificateScope" }
func (c *accessCertificateScope) Describe(e watchers.ResourceChange) string {
	return c.Kind() + " " + e.Key
}
func (c *accessCertificateScope) Handle(e watchers.ResourceChange) (err error) {
	phase := "key"
	var results []accessResult
	defer func() { c.manager.logResults("SecuredAccessCertificate", e.Key, phase, results, err) }()
	ns, _, err := cache.SplitMetaNamespaceKey(e.Key)
	if err != nil || !c.manager.isControlled(ns) {
		return err
	}
	phase = "snapshot"
	s, err := c.snapshot(e.Key)
	if err != nil {
		return err
	}
	phase = "plan"
	action, err := planAccessCertificate(s)
	if err != nil {
		return err
	}
	phase = "execute"
	if action != nil {
		start := time.Now()
		_, err = c.manager.executeAction(*action)
		results = append(results, accessResult{verb: action.verb, kind: "Certificate", namespace: s.namespace, name: s.name, duration: time.Since(start), err: err})
	}
	return err
}

func (c *accessCertificateScope) snapshot(key string) (accessCertificateSnapshot, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	s := accessCertificateSnapshot{namespace: ns, name: name}
	if err != nil {
		return s, err
	}
	w := c.manager.watcher
	observed, err := w.Certificates.Get(key)
	if err != nil {
		return s, err
	}
	s.certificate = observed.DeepCopy()
	for _, sa := range w.Accesses.List() {
		if sa.Namespace == ns && CertificateTarget(sa) == name {
			s.accesses = append(s.accesses, sa.DeepCopy())
		}
	}
	sort.Slice(s.accesses, func(i, j int) bool { return s.accesses[i].Name < s.accesses[j].Name })
	if c.manager.context != nil {
		s.policy = c.manager.context.SnapshotMetadata(ns)
	}
	return s, nil
}

// CertificateTarget is shared by aggregate planning and controller routing.
func CertificateTarget(sa *skupperv2alpha1.SecuredAccess) string {
	if sa == nil || sa.DeletionTimestamp != nil || sa.Spec.Issuer == "" {
		return ""
	}
	if sa.Spec.Certificate != "" {
		return sa.Spec.Certificate
	}
	return sa.Name
}

func reservedCertificate(name string) bool {
	switch name {
	case "skupper-site-ca", "skupper-local-ca", "skupper-local-server", "skupper-grant-server-ca":
		return true
	}
	return false
}

func accessManagedCertificate(c *skupperv2alpha1.Certificate) bool {
	if c == nil || c.Annotations[skuppertypes.ControlledQualifier] != "true" || len(c.OwnerReferences) == 0 {
		return false
	}
	for _, ref := range c.OwnerReferences {
		if !accessReference(ref) || ref.UID == "" {
			return false
		}
	}
	return true
}

// Shared validation gives Access status a useful error without introducing a
// second status writer in the Certificate scope. It also guards the actual writer.
func validateAccessCertificate(s accessCertificateSnapshot) error {
	if len(s.accesses) == 0 {
		return nil
	}
	if reservedCertificate(s.name) {
		return fmt.Errorf("Certificate name %q is reserved for infrastructure; set spec.certificate to a non-reserved name", s.name)
	}
	issuer := s.accesses[0].Spec.Issuer
	for _, sa := range s.accesses[1:] {
		if sa.Spec.Issuer != issuer {
			return fmt.Errorf("Certificate %q has incompatible SecuredAccess issuers %q and %q", s.name, issuer, sa.Spec.Issuer)
		}
	}
	if s.certificate != nil && !accessManagedCertificate(s.certificate) {
		return fmt.Errorf("Certificate %q has conflicting ownership", s.name)
	}
	return nil
}

func planAccessCertificate(s accessCertificateSnapshot) (*accessAction, error) {
	if err := validateAccessCertificate(s); err != nil {
		return nil, err
	}
	observed := s.certificate
	// This guard covers cleanup too: old invalid ownership never authorizes a
	// write or deletion of Site/grant infrastructure or mixed-owner Certificates.
	if reservedCertificate(s.name) || (observed != nil && (!accessManagedCertificate(observed) || observed.DeletionTimestamp != nil)) {
		return nil, nil
	}
	if len(s.accesses) == 0 {
		if observed == nil {
			return nil, nil
		}
		return &accessAction{verb: "delete", kind: "Certificate", object: observed.DeepCopy()}, nil
	}
	desired := observed.DeepCopy()
	verb := "update"
	if desired == nil {
		verb = "create"
		desired = &skupperv2alpha1.Certificate{TypeMeta: metav1.TypeMeta{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate"}, ObjectMeta: metav1.ObjectMeta{Name: s.name, Namespace: s.namespace}, Spec: skupperv2alpha1.CertificateSpec{Subject: s.accesses[0].Name}}
	}
	if desired.Labels == nil {
		desired.Labels = map[string]string{}
	}
	if desired.Annotations == nil {
		desired.Annotations = map[string]string{}
	}
	desired.Labels["internal.skupper.io/certificate"] = "true"
	desired.Annotations[skuppertypes.ControlledQualifier] = "true"
	for key := range desired.Annotations {
		if strings.HasPrefix(key, certificates.CertificateHostsAnnotationPrefix) {
			delete(desired.Annotations, key)
		}
	}
	desired.OwnerReferences = nil
	hosts := map[string]bool{}
	for _, sa := range s.accesses {
		contribution := getHosts(sa)
		configured := meta.FindStatusCondition(sa.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_CONFIGURED)
		resolved := meta.FindStatusCondition(sa.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_RESOLVED)
		if !sa.IsReady() || configured.ObservedGeneration != sa.Generation || resolved.ObservedGeneration != sa.Generation {
			// Keep a previously valid contribution while exposure is unresolved;
			// absent history contributes only the Access's canonical local names.
			contribution = []string{sa.Name, sa.Name + "." + sa.Namespace}
			if observed != nil {
				if old, ok := observed.Annotations[certificates.CertificateHostsAnnotationPrefix+string(sa.UID)]; ok {
					contribution = strings.Split(old, ",")
				}
			}
		}
		desired.OwnerReferences = append(desired.OwnerReferences, ownerReferences(sa)...)
		desired.Annotations[certificates.CertificateHostsAnnotationPrefix+string(sa.UID)] = strings.Join(contribution, ",")
		for _, host := range contribution {
			if host != "" {
				hosts[host] = true
			}
		}
	}
	desired.Spec.Ca = s.accesses[0].Spec.Issuer
	desired.Spec.Signing, desired.Spec.Client, desired.Spec.Server = false, false, true
	desired.Spec.Hosts = nil
	for host := range hosts {
		desired.Spec.Hosts = append(desired.Spec.Hosts, host)
	}
	sort.Strings(desired.Spec.Hosts)
	applyPolicy(s.policy, "Certificate", desired)
	desired.Annotations[skuppertypes.ControlledQualifier] = "true"
	desired.Annotations[certificates.SecuredAccessCertificateAnnotation] = "true"
	if reflect.DeepEqual(observed, desired) {
		return nil, nil
	}
	return &accessAction{verb: verb, kind: "Certificate", object: desired}, nil
}
