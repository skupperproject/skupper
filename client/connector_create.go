package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
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

func (cli *VanClient) ConnectorCreateFromFile(ctx context.Context, secretFile string, options types.ConnectorCreateOptions) (*corev1.Secret, error) {
	secret, err := cli.ConnectorCreateSecretFromFile(ctx, secretFile, options)
	if err == nil {
		current, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
		if err == nil {
			if options.Name == "" {
				options.Name = secret.ObjectMeta.Name
			}
			return secret, cli.createConnector(ctx, secret, options, current)
		} else {
			return nil, fmt.Errorf("Failed to retrieve transport deployment: %w", err)
		}
	} else {
		return nil, err
	}
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
				return &secret, fmt.Errorf("A connector secret of that name already exist, please choose a different name")
			} else {
				return nil, fmt.Errorf("Failed to create connector secret: %w", err)
			}
		}
	} else {
		return nil, fmt.Errorf("Failed to retrieve router deployment: %w", err)
	}
}

func (cli *VanClient) ConnectorCreate(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions) error {
	current, err := kube.GetDeployment(types.TransportDeploymentName, options.SkupperNamespace, cli.KubeClient)
	if err == nil {
		return cli.createConnector(ctx, secret, options, current)
	} else {
		return fmt.Errorf("Failed to retrieve router deployment: %w", err)
	}
}

func (cli *VanClient) createConnector(ctx context.Context, secret *corev1.Secret, options types.ConnectorCreateOptions, current *appsv1.Deployment) error {
	mode := qdr.GetTransportMode(current)
	//read annotations to get the host and port to connect to
	connector := types.Connector{
		Name: options.Name,
		Cost: options.Cost,
	}
	if mode == types.TransportModeInterior {
		connector.Host = secret.ObjectMeta.Annotations["inter-router-host"]
		connector.Port = secret.ObjectMeta.Annotations["inter-router-port"]
		connector.Role = string(types.ConnectorRoleInterRouter)
	} else {
		connector.Host = secret.ObjectMeta.Annotations["edge-host"]
		connector.Port = secret.ObjectMeta.Annotations["edge-port"]
		connector.Role = string(types.ConnectorRoleEdge)
	}
	qdr.AddConnector(&connector, current)

	_, err := cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(current)
	for i := 0; i < 10 && errors.IsConflict(err); i++ {
		time.Sleep(500 * time.Millisecond)
		_, err = cli.KubeClient.AppsV1().Deployments(options.SkupperNamespace).Update(current)
	}
	if err != nil {
		return fmt.Errorf("Failed to update qdr deployment: %w", err)
	} else {
		return nil
	}
}
