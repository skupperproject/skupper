package client

import (
	"context"
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	"k8s.io/apimachinery/pkg/api/errors"
)

// RouterRemove delete a VAN (router and controller) deployment
func (cli *VanClient) RouterRemove(ctx context.Context) error {
	err := cli.DeploymentManager(cli.Namespace).DeleteDeployment(types.TransportDeploymentName)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("Skupper not installed in '"+cli.Namespace+"': %w", err)
		} else {
			return fmt.Errorf("Error while trying to delete: %w", err)
		}
	}
	return nil
}
