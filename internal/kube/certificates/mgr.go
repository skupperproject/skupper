// Package certificates maintains a Secret for each Certificate scope.
package certificates

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

type ControllerContext interface {
	IsControlled(namespace string) bool
	SnapshotMetadata(namespace string) *labels.LabelsAndAnnotations
	SetLabels(namespace string, name string, kind string, labels map[string]string) bool
	SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool
}

type CertificateManager interface {
	EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error
	Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error
}

type CertificateManagerImpl struct {
	certificateWatcher *watchers.CertificateWatcher
	secretWatcher      *watchers.SecretWatcher
	processor          *watchers.EventProcessor
	context            ControllerContext
	logger             *slog.Logger
	// Authoring-only write-through view. A nil entry requires a live refresh;
	// an absent entry uses the informer. Reconciliation never reads this map.
	authored map[string]*skupperv2alpha1.Certificate
}

func NewCertificateManager(processor *watchers.EventProcessor) *CertificateManagerImpl {
	return &CertificateManagerImpl{processor: processor, logger: slog.Default().With("component", "kube.certificates.manager"), authored: map[string]*skupperv2alpha1.Certificate{}}
}

func (m *CertificateManagerImpl) SetControllerContext(context ControllerContext) { m.context = context }

func (m *CertificateManagerImpl) Watch(namespace string) {
	m.certificateWatcher = m.processor.WatchCertificates(namespace, m.certificateChanged)
	m.secretWatcher = m.processor.WatchAllSecrets(namespace, m.secretChanged)
}

func (m *CertificateManagerImpl) certificateChanged(key string, observed *skupperv2alpha1.Certificate) error {
	if written, ok := m.authored[key]; ok {
		if observed == nil || (written != nil && reflect.DeepEqual(written, observed)) {
			delete(m.authored, key)
		} else {
			// This may be an external change or an intermediate observation
			// predating our latest write. Do not guess.
			m.authored[key] = nil
		}
	}
	m.processor.Enqueue(m, key)
	return nil
}

func (m *CertificateManagerImpl) secretChanged(key string, _ *corev1.Secret) error {
	m.processor.Enqueue(m, key)
	// TODO consider an informer index instead of full scan
	for _, cert := range m.certificateWatcher.List() {
		if !cert.Spec.Signing && cert.Namespace+"/"+cert.Spec.Ca == key {
			m.processor.Enqueue(m, cert.Key())
		}
	}
	return nil
}

func (m *CertificateManagerImpl) isControlled(namespace string) bool {
	return m.context == nil || m.context.IsControlled(namespace)
}

func (m *CertificateManagerImpl) Kind() string { return "CertificateScope" }
func (m *CertificateManagerImpl) Describe(event watchers.ResourceChange) string {
	return m.Kind() + " " + event.Key
}

// Handle Certificate Scope
func (m *CertificateManagerImpl) Handle(event watchers.ResourceChange) error {
	namespace, _, err := cache.SplitMetaNamespaceKey(event.Key)
	if err != nil || !m.isControlled(namespace) {
		return err
	}
	snapshot, err := m.snapshot(event.Key)
	if err != nil {
		return err
	}
	plan := planCertificate(snapshot)
	results, err := m.execute(plan)
	m.logResults(event.Key, results, err)
	return err
}

func (m *CertificateManagerImpl) logResults(key string, results []certificateActionResult, err error) {
	failedActions := 0
	for _, result := range results {
		level := slog.LevelInfo
		if result.outcome == actionUnchanged {
			level = slog.LevelDebug
		} else if result.err != nil {
			level = slog.LevelError
			failedActions++
		}
		attributes := []any{"key", key,
			"action", result.action, "resource", result.resourceKind, "namespace", result.namespace, "name", result.name,
			"outcome", result.outcome, "duration", result.duration, "reason", result.reason, "error", result.err}
		if condition := result.readyCondition; condition != nil {
			attributes = append(attributes, "ready", condition.Status == metav1.ConditionTrue,
				"condition_reason", condition.Reason, "condition_message", condition.Message)
		}
		m.logger.Log(context.Background(), level, result.logMessage(), attributes...)
	}
	level := slog.LevelDebug
	outcome := actionSucceeded
	if err != nil {
		level = slog.LevelError
		outcome = actionFailed
	}
	m.logger.Log(context.Background(), level, "Reconciled certificate scope", "key", key,
		"actions", len(results), "failed_actions", failedActions, "outcome", outcome)
}

func (r certificateActionResult) logMessage() string {
	var operation, completed string
	switch r.action {
	case createSecret:
		operation, completed = "create Secret for Certificate", "Created Secret for Certificate"
	case renewSecret:
		operation, completed = "renew Secret for Certificate", "Renewed Secret for Certificate"
	case updateMetadata:
		operation, completed = "update Certificate Secret metadata", "Updated Certificate Secret metadata"
	case cleanupCertificate:
		operation, completed = "update Certificate owner contributions", "Updated Certificate owner contributions"
	case reconcileStatus:
		operation, completed = "update Certificate status", "Updated Certificate status"
		if r.outcome == actionUnchanged {
			return "Certificate status unchanged"
		}
	}
	if r.outcome == actionFailed {
		return "Failed to " + operation
	}
	return completed
}

func (m *CertificateManagerImpl) EnsureCA(namespace, name, subject string, refs []metav1.OwnerReference) error {
	return m.ensure(namespace, name, skupperv2alpha1.CertificateSpec{Subject: subject, Signing: true}, refs)
}

func (m *CertificateManagerImpl) Ensure(namespace, name, ca, subject string, hosts []string, client, server bool, refs []metav1.OwnerReference) error {
	return m.ensure(namespace, name, skupperv2alpha1.CertificateSpec{Ca: ca, Subject: subject, Hosts: hosts, Client: client, Server: server}, refs)
}

var compareSpecUnordered = []cmp.Option{cmpopts.EquateEmpty(), cmpopts.SortSlices(func(a, b string) bool { return a < b })}

// Authoring reads its own writes until informer notifications acknowledge them.
// Cache misses and ambiguous notifications use the API; settled no-ops do not.
func (m *CertificateManagerImpl) authoringCertificate(namespace, name string) (*skupperv2alpha1.Certificate, error) {
	key := namespace + "/" + name
	if written, ok := m.authored[key]; ok {
		if written != nil {
			return written.DeepCopy(), nil
		}
	} else if m.certificateWatcher != nil {
		observed, err := m.certificateWatcher.Get(key)
		if err != nil || observed != nil {
			return observed.DeepCopy(), err
		}
	}
	current, err := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		m.authored[key] = current.DeepCopy()
	}
	return current, err
}

func (m *CertificateManagerImpl) ensure(namespace, name string, spec skupperv2alpha1.CertificateSpec, refs []metav1.OwnerReference) error {
	key := namespace + "/" + name
	client := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(namespace)
	current, err := m.authoringCertificate(namespace, name)
	if apierrors.IsNotFound(err) {
		cert := &skupperv2alpha1.Certificate{
			TypeMeta: metav1.TypeMeta{APIVersion: "skupper.io/v2alpha1", Kind: "Certificate"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, OwnerReferences: refs,
				Labels: map[string]string{"internal.skupper.io/certificate": "true"}, Annotations: map[string]string{}},
			Spec: spec,
		}
		newOwnerMapping(refs, spec.Hosts).ApplyMetadata(cert)
		m.applyMetadata(namespace, name, "Certificate", &cert.ObjectMeta)
		created, err := client.Create(context.Background(), cert, metav1.CreateOptions{})
		m.authored[key] = nil
		if err == nil {
			m.authored[key] = created.DeepCopy()
		}
		return err
	}
	if err != nil {
		return err
	}
	ownerMap := certificateToOwnerMapping(current)
	if !ownerMap.IsControlled {
		return fmt.Errorf("certificate %q exists but is not controlled by skupper", name)
	}
	changed := mergeOwnerReferences(&current.ObjectMeta, refs)
	for _, ref := range refs {
		ownerMap.PerOwnerHosts[string(ref.UID)] = append([]string(nil), spec.Hosts...)
	}
	if ownerMap.ApplyMetadata(current) {
		changed = true
	}
	if len(current.OwnerReferences) > 1 {
		spec.Subject = current.Spec.Subject
	}
	spec.Hosts = ownerMap.CombinedHosts()
	if !cmp.Equal(spec, current.Spec, compareSpecUnordered...) {
		current.Spec = spec
		changed = true
	}
	if m.applyMetadata(namespace, name, "Certificate", &current.ObjectMeta) {
		changed = true
	}
	if !changed {
		return nil
	}
	updated, err := client.Update(context.Background(), current, metav1.UpdateOptions{})
	m.authored[key] = nil
	if err == nil {
		m.authored[key] = updated.DeepCopy()
	}
	return err
}

func (m *CertificateManagerImpl) applyMetadata(namespace, name, kind string, meta *metav1.ObjectMeta) bool {
	if m.context == nil {
		return false
	}
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	}
	labels := m.context.SetLabels(namespace, name, kind, meta.Labels)
	annotations := m.context.SetAnnotations(namespace, name, kind, meta.Annotations)
	return labels || annotations
}

func isSecretControlled(secret *corev1.Secret) bool {
	if _, ok := secret.Annotations[annotationKeySkupperControlled]; ok {
		return true
	}
	for _, owner := range secret.OwnerReferences {
		if owner.Kind == "Certificate" && owner.APIVersion == "skupper.io/v2alpha1" {
			return true
		}
	}
	return false
}

func ownerReferences(cert *skupperv2alpha1.Certificate) []metav1.OwnerReference {
	return []metav1.OwnerReference{{Kind: "Certificate", APIVersion: "skupper.io/v2alpha1", Name: cert.Name, UID: cert.UID}}
}

func mergeOwnerReferences(obj *metav1.ObjectMeta, added []metav1.OwnerReference) bool {
	changed := false
	byUID := map[types.UID]metav1.OwnerReference{}
	for _, ref := range obj.OwnerReferences {
		byUID[ref.UID] = ref
	}
	for _, ref := range added {
		if actual, ok := byUID[ref.UID]; !ok || actual != ref {
			obj.OwnerReferences = append(obj.OwnerReferences, ref)
			changed = true
		}
	}
	return changed
}
