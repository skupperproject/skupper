package client

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/ajssmith/skupper/api/types"
	"github.com/ajssmith/skupper/pkg/kube"
	"github.com/ajssmith/skupper/pkg/qdr"
	"github.com/ajssmith/skupper/pkg/utils/configs"
)

func removeConnector(name string, list []types.Connector) (bool, []types.Connector) {
	updated := []types.Connector{}
	found := false
	for _, c := range list {
		if c.Name != name {
			updated = append(updated, c)
		} else {
			found = true
		}
	}
	return found, updated
}

func (cli *VanClient) VanConnectorRemove(ctx context.Context, name string) error {
	current, err := kube.GetDeployment(types.QdrDeploymentName, cli.Namespace, cli.KubeClient)
	if err == nil {
		mode := qdr.GetQdrMode(current)
		found, connectors := removeConnector(name, qdr.ListRouterConnectors(mode, cli.Namespace, cli.KubeClient))
		if found {
			// TODO: Do the following as qdr.RemoveConnector
			config := kube.FindEnvVar(current.Spec.Template.Spec.Containers[0].Env, types.QdrEnvConfig)
			if config == nil {
				fmt.Println("Could not retrieve qdr config")
			} else {
				pattern := "## Connectors: ##"
				updated := strings.Split(config.Value, pattern)[0] + pattern
				for _, c := range connectors {
					updated += configs.ConnectorConfig(&c)
				}
				kube.SetEnvVarForDeployment(current, types.QdrEnvConfig, updated)
				kube.RemoveSecretVolumeForDeployment(name, current, 0)
				kube.DeleteSecret(name, cli.Namespace, cli.KubeClient)
				_, err = cli.KubeClient.AppsV1().Deployments(cli.Namespace).Update(current)
				if err != nil {
					fmt.Println("Failed to remove connection:", err.Error())
				}
			}
		}
	} else if errors.IsNotFound(err) {
		fmt.Println("Skupper not enabled in: ", cli.Namespace)
	} else {
		fmt.Println("Failed to retrieve qdr deployment: ", err.Error())
	}
	return err
}
