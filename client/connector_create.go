package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
	"regexp"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	certs "github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func generateConnectorName(namespace string, cli kubernetes.Interface) string {
	secrets, err := cli.CoreV1().Secrets(namespace).List(metav1.ListOptions{})
	max := 1
	if err == nil {
		connector_name_pattern := regexp.MustCompile("link([0-9]+)+")
		for _, s := range secrets.Items {
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

func secretFileAuthor(ctx context.Context, secretFile string) (author string, err error) {
	content, err := certs.GetSecretContent(secretFile)
	if err != nil {
		return "", err
	}
	generatedBy, ok := content["skupper.io/generated-by"]
	if !ok {
		return "", fmt.Errorf("Can't find secret origin.")
	}
	return string(generatedBy), nil
}

func (cli *VanClient) isOwnToken(ctx context.Context, secretFile string) (bool, error) {
	generatedBy, err := secretFileAuthor(ctx, secretFile)
	if err != nil {
		return false, err
	}
	siteConfig, err := cli.SiteConfigInspect(ctx, nil)
	if err != nil {
		return false, err
	}
	if siteConfig == nil {
		return false, fmt.Errorf("No site config")
	}
	return siteConfig.Reference.UID == string(generatedBy), nil
}

func (cli *VanClient) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	// Before doing any checks, make sure that Skupper is running.
	if _, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient); err != nil {
		return nil, err
	}

	// Disallow self-connection: make sure this token does not belong to this Skupper router.
	ownToken, err := cli.isOwnToken(ctx, secretFile)
	if err != nil {
		return nil, fmt.Errorf("Can't check secret ownership: '%s'", err.Error())
	}
	if ownToken {
		return nil, fmt.Errorf("Can't create connection to self with token '%s'", secretFile)
	}

	// Also disallow multiple use of same token.
	// Find its author, then compare against authors of already-existing
	// secrets that we have used to make connections.
	newConnectionAuthor, err := secretFileAuthor(ctx, secretFile)
	if err != nil {

		return nil, err
	}

	secrets, err := cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return nil, fmt.Errorf("Can't retrieve secrets.")
	}

	for _, oldSecret := range secrets.Items {
		oldConnectionAuthor, ok := oldSecret.Annotations["skupper.io/generated-by"]
		if !ok {
			return nil, fmt.Errorf("A secret has no author.")
		}
		if newConnectionAuthor == oldConnectionAuthor {
			return nil, fmt.Errorf("Already connected to \"%s\".", newConnectionAuthor)
		}
	}

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
		//deduce type from structire of secret
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
			if options.Name == "" {
				options.Name = generateConnectorName(options.SkupperNamespace, cli.KubeClient)
			}
			secret.ObjectMeta.Name = options.Name
			err = verify(&secret)
			if err != nil {
				return nil, err
			}
			// Verify if site link can be created
			err = cli.VerifySecretCompatibility(secret)
			if err != nil {
				return nil, err
			}
			if secret.ObjectMeta.Labels[types.SkupperTypeQualifier] == types.TypeClaimRequest {
				//can site handle claims?
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
			_, err = cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).Create(&secret)
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
		added := false
		//read annotations to get the host and port to connect to
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
			added = true
			updated = true
		}
		if updated {
			current.UpdateConfigMap(configmap)
			_, err = cli.KubeClient.CoreV1().ConfigMaps(options.SkupperNamespace).Update(configmap)
			if err != nil {
				return err
			}
			//need to mount the secret so router can access certs and key
			deployment, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
			if added {
				kube.AppendSecretVolume(&deployment.Spec.Template.Spec.Volumes, &deployment.Spec.Template.Spec.Containers[0].VolumeMounts, connector.Name, "/etc/qpid-dispatch-certs/"+profileName+"/")
			} else {
				touch(deployment)
			}
			_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(deployment)
			if err != nil {
				return err
			}
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
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(types.TransportConfigMapName, metav1.GetOptions{})
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
