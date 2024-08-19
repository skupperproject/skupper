package grants

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

type AutoConfigure struct {
	port                 int
	podname              string
	tlsCredentialsSecret string
	ownerRefs            []metav1.OwnerReference
	selector             map[string]string
}

func (s *AutoConfigure) getConfigurationFromPod(clients internalclient.Clients, namespace string) error {
	pod, err := clients.GetKubeClient().CoreV1().Pods(namespace).Get(context.TODO(), s.podname, metav1.GetOptions{})
	if err != nil {
		return err
	}
	s.ownerRefs = pod.ObjectMeta.OwnerReferences
	s.selector = pod.ObjectMeta.Labels
	return nil
}

func (s *AutoConfigure) ensureCert(clients internalclient.Clients, namespace string, desired *skupperv1alpha1.Certificate) error {
	existing, err := clients.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = clients.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Create(context.Background(), desired, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	}
	changed := false
	if !reflect.DeepEqual(existing.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		changed = true
		existing.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
	}
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		changed = true
		existing.Spec = desired.Spec
	}
	if !changed {
		return nil
	}
	_, err = clients.GetSkupperClient().SkupperV1alpha1().Certificates(namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
	return err
}

func (s *AutoConfigure) ensureSecuredAccess(clients internalclient.Clients, namespace string, desired *skupperv1alpha1.SecuredAccess) error {
	existing, err := clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(namespace).Get(context.Background(), desired.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(namespace).Create(context.Background(), desired, metav1.CreateOptions{})
		return err
	} else if err != nil {
		return err
	}
	changed := false
	if !reflect.DeepEqual(existing.ObjectMeta.OwnerReferences, desired.ObjectMeta.OwnerReferences) {
		changed = true
		existing.ObjectMeta.OwnerReferences = desired.ObjectMeta.OwnerReferences
	}
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		changed = true
		existing.Spec = desired.Spec
	}
	if !changed {
		return nil
	}
	_, err = clients.GetSkupperClient().SkupperV1alpha1().SecuredAccesses(namespace).Update(context.Background(), existing, metav1.UpdateOptions{})
	return err
}

func (s *AutoConfigure) configure(clients internalclient.Clients, namespace string) error {
	if err := s.getConfigurationFromPod(clients, namespace); err != nil {
		return err
	}

	cert := &skupperv1alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "skupper-grant-server-ca",
			OwnerReferences: s.ownerRefs,
		},
		Spec: skupperv1alpha1.CertificateSpec{
			Ca:      "",
			Subject: "SkupperGrantServerCA",
			Signing: true,
			Client:  false,
			Server:  false,
		},
	}
	if err := s.ensureCert(clients, namespace, cert); err != nil {
		return err
	}

	sa := &skupperv1alpha1.SecuredAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v1alpha1",
			Kind:       "SecuredAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "skupper-grant-server",
			OwnerReferences: s.ownerRefs,
		},
		Spec: skupperv1alpha1.SecuredAccessSpec{
			Selector: s.selector,
			Ports: []skupperv1alpha1.SecuredAccessPort{
				{
					Name: "https",
					Port: s.port,
				},
			},
			Issuer:      "skupper-grant-server-ca",
			Certificate: s.tlsCredentialsSecret,
		},
	}
	if err := s.ensureSecuredAccess(clients, namespace, sa); err != nil {
		return err
	}
	return nil
}

func newAutoConfigure(handler kube.SecuredAccessHandler, controller *kube.Controller, currentNamespace string, config *GrantConfig) (*AutoConfigure, error) {
	ac := &AutoConfigure{
		port:                 config.Port,
		tlsCredentialsSecret: config.TlsCredentialsSecret,
		podname:              config.Hostname,
	}
	if ac.tlsCredentialsSecret == "" {
		//TODO: should setting TlsCredentialsSecret be allowed when auto configure is enabled?
		ac.tlsCredentialsSecret = "skupper-grant-server"
	}
	if err := ac.configure(controller, currentNamespace); err != nil {
		return nil, fmt.Errorf("Error creating resources for grant server: %s", err)
	}
	controller.WatchSecuredAccessesWithOptions(kube.SkupperResourceByName("skupper-grant-server"), currentNamespace, handler)
	return ac, nil
}
