package client

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils/configs"
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

func (cli *VanClient) VanConnectorRemove(ctx context.Context, options types.VanConnectorRemoveOptions) error {
	current, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
	if err == nil {
		mode := qdr.GetTransportMode(current)
		found, connectors := removeConnector(options.Name, qdr.ListRouterConnectors(mode, options.SkupperNamespace, cli.KubeClient))
		if found || options.ForceCurrent {
			// TODO: Do the following as qdr.RemoveConnector
			config := kube.FindEnvVar(current.Spec.Template.Spec.Containers[0].Env, types.TransportEnvConfig)
			if config == nil {
				fmt.Println("Could not retrieve transport config")
			} else {
				pattern := "## Connectors: ##"
				updated := strings.Split(config.Value, pattern)[0] + pattern
				for _, c := range connectors {
					updated += configs.ConnectorConfig(&c)
				}
				kube.SetEnvVarForDeployment(current, types.TransportEnvConfig, updated)
				kube.RemoveSecretVolumeForDeployment(options.Name, current, 0)
				kube.DeleteSecret(options.Name, options.SkupperNamespace, cli.KubeClient)
				_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(current)
				if err != nil {
					fmt.Println("Failed to remove connection:", err.Error())
				}
			}
		}
	} else if errors.IsNotFound(err) {
		fmt.Println("Skupper not enabled in: ", options.SkupperNamespace)
	} else {
		fmt.Println("Failed to retrieve transport deployment: ", err.Error())
	}
	return err
}
