package client

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"k8s.io/apimachinery/pkg/api/errors"
)

func (cli *VanClient) CASiteCreate(options types.SiteConfig) error {

	ca := types.CertAuthority{Name: options.Reference.UID}
	siteOwnerRef := asOwnerReference(options.Reference)

	_, err := kube.NewCertAuthority(ca, siteOwnerRef, cli.Namespace, cli.KubeClient)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
