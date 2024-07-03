package client

import (
	"context"
	"fmt"
	"os"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (cli *VanClient) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	yaml, err := os.ReadFile(secretFile)
	if err != nil {
		fmt.Println("Could not read connection token", err.Error())
		return nil, err
	}
	ys := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
		scheme.Scheme)
	options.Secret = &corev1.Secret{}
	_, _, err = ys.Decode(yaml, nil, options.Secret)
	if err != nil {
		return nil, fmt.Errorf("Could not parse connection token: %w", err)
	}
	secret, err := cli.ConnectorCreateSecretFromData(ctx, options)
	if err != nil {
		return nil, err
	}
	if options.Name == "" {
		options.Name = secret.ObjectMeta.Name
	}

	err = cli.ConnectorCreate(ctx, secret, options)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (cli *VanClient) ConnectorCreateSecretFromData(ctx context.Context, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	// TODO REMOVE CLIENT PKG
	return nil, fmt.Errorf("broken implementation")
}

// VerifySecretCompatibility returns nil if current site version is compatible
// with the token or cert provided. If sites are not compatible an error is
// returned with the appropriate information
func (cli *VanClient) VerifySecretCompatibility(secret corev1.Secret) error {
	// TODO Removed broken v1 implementation
	return fmt.Errorf("broken implementation")
}

// VerifySiteCompatibility returns nil if current site version is compatible
// with the provided version, otherwise it returns a clear error.
func (cli *VanClient) VerifySiteCompatibility(siteVersion string) error {
	// TODO Removed broken v1 implementation
	return fmt.Errorf("broken implementation")
}

func isCertToken(secret *corev1.Secret) bool {
	return secret.ObjectMeta.Labels != nil && secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeToken
}

const SSL_PROFILE_PATH = "/etc/skupper-router-certs"

func (cli *VanClient) ConnectorCreate(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions) error {

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		siteConfig, err := cli.SiteConfigInspectInNamespace(ctx, nil, options.SkupperNamespace)
		if err != nil {
			return err
		}
		configmap, err := kube.GetConfigMap(types.TransportConfigMapName, options.SkupperNamespace, cli.KubeClient)
		if err != nil {
			return err
		}
		current, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}
		updated := false
		// read annotations to get the host and port to connect to
		profileName := options.Name + "-profile"
		if _, ok := current.SslProfiles[profileName]; !ok {
			_, hasClientCert := secret.Data["tls.crt"]
			current.AddSslProfile(qdr.ConfigureSslProfile(profileName, SSL_PROFILE_PATH, hasClientCert))
			updated = true
		}
		connector := qdr.Connector{
			Name:       options.Name,
			Cost:       options.Cost,
			SslProfile: profileName,
		}
		connector.SetMaxFrameSize(siteConfig.Spec.Router.MaxFrameSize)
		connector.SetMaxSessionFrames(siteConfig.Spec.Router.MaxSessionFrames)
		if current.IsEdge() {
			connector.Host = secret.ObjectMeta.Annotations["edge-host"]
			connector.Port = secret.ObjectMeta.Annotations["edge-port"]
			connector.Role = qdr.RoleEdge
		} else {
			connector.Host = secret.ObjectMeta.Annotations["inter-router-host"]
			connector.Port = secret.ObjectMeta.Annotations["inter-router-port"]
			connector.Role = qdr.RoleInterRouter
		}
		if existing, ok := current.Connectors[connector.Name]; ok {
			if !reflect.DeepEqual(existing, connector) {
				current.Connectors[connector.Name] = connector
				updated = true
			}
		} else {
			current.AddConnector(connector)
			updated = true
		}
		if updated {
			current.UpdateConfigMap(configmap)
			_, err = cli.KubeClient.CoreV1().ConfigMaps(options.SkupperNamespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("Failed to update skupper-router deployment: %w", err)
	}
	return nil
}

func (cli *VanClient) requireSiteVersion(ctx context.Context, namespace string, minimumVersion string) error {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	config, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}
	site := config.GetSiteMetadata()
	if !utils.IsValidFor(site.Version, minimumVersion) {
		return fmt.Errorf("Site version is %s, require %s", site.Version, minimumVersion)
	}
	return nil
}
