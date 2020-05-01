package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"

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

// TODO put these in kube or utils
func isInterior(qdr *appsv1.Deployment) bool {
	config := kube.FindEnvVar(qdr.Spec.Template.Spec.Containers[0].Env, "QDROUTERD_CONF")
	//match 'mode: interior' in that config
	if config == nil {
		log.Fatal("Could not retrieve qdr config")
	}
	match, _ := regexp.MatchString("mode:[ ]+interior", config.Value)
	return match
}

func getTransportMode(dep *appsv1.Deployment) types.TransportMode {
	if qdr.IsInterior(dep) {
		return types.TransportModeInterior
	} else {
		return types.TransportModeEdge
	}
}

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

func (cli *VanClient) VanConnectorCreate(ctx context.Context, secretFile string, options types.VanConnectorCreateOptions) error {
	yaml, err := ioutil.ReadFile(secretFile)
	if err != nil {
		fmt.Println("Could not read connection token", err.Error())
		return err
	}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme,
		scheme.Scheme)
	var secret corev1.Secret
	_, _, err = s.Decode(yaml, nil, &secret)
	if err != nil {
		fmt.Printf("Could not parse connection token: %s", err)
		fmt.Println()
		return err
	}
	current, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
	if err == nil {
		mode := qdr.GetTransportMode(current)
		if options.Name == "" {
			options.Name = generateConnectorName(cli.Namespace, cli.KubeClient)
		}
		secret.ObjectMeta.Name = options.Name
		secret.ObjectMeta.Labels = map[string]string{
			"skupper.io/type": "connection-token",
		}
		secret.ObjectMeta.SetOwnerReferences([]metav1.OwnerReference{
			kube.GetOwnerReference(current),
		})
		_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&secret)
		if err == nil {
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
			_, err = cli.KubeClient.AppsV1().Deployments(cli.Namespace).Update(current)
			if err != nil {
				fmt.Println("Failed to update qdr deployment: ", err.Error())
			} else {
				fmt.Printf("Skupper configured to connect to %s:%s (name=%s)", connector.Host, connector.Port, connector.Name)
				fmt.Println()
			}
		} else if errors.IsAlreadyExists(err) {
			fmt.Println("A connector secret of that name already exist, please choose a different name")
			return err
		} else {
			fmt.Println("Failed to create connector secret: ", err.Error())
			return err
		}
	} else {
		fmt.Println("Failed to retrieve qdr deployment: ", err.Error())
		return err
	}
	return nil
}
