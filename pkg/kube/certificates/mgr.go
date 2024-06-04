package certificates

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers/internalinterfaces"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
)

func options() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/certificate"
	}
}

type CertificateManager interface {
	EnsureCA(namespace string, name string, subject string, refs []metav1.OwnerReference) error
	Ensure(namespace string, name string, ca string, subject string, hosts []string, client bool, server bool, refs []metav1.OwnerReference) error
}

type CertificateManagerImpl struct {
	definitions        map[string]*skupperv1alpha1.Certificate
	changed            map[string]bool
	secrets            map[string]*corev1.Secret
	certificateWatcher *kube.CertificateWatcher
	secretWatcher      *kube.SecretWatcher
	controller         *kube.Controller
}

func NewCertificateManager(controller *kube.Controller) *CertificateManagerImpl {
	return &CertificateManagerImpl{
		definitions: map[string]*skupperv1alpha1.Certificate{},
		changed:     map[string]bool{},
		secrets:     map[string]*corev1.Secret{},
		controller:  controller,
	}
}

func (m *CertificateManagerImpl) Watch(watchNamespace string) {
	m.certificateWatcher = m.controller.WatchCertificates(watchNamespace, m.checkCertificate)
	m.secretWatcher = m.controller.WatchSecrets(options(), watchNamespace, m.checkSecret)
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
	m.definitions[key] = def
	m.changed[key] = true
}

func (m *CertificateManagerImpl) ensure(namespace string, name string, spec skupperv1alpha1.CertificateSpec, refs []metav1.OwnerReference) error {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if current, ok := m.definitions[key]; ok {
		if reflect.DeepEqual(spec, current.Spec) {
			return nil
		}
		current.Spec = spec
		updated, err := m.controller.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Update(context.Background(), current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.definitionUpdated(key, updated)
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
		created, err := m.controller.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Create(context.Background(), cert, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		m.definitionUpdated(key, created)
		return nil
	}
}
func (m *CertificateManagerImpl) checkChanged(key string) bool {
	if _, ok := m.changed[key]; !ok {
		return false
	}
	delete(m.changed, key)
	return true
}

func (m *CertificateManagerImpl) checkCertificate(key string, certificate *skupperv1alpha1.Certificate) error {
	log.Printf("Checking Certificate %s", key)
	if certificate == nil {
		return m.certificateDeleted(key)
	}
	if existing, ok := m.definitions[key]; ok && reflect.DeepEqual(existing.Spec, certificate.Spec) && !m.checkChanged(key) {
		log.Printf("Certificate %s is unchanged", key)
		return nil
	}
	if secret, ok := m.secrets[key]; ok {
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
		certificate.Status.Status = "Ok"
	} else {
		certificate.Status.Status = err.Error()
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
		secret, err := m.generateSecret(certificate)
		if err != nil {
			log.Printf("Error generating secret %s/%s for Certificate %s", certificate.Namespace, secret.Name, key)
			return err
		}
		log.Printf("Updating secret %s/%s for Certificate %s for hosts %v", secret.Namespace, secret.Name, key, certificate.Spec.Hosts)
		updated, err := m.controller.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Update(context.TODO(), secret, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		m.secrets[key] = updated
	} else {
		log.Printf("Secret %s/%s matches Certificate %s", secret.Namespace, secret.Name, key)
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
	log.Printf("Creating secret %s/%s for Certificate %s for hosts %v", certificate.Namespace, secret.Name, key, certificate.Spec.Hosts)
	created, err := m.controller.GetKubeClient().CoreV1().Secrets(certificate.Namespace).Create(context.TODO(), secret, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	m.secrets[key] = created
	return nil
}

func (m *CertificateManagerImpl) checkSecret(key string, secret *corev1.Secret) error {
	if secret == nil {
		return m.secretDeleted(key)
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
