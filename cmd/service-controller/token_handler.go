package main

import (
	"context"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type TokenHandler struct {
	name      string
	vanClient *client.VanClient
	siteId    string
	policy    *client.ClusterPolicyValidator
}

func (h *TokenHandler) Handle(name string, token *corev1.Secret) error {
	if token != nil {
		if h.isTokenValidInSite(token) {
			if h.isTokenDisabled(token) {
				return h.disconnect(name)
			} else {
				return h.connect(token)
			}
		}
		return nil
	} else {
		return h.disconnect(name)
	}
}

func newTokenHandler(cli *client.VanClient, siteId string) *SecretController {
	handler := &TokenHandler{
		name:      "TokenHandler",
		vanClient: cli,
		siteId:    siteId,
		policy:    client.NewClusterPolicyValidator(cli),
	}
	AddStaticPolicyWatcher(handler.policy)
	return NewSecretController(handler.name, types.TypeTokenQualifier, cli.KubeClient, cli.Namespace, handler)
}

func (c *TokenHandler) getTokenCost(token *corev1.Secret) (int32, bool) {
	if token.ObjectMeta.Annotations == nil {
		return 0, false
	}
	if costString, ok := token.ObjectMeta.Annotations[types.TokenCost]; ok {
		cost, err := strconv.Atoi(costString)
		if err != nil {
			event.Recordf(c.name, "Ignoring invalid cost annotation %q", costString)
			return 0, false
		}
		return int32(cost), true
	}
	return 0, false
}

func (c *TokenHandler) connect(token *corev1.Secret) error {
	event.Recordf(c.name, "Connecting using token %s", token.ObjectMeta.Name)
	var options types.ConnectorCreateOptions
	options.Name = token.ObjectMeta.Name
	options.SkupperNamespace = c.vanClient.Namespace
	if cost, ok := c.getTokenCost(token); ok {
		options.Cost = cost
	}
	return c.vanClient.ConnectorCreate(context.Background(), token, options)
}

func (c *TokenHandler) disconnect(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	event.Recordf(c.name, "Disconnecting connector %s", name)
	return c.removeConnectorFromConfig(context.Background(), name)
}

func (c *TokenHandler) removeConnectorFromConfig(ctx context.Context, name string) error {
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}
	updated, connector := current.RemoveConnector(name)
	if connector.SslProfile != "" && current.RemoveSslProfile(connector.SslProfile) {
		updated = true
	}
	if updated {
		_, err := current.UpdateConfigMap(configmap)
		if err != nil {
			return err
		}
		_, err = c.vanClient.KubeClient.CoreV1().ConfigMaps(c.vanClient.Namespace).Update(configmap)
		if err != nil {
			return err
		}
	}
	deployment, err := kube.GetDeployment(types.TransportDeploymentName, c.vanClient.Namespace, c.vanClient.KubeClient)
	if err != nil {
		return err
	}
	kube.RemoveSecretVolumeForDeployment(name, deployment, 0)
	_, err = c.vanClient.KubeClient.AppsV1().Deployments(c.vanClient.Namespace).Update(deployment)
	if err != nil {
		return err
	}
	return nil
}

func (c *TokenHandler) isTokenValidInSite(token *corev1.Secret) bool {
	if author, ok := token.ObjectMeta.Annotations[types.TokenGeneratedBy]; ok && author == c.siteId {
		// token was generated by this site so should not be applied
		return false
	} else {
		return true
	}
}

func (c *TokenHandler) isTokenDisabled(token *corev1.Secret) bool {
	// validate if host is still allowed
	hostname := token.ObjectMeta.Annotations["inter-router-host"]
	r, err := c.vanClient.RouterInspect(context.Background())
	if err == nil {
		if r.Status.Mode == string(types.TransportModeEdge) {
			hostname = token.ObjectMeta.Annotations["edge-host"]
		}
	} else {
		event.Recordf(c.name, "Unable to determine router mode: %v", err)
	}
	res := c.policy.ValidateOutgoingLink(hostname)
	return !res.Allowed()
}
