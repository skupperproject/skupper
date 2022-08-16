package client

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"log"

	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/kube/qdr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) ServiceInterfaceRemove(ctx context.Context, address string) error {
	var unretryable error = nil
	err := retry.RetryOnConflict(defaultRetry, func() error {
		current, _, err := cli.ConfigMapManager(cli.Namespace).GetConfigMap(types.ServiceInterfaceConfigMap, &metav1.GetOptions{})
		if err == nil && current.Data != nil {
			jsonDef := current.Data[address]
			if jsonDef == "" {
				unretryable = fmt.Errorf("Service %s not defined", address)
				return nil
			} else {
				service := types.ServiceInterface{}
				err = jsonencoding.Unmarshal([]byte(jsonDef), &service)
				if service.IsAnnotated() && kube.IsOriginalServiceModified(service.Address, cli.Namespace, cli.GetKubeClient()) {
					_, err = kube.RemoveServiceAnnotations(service.Address, cli.Namespace, cli.KubeClient, []string{types.ProxyQualifier})
				} else {
					delete(current.Data, address)
					_, err = cli.ConfigMapManager(cli.Namespace).UpdateConfigMap(current)
				}
				if err != nil {
					// do not encapsulate this error, or it won't pass the errors.IsConflict test
					return err
				} else {
					handleServiceCertificateRemoval(address, cli)
					return nil
				}
			}
		} else if errors.IsNotFound(err) {
			unretryable = fmt.Errorf("No skupper services defined: %v", err.Error())
			return nil
		} else if current.Data == nil {
			unretryable = fmt.Errorf("Service %s not defined", address)
			return nil
		} else {
			unretryable = fmt.Errorf("Could not retrieve service definitions from configmap 'skupper-services': %s", err.Error())
			return nil
		}
	})
	if unretryable != nil {
		return unretryable
	}
	return err
}

func handleServiceCertificateRemoval(address string, cli *VanClient) {
	certName := types.SkupperServiceCertPrefix + address

	secret, _, err := cli.SecretManager(cli.Namespace).GetSecret(certName, &metav1.GetOptions{})

	if err == nil && secret != nil {

		err = qdr.RemoveSslProfile(secret.Name, cli.Namespace, cli.KubeClient)
		if err != nil {
			log.Printf("Failed to remove sslProfile from the router: %v", err.Error())
		}

		err = cli.SecretManager(cli.Namespace).DeleteSecret(secret, &metav1.DeleteOptions{})

		if err != nil {
			log.Printf("Failed to remove secret from the site: %v", err.Error())
		}
	}

}
