package client

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func isToken(secret *corev1.Secret) bool {
	typename, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]
	return ok && (typename == types.TypeClaimRequest || typename == types.TypeToken)
}

func (cli *VanClient) ConnectorRemove(ctx context.Context, options types.ConnectorRemoveOptions) error {
	secret, err := cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).Get(options.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) || (err == nil && !isToken(secret)) {
		return fmt.Errorf("No such link %q", options.Name)
	} else if err != nil {
		return err
	}
	return kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)
}
