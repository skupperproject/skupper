package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

// VanRouterRemove delete a VAN (router and controller) deployment
func (cli *VanClient) VanRouterRemove(ctx context.Context) error {
	err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Delete(types.DefaultSiteName, &metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Skupper not installed in '"+cli.Namespace+"': %w", err)
		} else {
			return fmt.Errorf("Error while trying to delete: %w", err)
		}
	}
	return nil
}
