package client

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ajssmith/skupper/api/types"
)

// VanRouterRemove delete a VAN (router and controller) deployment
func (cli *VanClient) VanRouterRemove(ctx context.Context) error {
	err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Delete(types.QdrDeploymentName, &metav1.DeleteOptions{})
	if err == nil {
		fmt.Println("Skupper is now removed from '" + cli.Namespace + "'.")
	} else if errors.IsNotFound(err) {
		fmt.Println("Skupper not installed in '" + cli.Namespace + "'.")
	} else {
		fmt.Println("Error while trying to delete:", err.Error())
	}
	return err
}
