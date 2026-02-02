// Package certificates provides the ability to create or update
// instances of the v2alpha1 Certificate resource, and ensure that a
// corresponding Secret resource is maintained for each.
package certificates

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

// The ControllerContext interface defines the invocations the
// CertificateManager needs to make to correctly manage the resources
// it is responsible for.
type ControllerContext interface {
	// Determines whether resources in a given namespace are in
	// scope for control by the CertificateManager.
	IsControlled(namespace string) bool
	// Called to set any extra labels on resources managed by the CertificateManager.
	SetLabels(namespace string, name string, kind string, labels map[string]string) bool
	// Called to set any extra annotations on resources managed by the CertificateManager.
	SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool
}

// The CertificateManager interface defines the methods through which
// the existence of a particular Certificate resource can be
// ensured. It is currently used by package internal/kube/site.
type CertificateManager interface {
	EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error
	Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error
}

type CertificateManagerImpl struct {
	definitions        map[string]*skupperv2alpha1.Certificate
	secrets            map[string]*corev1.Secret
	certificateWatcher *watchers.CertificateWatcher
	secretWatcher      *watchers.SecretWatcher
	processor          *watchers.EventProcessor
	context            ControllerContext
	logger             *slog.Logger
}

// Returns a correctly initialised CertificateManager.
func NewCertificateManager(processor *watchers.EventProcessor) *CertificateManagerImpl {
	return &CertificateManagerImpl{
		definitions: map[string]*skupperv2alpha1.Certificate{},
		secrets:     map[string]*corev1.Secret{},
		processor:   processor,
		logger:      slog.New(slog.Default().Handler()).With(slog.String("component", "kube.certificates.manager")),
	}
}

// Allows a ControllerContext to be set for this CertificateManager.
func (m *CertificateManagerImpl) SetControllerContext(context ControllerContext) {
	m.context = context
}

// Causes the CertificateManager to start watching relevant resources.
func (m *CertificateManagerImpl) Watch(watchNamespace string) {
	m.certificateWatcher = m.processor.WatchCertificates(watchNamespace, watchers.FilterByNamespace(m.isControlled, m.checkCertificate))
	m.secretWatcher = m.processor.WatchAllSecrets(watchNamespace, watchers.FilterByNamespace(m.isControlled, m.checkSecret))
}

func (m *CertificateManagerImpl) isControlled(namespace string) bool {
	if m.context != nil {
		return m.context.IsControlled(namespace)
	}
	return true
}

// This will iterate through the existing resources to recover the
// correct internal state. This should only be called after Watch()
// has been invoked.
func (m *CertificateManagerImpl) Recover() {
	for _, secret := range m.secretWatcher.List() {
		if !m.isControlled(secret.Namespace) {
			continue
		}
		m.secrets[secretKey(secret)] = secret
	}
	for _, cert := range m.certificateWatcher.List() {
		if !m.isControlled(cert.Namespace) {
			continue
		}
		if err := m.checkCertificate(cert.Key(), cert); err != nil {
			m.logger.Error("Error trying to reconcile certificate", slog.String("key", cert.Key()), slog.Any("error", err))
		}
	}
}

// This method is called to ensure that a Certificate resource exists
// to represent a CA (i.e. certificate issuer) with the properties
// specified in the arguments.
func (m *CertificateManagerImpl) EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error {
	spec := skupperv2alpha1.CertificateSpec{
		Subject: subject,
		Signing: true,
	}
	return m.ensure(namespace, name, spec, refs)
}

// This method is called to ensure that a Certificate resource exists
// with the properties specified in the arguments. This can be called
// with different owners, in which case the owenres are all merged
// in. Hosts are tracked per owner, so if two different owners specify
// different hosts, they will all be included in the certificate, but
// if the same owner changes the hosts then they will be changed on
// the certificate. This allows the same certificate to be used for
// multiple resources such as Routes.
func (m *CertificateManagerImpl) Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error {
	spec := skupperv2alpha1.CertificateSpec{
		Ca:      ca,
		Subject: subject,
		Hosts:   hosts,
		Client:  client,
		Server:  server,
	}
	return m.ensure(namespace, name, spec, refs)
}

var compareSpecUnordered []cmp.Option = []cmp.Option{
	cmpopts.EquateEmpty(),
	cmpopts.SortSlices(func(a, b string) bool { return a < b }),
}

func (m *CertificateManagerImpl) ensure(namespace string, name string, spec skupperv2alpha1.CertificateSpec, refs []metav1.OwnerReference) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if current, ok := m.definitions[key]; ok {
		changed := false
		ownerMap := certificateToOwnerMapping(current)
		if !ownerMap.IsControlled {
			return fmt.Errorf("certificate %q exists but is not controlled by skupper", name)
		}
		if mergeOwnerReferences(&current.ObjectMeta, refs) {
			changed = true
		}
		for _, ref := range refs {
			refUID := string(ref.UID)
			configuredHosts := ownerMap.PerOwnerHosts[refUID]
			if !cmp.Equal(configuredHosts, spec.Hosts, compareSpecUnordered...) {
				ownerMap.PerOwnerHosts[refUID] = spec.Hosts
			}
		}
		if ownerMap.ApplyMetadata(current) {
			changed = true
		}
		ownerRefsLength := len(current.ObjectMeta.OwnerReferences)
		if ownerRefsLength > 1 {
			// once a certificate is created and gets multiple owners ignore
			// subject changes to prevent flapping subject from differing owner
			// spec.
			spec.Subject = current.Spec.Subject
		}
		spec.Hosts = ownerMap.CombinedHosts()
		if !cmp.Equal(spec, current.Spec, compareSpecUnordered...) {
			current.Spec = spec
			changed = true
		}
		if m.context != nil {
			if current.ObjectMeta.Labels == nil {
				current.ObjectMeta.Labels = map[string]string{}
			}
			if current.ObjectMeta.Annotations == nil {
				current.ObjectMeta.Annotations = map[string]string{}
			}
			if m.context.SetLabels(namespace, name, "Certificate", current.ObjectMeta.Labels) {
				changed = true
			}
			if m.context.SetAnnotations(namespace, name, "Certificate", current.ObjectMeta.Annotations) {
				changed = true
			}
		}
		if !changed {
			return nil
		}
		updated, err := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.logger.Info("Updated certificate", slog.String("namespace", updated.Namespace), slog.String("name", updated.Name))
		m.definitions[key] = updated
		return nil
	} else {
		cert := &skupperv2alpha1.Certificate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "Certificate",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: refs,
				Labels: map[string]string{
					"internal.skupper.io/certificate": "true",
				},
				Annotations: map[string]string{},
			},
			Spec: spec,
		}
		ownerMap := newOwnerMapping(refs, spec.Hosts)
		ownerMap.ApplyMetadata(cert)
		if m.context != nil {
			m.context.SetLabels(namespace, cert.Name, "Certificate", cert.ObjectMeta.Labels)
			m.context.SetAnnotations(namespace, cert.Name, "Certificate", cert.ObjectMeta.Annotations)
		}

		created, err := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(namespace).Create(context.Background(), cert, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = created
		return nil
	}
}

// Called by EventProcessor whenever there is a change to a Certificate reasource.
func (m *CertificateManagerImpl) checkCertificate(key string, certificate *skupperv2alpha1.Certificate) error {
	if certificate == nil {
		return m.certificateDeleted(key)
	}
	ownerMap := certificateToOwnerMapping(certificate)
	if ownerMap.IsControlled {
		// check for deleted owner references
		ownerUIDs := map[string]struct{}{}
		for _, ref := range certificate.OwnerReferences {
			ownerUIDs[string(ref.UID)] = struct{}{}
		}
		for configuredOwner := range ownerMap.PerOwnerHosts {
			if _, ok := ownerUIDs[configuredOwner]; !ok {
				delete(ownerMap.PerOwnerHosts, configuredOwner)
			}
		}
		if ownerMap.ApplyMetadata(certificate) {
			certificate.Spec.Hosts = ownerMap.CombinedHosts()
			updated, err := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(certificate.Namespace).Update(context.Background(), certificate, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
			m.definitions[key] = updated
			return nil
		}

	}
	return m.reconcileSecret(key, certificate, m.secrets[key])
}

// This method does whatever is required to ensure that there is a
// Secret resource corresponding to the supplied CertificateResource.
func (m *CertificateManagerImpl) reconcileSecret(key string, certificate *skupperv2alpha1.Certificate, secret *corev1.Secret) error {

	var err error
	if secret != nil {
		err = m.updateSecret(key, certificate, secret)
	} else {
		err = m.createSecret(key, certificate)
	}
	return m.updateStatus(certificate, err)
}

func (m *CertificateManagerImpl) certificateDeleted(key string) error {
	delete(m.definitions, key)
	if secret, ok := m.secrets[key]; ok {
		err := m.processor.GetKubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		delete(m.secrets, key)
	}
	return nil
}

func (m *CertificateManagerImpl) secretDeleted(key string) error {
	delete(m.secrets, key)
	//TODO
	return nil
}

func (m *CertificateManagerImpl) updateStatus(certificate *skupperv2alpha1.Certificate, err error) error {
	if certificate.SetReady(err) {
		latest, err := m.processor.GetSkupperClient().SkupperV2alpha1().Certificates(certificate.Namespace).UpdateStatus(context.TODO(), certificate, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		certificate = latest
		m.logger.Info("Updated certificate status", slog.String("namespace", certificate.Namespace), slog.String("name", certificate.Name))
		m.definitions[certificate.Key()] = latest
	}
	m.definitions[certificate.Key()] = certificate
	return nil
}

func (m *CertificateManagerImpl) updateSecret(key string, certificate *skupperv2alpha1.Certificate, secret *corev1.Secret) error {
	changed := false
	controlled := isSecretControlled(secret)
	if !isSecretCorrect(certificate, secret) {
		if !controlled {
			return errors.New("secret exists but is not controlled by skupper")
		}

		regenerated, err := m.generateSecret(certificate)
		if err != nil {
			m.logger.Error("Error generating Secret for Certificate",
				slog.String("namespace", certificate.Namespace),
				slog.String("name", secret.Name),
				slog.String("key", key))
			return err
		}
		changed = true
		secret.Data = regenerated.Data
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		secret.Annotations["internal.skupper.io/hosts"] = strings.Join(certificate.Spec.Hosts, ",")
	}
	if m.context != nil && controlled {
		if secret.Labels == nil {
			secret.Labels = map[string]string{}
		}
		if secret.Annotations == nil {
			secret.Annotations = map[string]string{}
		}
		if m.context.SetLabels(certificate.Namespace, secret.Name, "Secret", secret.Labels) {
			changed = true
		}
		if m.context.SetAnnotations(certificate.Namespace, secret.Name, "Secret", secret.Annotations) {
			changed = true
		}
	}
	if !changed {
		return nil
	}

	updated, err := m.processor.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
	if err != nil {
		m.logger.Error("Error updating Secret for Certificate",
			slog.String("namespace", secret.Namespace),
			slog.String("name", secret.Name),
			slog.String("key", key),
			slog.Any("error", err))
		return err
	}
	m.secrets[key] = updated
	m.logger.Info("Updated Secret for Certificate",
		slog.String("namespace", secret.Namespace),
		slog.String("name", secret.Name),
		slog.String("key", key),
		slog.Any("hosts", certificate.Spec.Hosts))
	return nil
}

func (m *CertificateManagerImpl) generateSecret(certificate *skupperv2alpha1.Certificate) (*corev1.Secret, error) {
	var secret *corev1.Secret
	var err error
	if certificate.Spec.Signing {
		secret, err = certs.GenerateSecret(certificate.Name, certificate.Spec.Subject, nil, 0, nil)
		if err != nil {
			return secret, err
		}
	} else {
		expiration := time.Hour * 24 * 365 * 5 // TODO: make this configurable (through controller setting or field on certificate?)
		caKey := fmt.Sprintf("%s/%s", certificate.Namespace, certificate.Spec.Ca)
		ca, ok := m.secrets[caKey]
		if !ok {
			// TODO: no CA exists yet, set error on certificate status
			return nil, fmt.Errorf("CA %q not found", caKey)
		}
		// TODO: handle server and client roles properly
		secret, err = certs.GenerateSecret(certificate.Name, certificate.Spec.Subject, certificate.Spec.Hosts, expiration, ca)
		if err != nil {
			return nil, err
		}
	}
	secret.ObjectMeta.OwnerReferences = ownerReferences(certificate)
	return secret, nil
}

func (m *CertificateManagerImpl) createSecret(key string, certificate *skupperv2alpha1.Certificate) error {
	secret, err := m.generateSecret(certificate)
	if err != nil {
		m.logger.Error("Error generating secret for Certificate",
			slog.String("key", key),
			slog.Any("error", err))
		return err
	}
	secret.Annotations = map[string]string{
		"internal.skupper.io/controlled":  "true",
		"internal.skupper.io/certificate": "true",
		"internal.skupper.io/hosts":       strings.Join(certificate.Spec.Hosts, ","),
	}
	secret.Labels = map[string]string{}

	if m.context != nil {
		m.context.SetLabels(certificate.Namespace, secret.Name, "Secret", secret.Labels)
		m.context.SetAnnotations(certificate.Namespace, secret.Name, "Secret", secret.Annotations)
	}
	m.logger.Info("Creating Secret for Certificate",
		slog.String("namespace", certificate.Namespace),
		slog.String("name", secret.Name),
		slog.String("key", key),
		slog.Any("hosts", certificate.Spec.Hosts))
	created, err := m.processor.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		m.logger.Error("Error creating Secret for Certificate",
			slog.String("namespace", certificate.Namespace),
			slog.String("name", secret.Name),
			slog.String("key", key),
			slog.Any("error", err))
		return err
	}
	m.secrets[key] = created
	m.logger.Info("Created Secret for Certificate",
		slog.String("namespace", certificate.Namespace),
		slog.String("name", secret.Name),
		slog.String("key", key),
		slog.Any("hosts", certificate.Spec.Hosts))
	return nil
}

// Called by EventProcessor whenever there is a change in a relevant
// Secret resource.
func (m *CertificateManagerImpl) checkSecret(key string, secret *corev1.Secret) error {
	if secret == nil {
		return m.secretDeleted(key)
	}
	m.secrets[key] = secret
	if definition, ok := m.definitions[key]; ok {
		return m.reconcileSecret(key, definition, secret)
	}

	return nil
}

func isSecretCorrect(certificate *skupperv2alpha1.Certificate, secret *corev1.Secret) bool {
	data, ok := secret.Data["tls.crt"]
	if !ok {
		return false
	}
	cert, err := certs.DecodeCertificate(data)
	if err != nil {
		slog.Error("Bad certificate secret", slog.String("key", certificate.Key()), slog.Any("error", err))
		return false
	}
	if time.Now().After(cert.NotAfter) {
		slog.Info("Certificate has expired", slog.String("key", certificate.Key()))
		return false
	}
	if certificate.Spec.Subject != cert.Subject.CommonName {
		return false
	}
	validFor := map[string]string{}
	for _, host := range cert.DNSNames {
		// Ignore empty DNSNames - GH-2277
		if host == "" {
			continue
		}
		validFor[host] = host
	}
	for _, ip := range cert.IPAddresses {
		validFor[ip.String()] = ip.String()
	}
	if len(certificate.Spec.Hosts) != len(validFor) {
		return false
	}
	for _, host := range certificate.Spec.Hosts {
		if _, ok := validFor[host]; !ok {
			return false
		}
	}
	return true
}

func isSecretControlled(secret *corev1.Secret) bool {
	return hasControlledAnnotation(secret) || hasCertificateOwner(secret)
}

func hasControlledAnnotation(secret *corev1.Secret) bool {
	if secret.Annotations == nil {
		return false
	}
	_, ok := secret.Annotations["internal.skupper.io/controlled"]
	return ok
}

func hasCertificateOwner(secret *corev1.Secret) bool {
	for _, owner := range secret.ObjectMeta.OwnerReferences {
		if owner.Kind == "Certificate" && owner.APIVersion == "skupper.io/v2alpha1" {
			return true
		}
	}
	return false
}

func secretKey(secret *corev1.Secret) string {
	return fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
}

func ownerReferences(cert *skupperv2alpha1.Certificate) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			Kind:       "Certificate",
			APIVersion: "skupper.io/v2alpha1",
			Name:       cert.Name,
			UID:        cert.ObjectMeta.UID,
		},
	}
}

func mergeOwnerReferences(obj *metav1.ObjectMeta, added []metav1.OwnerReference) bool {
	changed := false
	byUid := map[types.UID]metav1.OwnerReference{}
	original := obj.OwnerReferences
	for _, ref := range original {
		byUid[ref.UID] = ref
	}
	for _, ref := range added {
		if actual, ok := byUid[ref.UID]; !ok || actual != ref {
			original = append(original, ref)
			changed = true
		}
	}
	if changed {
		obj.OwnerReferences = original
	}
	return changed
}
