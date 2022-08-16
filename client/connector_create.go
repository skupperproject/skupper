package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"reflect"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func generateConnectorName(cli types.Secrets) string {
	secrets, err := cli.ListSecrets(&metav1.ListOptions{})
	max := 1
	if err == nil {
		connector_name_pattern := regexp.MustCompile("link([0-9]+)+")
		for _, s := range secrets {
			count := connector_name_pattern.FindStringSubmatch(s.ObjectMeta.Name)
			if len(count) > 1 {
				v, _ := strconv.Atoi(count[1])
				if v >= max {
					max = v + 1
				}
			}

		}
	} else {
		log.Fatal("Could not retrieve token secrets:", err)
	}
	return "link" + strconv.Itoa(max)
}

func (cli *VanClient) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	yaml, err := ioutil.ReadFile(secretFile)
	if err != nil {
		fmt.Println("Could not read connection token", err.Error())
		return nil, err
	}
	secret, err := cli.ConnectorCreateSecretFromData(ctx, yaml, options)
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

func verify(secret *corev1.Secret) error {
	if secret.ObjectMeta.Labels == nil {
		secret.ObjectMeta.Labels = map[string]string{}
	}
	if _, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]; !ok {
		// deduce type from structire of secret
		if _, ok = secret.Data["tls.crt"]; ok {
			secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeToken
		} else if secret.ObjectMeta.Annotations != nil && secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey] != "" {
			secret.ObjectMeta.Labels[types.SkupperTypeQualifier] = types.TypeClaimRequest
		}
	}
	switch secret.ObjectMeta.Labels[types.SkupperTypeQualifier] {
	case types.TypeToken:
		CertTokenDataFields := []string{"tls.key", "tls.crt", "ca.crt"}
		for _, name := range CertTokenDataFields {
			if _, ok := secret.Data[name]; !ok {
				return fmt.Errorf("Expected %s field in secret data", name)
			}
		}
	case types.TypeClaimRequest:
		if _, ok := secret.Data["password"]; !ok {
			return fmt.Errorf("Expected password field in secret data")
		}
		if secret.ObjectMeta.Annotations == nil || secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey] == "" {
			return fmt.Errorf("Expected %s annotation", types.ClaimUrlAnnotationKey)
		}
	default:
		return fmt.Errorf("Secret is not a valid skupper token")
	}
	return nil
}

func (cli *VanClient) ConnectorCreateSecretFromData(ctx context.Context, secretData []byte, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	current, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
	if err == nil {
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
			scheme.Scheme)
		var secret corev1.Secret
		_, _, err = s.Decode(secretData, nil, &secret)
		if err != nil {
			return nil, fmt.Errorf("Could not parse connection token: %w", err)
		} else {
			// Validating destination host
			siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
			if err != nil {
				return nil, err
			}
			hostname := ""
			if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeToken {
				if siteConfig.Spec.RouterMode == string(types.TransportModeEdge) {
					hostname = secret.ObjectMeta.Annotations["edge-host"]
				} else {
					hostname = secret.ObjectMeta.Annotations["inter-router-host"]
				}
			} else {
				destUrl, err := url.Parse(secret.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey])
				if err != nil {
					return nil, fmt.Errorf("Invalid URL defined in token: %s", err)
				}
				hostname = destUrl.Hostname()
			}
			policy := NewClusterPolicyValidator(cli)
			res := policy.ValidateOutgoingLink(hostname)
			if !res.Allowed() {
				return nil, fmt.Errorf("outgoing link to %s is not allowed", hostname)
			}

			if options.Name == "" {
				options.Name = generateConnectorName(cli.SecretManager(options.SkupperNamespace))
			}
			secret.ObjectMeta.Name = options.Name
			err = verify(&secret)
			if err != nil {
				return nil, err
			}
			// Verify if site link can be created
			err = cli.verifyNotSelfOrDuplicate(secret, siteConfig.Reference.UID, options)
			if err != nil {
				return nil, err
			}
			err = cli.VerifySecretCompatibility(secret)
			if err != nil {
				return nil, err
			}
			if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeClaimRequest {
				// can site handle claims?
				err := cli.requireSiteVersion(ctx, options.SkupperNamespace, "0.7.0")
				if err != nil {
					return nil, fmt.Errorf("Claims not supported. %s", err)
				}
			}
			secret.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{
				kube.GetDeploymentOwnerReference(current),
			})
			if options.Cost != 0 {
				if secret.ObjectMeta.Annotations == nil {
					secret.ObjectMeta.Annotations = map[string]string{}
				}
				secret.ObjectMeta.Annotations[types.TokenCost] = strconv.Itoa(int(options.Cost))
			}
			_, err = cli.SecretManager(options.SkupperNamespace).CreateSecret(&secret)
			if err == nil {
				return &secret, nil
			} else if errors.IsAlreadyExists(err) {
				return &secret, fmt.Errorf("The connector secret \"%s\"already exists, please choose a different name", secret.ObjectMeta.Name)
			} else {
				return nil, fmt.Errorf("Failed to create connector secret: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("Failed to retrieve router deployment: %w", err)
	}
}

// VerifySecretCompatibility returns nil if current site version is compatible
// with the token or cert provided. If sites are not compatible an error is
// returned with the appropriate information
func (cli *VanClient) VerifySecretCompatibility(secret corev1.Secret) error {
	var secretVersion string
	if secret.Annotations != nil {
		secretVersion = secret.Annotations[types.SiteVersion]
	}
	if err := cli.VerifySiteCompatibility(secretVersion); err != nil {
		if secretVersion == "" {
			secretVersion = "undefined"
		}
		return fmt.Errorf("%v - remote site version is %s", err, secretVersion)
	}
	return nil
}

// VerifySiteCompatibility returns nil if current site version is compatible
// with the provided version, otherwise it returns a clear error.
func (cli *VanClient) VerifySiteCompatibility(siteVersion string) error {
	siteMeta, err := cli.GetSiteMetadata()
	if err != nil {
		return err
	}
	if utils.LessRecentThanVersion(siteVersion, siteMeta.Version) {
		if !utils.IsValidFor(siteVersion, cli.GetMinimumCompatibleVersion()) {
			return fmt.Errorf("minimum version required %s", cli.GetMinimumCompatibleVersion())
		}
	}
	return nil
}

func (cli *VanClient) verifyNotSelfOrDuplicate(secret corev1.Secret, self string, options types.ConnectorCreateOptions) error {
	if secret.ObjectMeta.Annotations == nil {
		return fmt.Errorf("The secret has not annotations")
	}
	generatedBy, ok := secret.ObjectMeta.Annotations[types.TokenGeneratedBy]
	if !ok {
		return fmt.Errorf("Can't find secret origin.")
	}
	if self == string(generatedBy) {
		return fmt.Errorf("Can't create connection to self with token")
	}
	currentSecrets, err := cli.SecretManager(options.SkupperNamespace).ListSecrets(&metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return fmt.Errorf("Could not retrieve secrets: %w", err)
	}
	for _, currentSecret := range currentSecrets {
		currentAuthor, ok := currentSecret.Annotations[types.TokenGeneratedBy]
		if !ok {
			return fmt.Errorf("A secret has no author.")
		}
		if generatedBy == currentAuthor {
			return fmt.Errorf("Already connected to \"%s\".", currentAuthor)
		}
	}
	return nil
}

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
			current.AddSslProfile(qdr.SslProfile{
				Name: profileName,
			})
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
			_, err = cli.ConfigMapManager(options.SkupperNamespace).UpdateConfigMap(configmap)
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
	configmap, _, err := cli.ConfigMapManager(namespace).GetConfigMap(types.TransportConfigMapName, &metav1.GetOptions{})
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
