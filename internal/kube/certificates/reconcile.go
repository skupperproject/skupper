package certificates

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type certificateSnapshot struct {
	certificate    *skupperv2alpha1.Certificate
	secret, ca     *corev1.Secret
	metadataPolicy *labels.LabelsAndAnnotations
	now            time.Time
}

type certificateActionKind string

const (
	cleanupCertificate certificateActionKind = "cleanup-certificate"
	createSecret       certificateActionKind = "create-secret"
	renewSecret        certificateActionKind = "renew-secret"
	updateMetadata     certificateActionKind = "update-secret-metadata"
	reconcileStatus    certificateActionKind = "reconcile-status"
)

type issuanceInputs struct {
	subject  string
	hosts    []string
	lifetime time.Duration
	ca       *corev1.Secret
}

type certificateAction struct {
	kind          certificateActionKind
	reason        string
	certificate   *skupperv2alpha1.Certificate
	secret        *corev1.Secret
	issuance      *issuanceInputs
	validationErr error
}

type certificatePlan []certificateAction

type actionOutcome string

const (
	actionSucceeded actionOutcome = "succeeded"
	actionFailed    actionOutcome = "failed"
	actionUnchanged actionOutcome = "unchanged"
)

type certificateActionResult struct {
	action                        certificateActionKind
	resourceKind, namespace, name string
	outcome                       actionOutcome
	duration                      time.Duration
	reason                        string
	readyCondition                *metav1.Condition
	err                           error
}

func (m *CertificateManagerImpl) snapshot(key string) (certificateSnapshot, error) {
	s := certificateSnapshot{now: time.Now()}
	certificate, err := m.certificateWatcher.Get(key)
	if err != nil {
		return s, err
	}
	secret, err := m.secretWatcher.Get(key)
	if err != nil {
		return s, err
	}
	s.certificate, s.secret = certificate.DeepCopy(), secret.DeepCopy()
	if certificate != nil {
		if !certificate.Spec.Signing {
			ca, err := m.secretWatcher.Get(certificate.Namespace + "/" + certificate.Spec.Ca)
			if err != nil {
				return s, err
			}
			s.ca = ca.DeepCopy()
		}
		if m.context != nil {
			s.metadataPolicy = m.context.SnapshotMetadata(certificate.Namespace)
		}
	}
	return s, nil
}

// planCertificate derives desired metadata and validity from snapshot.
// Crypto and condition transition timestamps are execution.
func planCertificate(s certificateSnapshot) certificatePlan {
	c, secret := s.certificate.DeepCopy(), s.secret.DeepCopy()
	// Kubernetes garbage collection owns Secret deletion through owner references.
	if c == nil {
		return nil
	}
	if c.DeletionTimestamp != nil || (secret != nil && secret.DeletionTimestamp != nil) {
		return nil
	}
	// check for deleted co-owner cleanup
	owners := certificateToOwnerMapping(c)
	if owners.IsControlled {
		present := map[string]bool{}
		for _, ref := range c.OwnerReferences {
			present[string(ref.UID)] = true
		}
		for uid := range owners.PerOwnerHosts {
			if !present[uid] {
				delete(owners.PerOwnerHosts, uid)
			}
		}
		if owners.ApplyMetadata(c) {
			c.Spec.Hosts = owners.CombinedHosts()
			return certificatePlan{{kind: cleanupCertificate, certificate: c, reason: "owner-contributions-changed"}}
		}
	}

	var validationErr error
	var secretAction certificateActionKind
	var renewalReason string
	if secret != nil {
		for _, ref := range secret.OwnerReferences {
			if ref.APIVersion == "skupper.io/v2alpha1" && ref.Kind == "Certificate" && (ref.Name != c.Name || ref.UID != c.UID) {
				validationErr = errors.New("secret has conflicting Certificate ownership")
				break
			}
		}
	}
	if secret == nil {
		secretAction = createSecret
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: c.Namespace,
			OwnerReferences: ownerReferences(c), Labels: map[string]string{}, Annotations: map[string]string{
				"internal.skupper.io/controlled": "true", "internal.skupper.io/certificate": "true",
				"internal.skupper.io/hosts": strings.Join(c.Spec.Hosts, ","),
			}}}
	} else if validationErr == nil {
		renewalReason = secretRenewalReason(c, secret, s.now)
		if renewalReason != "" {
			if !isSecretControlled(secret) {
				validationErr = errors.New("secret exists but is not controlled by skupper")
			} else {
				secretAction = renewSecret
				setAnnotation(&secret.ObjectMeta, "internal.skupper.io/hosts", strings.Join(c.Spec.Hosts, ","))
			}
		}
	}
	var issuance *issuanceInputs
	if secretAction == createSecret || secretAction == renewSecret {
		issuance = &issuanceInputs{subject: c.Spec.Subject}
		if !c.Spec.Signing {
			issuance.hosts, issuance.lifetime, issuance.ca = c.Spec.Hosts, time.Hour*24*365*5, s.ca.DeepCopy()
			if issuance.ca == nil {
				validationErr = fmt.Errorf("CA %q not found", c.Namespace+"/"+c.Spec.Ca)
			}
		}
	}
	if validationErr == nil && (s.secret == nil || isSecretControlled(secret)) {
		// Evaluate the captured policy after deriving built-in annotations.
		metadata := secret.ObjectMeta.DeepCopy()
		if s.metadataPolicy != nil {
			if metadata.Labels == nil {
				metadata.Labels = map[string]string{}
			}
			if metadata.Annotations == nil {
				metadata.Annotations = map[string]string{}
			}
			s.metadataPolicy.SetLabels(c.Namespace, c.Name, "Secret", metadata.Labels)
			s.metadataPolicy.SetAnnotations(c.Namespace, c.Name, "Secret", metadata.Annotations)
		}
		for k, v := range metadata.Labels {
			if secret.Labels == nil {
				secret.Labels = map[string]string{}
			}
			secret.Labels[k] = v
		}
		for k, v := range metadata.Annotations {
			setAnnotation(&secret.ObjectMeta, k, v)
		}
		if secretAction == "" && !reflect.DeepEqual(secret.ObjectMeta, s.secret.ObjectMeta) {
			secretAction = updateMetadata
		}
	}
	var plan certificatePlan
	if validationErr == nil && secretAction != "" {
		plan = append(plan, certificateAction{kind: secretAction, secret: secret, issuance: issuance, reason: renewalReason})
	}
	desired := skupperv2alpha1.ErrorOrReadyCondition(validationErr)
	current := meta.FindStatusCondition(c.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
	statusDiffers := current == nil || current.Status != desired.Status || current.Reason != string(desired.Reason) || current.Message != desired.Message || current.ObservedGeneration != c.Generation
	// Reconcile status when there is a validation error, when the status
	// differs, OR there is a plan action that could fail in execution that
	// needs reported back to certificate status.
	if statusDiffers || validationErr != nil || len(plan) != 0 {
		plan = append(plan, certificateAction{kind: reconcileStatus, certificate: c, validationErr: validationErr})
	}
	return plan
}

func (m *CertificateManagerImpl) execute(plan certificatePlan) ([]certificateActionResult, error) {
	ctx := context.Background()
	var results []certificateActionResult
	var executionErr error
	for _, action := range plan {
		started := time.Now()
		result := certificateActionResult{action: action.kind, outcome: actionSucceeded, reason: action.reason}
		if action.certificate != nil {
			result.resourceKind, result.namespace, result.name = "Certificate", action.certificate.Namespace, action.certificate.Name
		} else {
			result.resourceKind, result.namespace, result.name = "Secret", action.secret.Namespace, action.secret.Name
		}
		switch action.kind {
		case cleanupCertificate:
			c := action.certificate.DeepCopy()
			_, result.err = m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(c.Namespace).Update(ctx, c, metav1.UpdateOptions{})
		case createSecret, renewSecret, updateMetadata:
			s := action.secret.DeepCopy()
			if action.issuance != nil {
				inputs := action.issuance
				var generated *corev1.Secret
				generated, result.err = certs.GenerateSecret(s.Name, inputs.subject, inputs.hosts, inputs.lifetime, inputs.ca)
				if result.err == nil {
					s.Data = generated.Data
					if action.kind == createSecret {
						s.Type = generated.Type
					}
				}
			}
			if result.err == nil {
				client := m.processor.GetKubeClient().CoreV1().Secrets(s.Namespace)
				if action.kind == createSecret {
					_, result.err = client.Create(ctx, s, metav1.CreateOptions{})
				} else {
					_, result.err = client.Update(ctx, s, metav1.UpdateOptions{})
				}
			}
		case reconcileStatus:
			c := action.certificate.DeepCopy()
			if c.SetReady(errors.Join(executionErr, action.validationErr)) {
				_, result.err = m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(c.Namespace).UpdateStatus(ctx, c, metav1.UpdateOptions{})
			} else {
				result.outcome = actionUnchanged
			}
			if result.err == nil {
				result.readyCondition = meta.FindStatusCondition(c.Status.Conditions, skupperv2alpha1.CONDITION_TYPE_READY)
			}
		}
		if result.err != nil {
			result.outcome = actionFailed
			executionErr = errors.Join(executionErr, result.err)
		}
		result.duration = time.Since(started)
		results = append(results, result)
	}
	return results, executionErr
}

// An empty reason means the existing certificate satisfies the current policy.
// When several checks fail, report the first in this order.
func secretRenewalReason(certificate *skupperv2alpha1.Certificate, secret *corev1.Secret, now time.Time) string {
	cert, err := certs.DecodeCertificate(secret.Data["tls.crt"])
	if err != nil {
		return "invalid-certificate"
	}
	if _, err := tls.X509KeyPair(secret.Data["tls.crt"], secret.Data["tls.key"]); err != nil {
		return "invalid-key-pair"
	}
	if now.After(cert.NotAfter) {
		return "expired"
	}
	if certificate.Spec.Subject != cert.Subject.CommonName {
		return "subject-changed"
	}
	validFor := map[string]bool{}
	for _, host := range cert.DNSNames {
		if host != "" {
			validFor[host] = true
		}
	}
	for _, ip := range cert.IPAddresses {
		validFor[ip.String()] = true
	}
	if len(certificate.Spec.Hosts) != len(validFor) {
		return "hosts-changed"
	}
	for _, host := range certificate.Spec.Hosts {
		if !validFor[host] {
			return "hosts-changed"
		}
	}
	return ""
}
