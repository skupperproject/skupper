package client

import (
	"context"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	kubeqdr "github.com/skupperproject/skupper/pkg/kube/qdr"
	"github.com/skupperproject/skupper/pkg/qdr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) ServiceInterfaceCreate(ctx context.Context, service *types.ServiceInterface) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.Service(service.Address)
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	owner, err := getRootObject(cli)
	if err == nil {
		err = validateServiceInterface(service, cli)
		if err != nil {
			return err
		}

		tlsSupport := kubeqdr.TlsSupport{Address: service.Address, Credentials: service.TlsCredentials}
		err = kubeqdr.EnableTlsSupport(tlsSupport, cli.getSecret, cli.getConfigMap, cli.newSecret, cli.addSslProfile, cli.existsSslProfile)
		if err != nil {
			return err
		}

		return updateServiceInterface(service, false, owner, cli)
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	} else {
		return err
	}
}

func (cli *VanClient) getSecret(name string) (*corev1.Secret, error) {
	return cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(name, metav1.GetOptions{})
}

func (cli *VanClient) getConfigMap() (*corev1.ConfigMap, error) {
	return cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.TransportConfigMapName, metav1.GetOptions{})
}

func (cli *VanClient) newSecret(credential types.Credential, ownerReference *metav1.OwnerReference) (*corev1.Secret, error) {
	return kube.NewSecret(credential, ownerReference, cli.Namespace, cli.KubeClient)
}

func (cli *VanClient) addSslProfile(sslProfile qdr.SslProfile) error {
	return kubeqdr.AddSslProfile(sslProfile, cli.Namespace, cli.KubeClient)
}

func (cli *VanClient) existsSslProfile(sslProfile string) (bool, error) {
	return kubeqdr.ExistsSslProfile(sslProfile, cli.Namespace, cli.KubeClient)
}
