package client

import (
	"context"
	"fmt"
	"strconv"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube/resolver"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (cli *VanClient) checkNotEdgeAndGetVersion(ctx context.Context, namespace string) (string, error) {
	current, err := cli.getRouterConfig(ctx, namespace)
	if err != nil {
		return "", err
	}
	if current.IsEdge() {
		return "", fmt.Errorf("Edge configuration cannot accept connections")
	}
	return current.GetSiteMetadata().Version, nil
}

func (cli *VanClient) ConnectorTokenCreateFromTemplate(ctx context.Context, tokenName string, templateName string) (*corev1.Secret, bool, error) {
	version, err := cli.checkNotEdgeAndGetVersion(ctx, cli.Namespace)
	if err != nil {
		return nil, false, err
	}
	template, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(ctx, templateName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: tokenName,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeToken,
			},
			Annotations: map[string]string{
				types.TokenTemplate: templateName,
			},
		},
		Data: template.Data,
	}
	if _, hasCert := template.Data["tls.crt"]; hasCert {
		if _, hasKey := template.Data["tls.key"]; hasKey {
			secret.Type = "kubernetes.io/tls"
		}
	}
	localOnly, err := cli.annotateConnectorToken(ctx, cli.Namespace, secret, version)
	if err != nil {
		return nil, false, err
	}
	return secret, localOnly, nil
}

func (cli *VanClient) ConnectorTokenCreate(ctx context.Context, subject string, namespace string) (*corev1.Secret, bool, error) {
	if namespace == "" {
		namespace = cli.Namespace
	}
	version, err := cli.checkNotEdgeAndGetVersion(ctx, namespace)
	if err != nil {
		return nil, false, err
	}
	caSecret, err := cli.KubeClient.CoreV1().Secrets(namespace).Get(context.TODO(), types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}
	secret := certs.GenerateSecret(subject, subject, "", caSecret)
	localOnly, err := cli.annotateConnectorToken(ctx, namespace, &secret, version)
	if err != nil {
		return nil, false, err
	}
	return &secret, localOnly, nil
}

func (cli *VanClient) annotateConnectorToken(ctx context.Context, namespace string, token *corev1.Secret, version string) (bool, error) {
	// get the host and port for inter-router and edge
	siteConfigMap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, types.SiteConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	siteConfig, err := cli.SiteConfigInspect(ctx, siteConfigMap)
	if err != nil {
		return false, err
	}
	if siteConfig == nil {
		return false, fmt.Errorf("No site config found in %s", namespace)
	}
	rslvr, err := resolver.NewResolver(cli, namespace, &siteConfig.Spec)
	if err != nil {
		return false, err
	}
	interRouterHostPort, err := rslvr.GetHostPortForInterRouter()
	if err != nil {
		return false, err
	}
	edgeHostPort, err := rslvr.GetHostPortForEdge()
	if err != nil {
		return false, err
	}

	certs.AnnotateConnectionToken(token, "inter-router", interRouterHostPort.Host, strconv.Itoa(int(interRouterHostPort.Port)))
	certs.AnnotateConnectionToken(token, "edge", edgeHostPort.Host, strconv.Itoa(int(edgeHostPort.Port)))
	token.Annotations[types.SiteVersion] = version
	if token.ObjectMeta.Labels == nil {
		token.ObjectMeta.Labels = map[string]string{}
	}
	token.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
	// Store our siteID in the token, to prevent later self-connection.
	if siteConfig != nil {
		token.ObjectMeta.Annotations[types.TokenGeneratedBy] = siteConfig.Reference.UID
	}
	return rslvr.IsLocalAccessOnly(), nil
}

func (cli *VanClient) ConnectorTokenCreateFile(ctx context.Context, subject string, secretFile string) error {
	policy := NewPolicyValidatorAPI(cli)
	res, err := policy.IncomingLink()
	if err != nil {
		return err
	}
	if !res.Allowed {
		return res.Err()
	}
	secret, localOnly, err := cli.ConnectorTokenCreate(ctx, subject, "")
	if err == nil {
		return certs.GenerateSecretFile(secretFile, secret, localOnly)
	} else {
		return err
	}
}
