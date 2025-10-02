// Package certificates provides the ability to create or update
// instances of the v2alpha1 Certificate resource, and ensure that a
// corresponding Secret resource is maintained for each.
package certificates

import (
	"context"
	"errors"
	"fmt"
	"log"
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
}

// Returns a correctly initialised CertificateManager.
func NewCertificateManager(processor *watchers.EventProcessor) *CertificateManagerImpl {
	return &CertificateManagerImpl{
		definitions: map[string]*skupperv2alpha1.Certificate{},
		secrets:     map[string]*corev1.Secret{},
		processor:   processor,
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
			log.Printf("Error trying to reconcile %s: %s", cert.Key(), err)
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
		if mergeOwnerReferences(&current.ObjectMeta, refs) {
			changed = true
		}
		ownerRefsLength := len(current.ObjectMeta.OwnerReferences)
		specHostsCsv := strings.Join(spec.Hosts, ",")
		specHosts := spec.Hosts
		if ownerRefsLength > 1 {
			spec.Subject = current.Spec.Subject
			specHosts = append(specHosts, current.Spec.Hosts...)
		}
		// merge hosts as the certificate may be shared by sources each requiring different sets of hosts:
		spec.Hosts = getHostChanges(getPreviousHosts(current, refs), specHosts, key).apply(current.Spec.Hosts)
		if !cmp.Equal(spec, current.Spec, compareSpecUnordered...) {
			current.Spec = spec
			if current.Annotations == nil {
				current.Annotations = map[string]string{}
			}
			if len(refs) > 0 {
				current.ObjectMeta.Annotations["internal.skupper.io/hosts-"+string(refs[0].UID)] = specHostsCsv
			}
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
		log.Printf("Updated certificate %s/%s", updated.Namespace, updated.Name)
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
				Annotations: map[string]string{
					"internal.skupper.io/controlled": "true",
				},
			},
			Spec: spec,
		}
		if len(refs) > 0 {
			cert.ObjectMeta.Annotations["internal.skupper.io/hosts-"+string(refs[0].UID)] = strings.Join(spec.Hosts, ",")
		}
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
	if secret, ok := m.secrets[key]; ok {
		return m.reconcile(key, certificate, secret)
	} else {
		return m.reconcile(key, certificate, nil)
	}
}

// This method does whatever is required to ensure that there is a
// Secret resource corresponding to the supplied CertificateResource.
func (m *CertificateManagerImpl) reconcile(key string, certificate *skupperv2alpha1.Certificate, secret *corev1.Secret) error {
	if secret != nil {
		if err := m.updateSecret(key, certificate, secret); err != nil {
			return m.updateStatus(certificate, err)
		}
	} else {
		if err := m.createSecret(key, certificate); err != nil {
			return m.updateStatus(certificate, err)
		}
	}
	return m.updateStatus(certificate, nil)
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
		log.Printf("Updated certificate status %s/%s", certificate.Namespace, certificate.Name)
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
			return errors.New("Secret exists but is not controlled by skupper")
		}

		regenerated, err := m.generateSecret(certificate)
		if err != nil {
			log.Printf("Error generating Secret %s/%s for Certificate %s", certificate.Namespace, secret.Name, key)
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
		log.Printf("Error updating Secret %s/%s for Certificate %s: %s", secret.Namespace, secret.Name, key, err)
		return err
	}
	m.secrets[key] = updated
	log.Printf("Updated Secret %s/%s for Certificate %s (hosts %v)", secret.Namespace, secret.Name, key, certificate.Spec.Hosts)
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
		log.Printf("Error generating secret for Certificate %s: %s", key, err)
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
	log.Printf("Creating Secret %s/%s for Certificate %s for hosts %v", certificate.Namespace, secret.Name, key, certificate.Spec.Hosts)
	created, err := m.processor.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error creating Secret %s/%s for Certificate %s: %s", certificate.Namespace, secret.Name, key, err)
		return err
	}
	m.secrets[key] = created
	log.Printf("Created Secret %s/%s for Certificate %s (hosts %v)", certificate.Namespace, secret.Name, key, certificate.Spec.Hosts)
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
		return m.reconcile(key, definition, secret)
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
		log.Printf("Bad certificate secret %s: %s", certificate.Key(), err)
		return false
	}
	if time.Now().After(cert.NotAfter) {
		log.Printf("Certificate %s has expired", certificate.Key())
		return false
	}
	if certificate.Spec.Subject != cert.Subject.CommonName {
		return false
	}
	validFor := map[string]string{}
	for _, host := range cert.DNSNames {
		validFor[host] = host
	}
	for _, ip := range cert.IPAddresses {
		validFor[ip.String()] = ip.String()
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

type HostChanges struct {
	key       string
	additions []string
	deletions []string
}

func (changes *HostChanges) apply(original []string) []string {
	changed := false
	index := map[string]bool{}
	for _, value := range original {
		index[value] = true
	}
	for _, host := range changes.additions {
		if _, ok := index[host]; !ok {
			index[host] = true
			changed = true
		}
	}
	for _, host := range changes.deletions {
		if _, ok := index[host]; ok {
			delete(index, host)
			changed = true
		}
	}
	if !changed {
		return original
	}
	var hosts []string
	for key, _ := range index {
		hosts = append(hosts, key)
	}
	log.Printf("Changing hosts for Certificate %s from %v to %v", changes.key, original, hosts)
	return hosts
}

func getPreviousHosts(cert *skupperv2alpha1.Certificate, refs []metav1.OwnerReference) map[string]bool {
	if len(refs) > 0 {
		if value, ok := cert.ObjectMeta.Annotations["internal.skupper.io/hosts-"+string(refs[0].UID)]; ok {
			hosts := map[string]bool{}
			for _, value := range strings.Split(value, ",") {
				hosts[value] = true
			}
			return hosts
		}
	}
	return nil
}

func getHostChanges(previous map[string]bool, current []string, key string) *HostChanges {
	changes := &HostChanges{
		key: key,
	}
	if len(previous) > 0 {
		for _, value := range current {
			if _, ok := previous[value]; ok {
				delete(previous, value)
			} else {
				changes.additions = append(changes.additions, value)
			}
		}
		for value, _ := range previous {
			changes.deletions = append(changes.deletions, value)
		}
	} else {
		changes.additions = current
	}
	return changes
}
