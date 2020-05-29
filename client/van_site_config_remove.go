package client

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) VanSiteConfigRemove(ctx context.Context) error {
	return cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Delete("skupper-site", &metav1.DeleteOptions{})
}
