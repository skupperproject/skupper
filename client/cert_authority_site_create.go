package client

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) CASiteCreate(ownerRef *metav1.OwnerReference) error {

	ca := types.CertAuthority{Name: types.SiteCaServicesSecret}

	_, err := kube.NewCertAuthority(ca, ownerRef, cli.Namespace, cli.KubeClient)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
