package certificates

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
)

type CertificateManager interface {
	EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error
	Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error
}

type CertificateManagerImpl struct {
	definitions        map[string]*skupperv1alpha1.Certificate
	secrets            map[string]*corev1.Secret
	certificateWatcher *kube.CertificateWatcher
	secretWatcher      *kube.SecretWatcher
	controller         *kube.Controller
}

func NewCertificateManager(controller *kube.Controller) *CertificateManagerImpl {
	return &CertificateManagerImpl{
		definitions: map[string]*skupperv1alpha1.Certificate{},
		secrets:     map[string]*corev1.Secret{},
		controller:  controller,
	}
}

func (m *CertificateManagerImpl) Watch(watchNamespace string) {
	m.certificateWatcher = m.controller.WatchCertificates(watchNamespace, m.checkCertificate)
	m.secretWatcher = m.controller.WatchAllSecrets(watchNamespace, m.checkSecret)
}

func (m *CertificateManagerImpl) Recover() {
	for _, secret := range m.secretWatcher.List() {
		m.secrets[secretKey(secret)] = secret
	}
	for _, cert := range m.certificateWatcher.List() {
		if err := m.checkCertificate(cert.Key(), cert); err != nil {
			log.Printf("Error trying to reconcile %s: %s", cert.Key(), err)
		}
	}
}

func (m *CertificateManagerImpl) EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error {
	spec := skupperv1alpha1.CertificateSpec{
		Subject: subject,
		Signing: true,
	}
	return m.ensure(namespace, name, spec, refs)
}

func (m *CertificateManagerImpl) Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error {
	spec := skupperv1alpha1.CertificateSpec{
		Ca:      ca,
		Subject: subject,
		Hosts:   hosts,
		Client:  client,
		Server:  server,
	}
	return m.ensure(namespace, name, spec, refs)
}

func (m *CertificateManagerImpl) definitionUpdated(key string, def *skupperv1alpha1.Certificate) {
}

func (m *CertificateManagerImpl) ensure(namespace string, name string, spec skupperv1alpha1.CertificateSpec, refs []metav1.OwnerReference) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if current, ok := m.definitions[key]; ok {
		if !mergeOwnerReferences(current.ObjectMeta.OwnerReferences, refs) && reflect.DeepEqual(spec, current.Spec) {
			return nil
		}
		// merge hosts as the certificate may be shared by sources each requiring different sets of hosts:
		hosts := getHostChanges(getPreviousHosts(current, refs), spec.Hosts, key).apply(current.Spec.Hosts)
		current.Spec = spec
		current.Spec.Hosts = hosts
		updated, err := m.controller.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = updated
		return nil
	} else {
		cert := &skupperv1alpha1.Certificate{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
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
		created, err := m.controller.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Create(context.Background(), cert, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		m.definitions[key] = created
		return nil
	}
}

func (m *CertificateManagerImpl) checkCertificate(key string, certificate *skupperv1alpha1.Certificate) error {
	if certificate == nil {
		return m.certificateDeleted(key)
	}
	if secret, ok := m.secrets[key]; ok {
		return m.reconcile(key, certificate, secret)
	} else {
		return m.reconcile(key, certificate, nil)
	}
}

func (m *CertificateManagerImpl) reconcile(key string, certificate *skupperv1alpha1.Certificate, secret *corev1.Secret) error {
	if secret != nil {
		if err := m.updateSecret(key, certificate, secret); err != nil {
			return m.updateStatus(certificate, err)
		}
	} else {
		if err := m.createSecret(key, certificate); err != nil {
			return m.updateStatus(certificate, err)
		}
	}
	m.definitionUpdated(key, certificate)
	return m.updateStatus(certificate, nil)
}

func (m *CertificateManagerImpl) certificateDeleted(key string) error {
	delete(m.definitions, key)
	if secret, ok := m.secrets[key]; ok {
		err := m.controller.GetKubeClient().CoreV1().Secrets(secret.Namespace).Delete(context.Background(), secret.Name, metav1.DeleteOptions{})
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

func (m *CertificateManagerImpl) updateStatus(certificate *skupperv1alpha1.Certificate, err error) error {
	if err == nil {
		certificate.Status.StatusMessage = skupperv1alpha1.STATUS_OK
	} else {
		certificate.Status.StatusMessage = err.Error()
	}
	latest, err := m.controller.GetSkupperClient().SkupperV1alpha1().Certificates(certificate.Namespace).UpdateStatus(context.TODO(), certificate, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	m.definitions[certificate.Key()] = latest
	return nil
}

func (m *CertificateManagerImpl) updateSecret(key string, certificate *skupperv1alpha1.Certificate, secret *corev1.Secret) error {
	if !isSecretCorrect(certificate, secret) {
		if !isSecretControlled(secret) {
			return errors.New("Secret exists but is not controlled by skupper")
		}

		secret, err := m.generateSecret(certificate)
		if err != nil {
			log.Printf("Error generating Secret %s/%s for Certificate %s", certificate.Namespace, secret.Name, key)
			return err
		}
		updated, err := m.controller.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			log.Printf("Error updating Secret %s/%s for Certificate %s: %s", secret.Namespace, secret.Name, key, err)
			return err
		}
		m.secrets[key] = updated
		log.Printf("Updated Secret %s/%s for Certificate %s (hosts %v)", secret.Namespace, secret.Name, key, certificate.Spec.Hosts)
	}
	return nil
}

func (m *CertificateManagerImpl) generateSecret(certificate *skupperv1alpha1.Certificate) (*corev1.Secret, error) {
	var secret corev1.Secret
	if certificate.Spec.Signing {
		secret = certs.GenerateCASecret(certificate.Name, certificate.Spec.Subject)
	} else {
		expiration := time.Hour * 24 * 365 * 5 // TODO: make this configurable (through controller setting or field on certificate?)
		caKey := fmt.Sprintf("%s/%s", certificate.Namespace, certificate.Spec.Ca)
		ca, ok := m.secrets[caKey]
		if !ok {
			// TODO: no CA exists yet, set error on certificate status
			return nil, fmt.Errorf("CA %q not found", caKey)
		}
		// TODO: handle server and client roles properly
		secret = certs.GenerateSecretWithExpiration(certificate.Name, certificate.Spec.Subject, strings.Join(certificate.Spec.Hosts, ","), expiration, ca)
	}
	//TODO: add labels and annotations from certificate to secret
	secret.ObjectMeta.OwnerReferences = ownerReferences(certificate)
	return &secret, nil
}

func (m *CertificateManagerImpl) createSecret(key string, certificate *skupperv1alpha1.Certificate) error {
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

	log.Printf("Creating Secret %s/%s for Certificate %s for hosts %v", certificate.Namespace, secret.Name, key, certificate.Spec.Hosts)
	created, err := m.controller.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		log.Printf("Error creating Secret %s/%s for Certificate %s: %s", certificate.Namespace, secret.Name, key, err)
		return err
	}
	m.secrets[key] = created
	log.Printf("Created Secret %s/%s for Certificate %s (hosts %v)", certificate.Namespace, secret.Name, key, certificate.Spec.Hosts)
	return nil
}

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

func isSecretCorrect(certificate *skupperv1alpha1.Certificate, secret *corev1.Secret) bool {
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
	if secret.Annotations == nil {
		return false
	}
	_, ok := secret.Annotations["internal.skupper.io/controlled"]
	return ok
}

func secretKey(secret *corev1.Secret) string {
	return fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
}

func ownerReferences(cert *skupperv1alpha1.Certificate) []metav1.OwnerReference {
	return []metav1.OwnerReference{
		{
			Kind:       "Certificate",
			APIVersion: "skupper.io/v1alpha1",
			Name:       cert.Name,
			UID:        cert.ObjectMeta.UID,
		},
	}
}

func mergeOwnerReferences(original []metav1.OwnerReference, added []metav1.OwnerReference) bool {
	changed := false
	byUid := map[types.UID]metav1.OwnerReference{}
	for _, ref := range original {
		byUid[ref.UID] = ref
	}
	for _, ref := range added {
		if actual, ok := byUid[ref.UID]; !ok || actual != ref {
			original = append(original, ref)
			changed = true
		}
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

func getPreviousHosts(cert *skupperv1alpha1.Certificate, refs []metav1.OwnerReference) map[string]bool {
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
