package client

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) ServiceCACreate(ownerRef *metav1.OwnerReference) error {

	ca := types.CertAuthority{Name: types.ServiceCaSecret}

	caSecret, err := kube.NewCertAuthority(ca, ownerRef, cli.Namespace, cli.KubeClient)

	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	_, err = kube.NewSimpleSecret(types.ServiceClientSecret, caSecret, ownerRef, cli.Namespace, cli.KubeClient)

	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
