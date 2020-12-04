package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
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
	"os"
)

func generateConnectorName(namespace string, cli kubernetes.Interface) string {
	secrets, err := cli.CoreV1().Secrets(namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	max := 1
	if err == nil {
		connector_name_pattern := regexp.MustCompile("conn([0-9])+")
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
		log.Fatal("Could not retrieve connection-token secrets:", err)
	}
	return "conn" + strconv.Itoa(max)
}

func secretFileVersion(ctx context.Context, secretFile string) (float64, error) {
	content, err := certs.GetSecretContent(secretFile)
	if err != nil {
		return 0.0, err
	}
	version_str, ok := content["skupper.io/version"]
	if !ok {
		return 0.0, fmt.Errorf("Can't find secret version.")
	}

	return strconv.ParseFloat(string(version_str), 64)
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

func (cli *VanClient) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, float64, error) {
	// Before doing any checks, make sure that Skupper is running.
	if _, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient); err != nil {
		return nil, 0.0, err
	}

	version, err := secretFileVersion(ctx, secretFile)
	if err != nil {
		return nil, 0.0, err
	}

	// Disallow self-connection: make sure this token does not belong to this Skupper router.
	ownToken, err := cli.isOwnToken(ctx, secretFile)
	if err != nil {
		return nil, 0.0, fmt.Errorf("Can't check secret ownership: '%s'", err.Error())
	}
	if ownToken {
		return nil, 0.0, fmt.Errorf("Can't create connection to self with token '%s'", secretFile)
	}

	// Also disallow multiple use of same token.
	// Find its author, then compare against authors of already-existing
	// secrets that we have used to make connections.
	newConnectionAuthor, err := secretFileAuthor(ctx, secretFile)
	if err != nil {
		return nil, 0.0, err
	}

	secrets, err := cli.KubeClient.CoreV1().Secrets(options.SkupperNamespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return nil, 0.0, fmt.Errorf("Can't retrieve secrets.")
	}

	for _, oldSecret := range secrets.Items {
		oldConnectionAuthor, ok := oldSecret.Annotations["skupper.io/generated-by"]
		if !ok {
			return nil, 0.0, fmt.Errorf("A secret has no author.")
		}
		if newConnectionAuthor == oldConnectionAuthor {
			return nil, 0.0, fmt.Errorf("Already connected to \"%s\".", newConnectionAuthor)
		}
	}

	secret, err := cli.ConnectorCreateSecretFromFile(ctx, secretFile, options)
	if err != nil {
		return nil, 0.0, err
	}
	if options.Name == "" {
		options.Name = secret.ObjectMeta.Name
	}

	err = cli.ConnectorCreate(ctx, secret, options)
	if err != nil {
		return nil, 0.0, err
	}
	fmt.Fprintf(os.Stdout, "ConnectorCreateFromFile returning version %.2f\n", version)
	return secret, version, nil
}

func (cli *VanClient) ConnectorCreateSecretFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	yaml, err := ioutil.ReadFile(secretFile)
	if err != nil {
		fmt.Println("Could not read connection token", err.Error())
		return nil, err
	}
	current, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
	if err == nil {
		s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
			scheme.Scheme)
		var secret corev1.Secret
		_, _, err = s.Decode(yaml, nil, &secret)
		if err != nil {
			return nil, fmt.Errorf("Could not parse connection token: %w", err)
		} else {
			if options.Name == "" {
				options.Name = generateConnectorName(options.SkupperNamespace, cli.KubeClient)
			}
			secret.ObjectMeta.Name = options.Name
			secret.ObjectMeta.Labels = map[string]string{
				"skupper.io/type": "connection-token",
			}
			secret.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{
				kube.GetDeploymentOwnerReference(current),
			})
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

func (cli *VanClient) ConnectorCreate(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions) error {

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configmap, err := kube.GetConfigMap("skupper-internal" /*TODO: change to constant*/, options.SkupperNamespace, cli.KubeClient)
		if err != nil {
			return err
		}
		current, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}
		//read annotations to get the host and port to connect to
		profileName := options.Name + "-profile"
		current.AddSslProfile(qdr.SslProfile{
			Name: profileName,
		})
		connector := qdr.Connector{
			Name:       options.Name,
			Cost:       options.Cost,
			SslProfile: profileName,
		}
		if current.IsEdge() {
			connector.Host = secret.ObjectMeta.Annotations["edge-host"]
			connector.Port = secret.ObjectMeta.Annotations["edge-port"]
			connector.Role = qdr.RoleEdge
		} else {
			connector.Host = secret.ObjectMeta.Annotations["inter-router-host"]
			connector.Port = secret.ObjectMeta.Annotations["inter-router-port"]
			connector.Role = qdr.RoleInterRouter
		}
		current.AddConnector(connector)
		current.UpdateConfigMap(configmap)
		_, err = cli.KubeClient.CoreV1().ConfigMaps(options.SkupperNamespace).Update(configmap)
		if err != nil {
			return err
		}
		//need to mount the secret so router can access certs and key
		deployment, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
		kube.AppendSecretVolume(&deployment.Spec.Template.Spec.Volumes, &deployment.Spec.Template.Spec.Containers[0].VolumeMounts, connector.Name, "/etc/qpid-dispatch-certs/"+profileName+"/")
		_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(deployment)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to update skupper-router deployment: %w", err)
	}
	return nil
}
