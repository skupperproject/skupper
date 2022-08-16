package client

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) SiteConfigRemove(ctx context.Context) error {
	return cli.ConfigMapManager(cli.Namespace).DeleteConfigMap(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: types.SiteConfigMapName}}, &metav1.DeleteOptions{})
}
