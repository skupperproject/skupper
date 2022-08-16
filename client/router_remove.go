package client

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
)

// RouterRemove delete a VAN (router and controller) deployment
func (cli *VanClient) RouterRemove(ctx context.Context) error {
	err := cli.DeploymentManager(cli.Namespace).DeleteDeployment(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: types.TransportDeploymentName}}, &metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Skupper not installed in '"+cli.Namespace+"': %w", err)
		} else {
			return fmt.Errorf("Error while trying to delete: %w", err)
		}
	}
	return nil
}
