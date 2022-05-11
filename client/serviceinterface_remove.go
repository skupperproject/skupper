package client

import (
	"context"
	"fmt"
	"log"

	"github.com/skupperproject/skupper/pkg/qdr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
)

func (cli *VanClient) ServiceInterfaceRemove(ctx context.Context, address string) error {
	var unretryable error = nil
	err := retry.RetryOnConflict(defaultRetry, func() error {
		current, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(types.ServiceInterfaceConfigMap, metav1.GetOptions{})
		if err == nil && current.Data != nil {
			jsonDef := current.Data[address]
			if jsonDef == "" {
				unretryable = fmt.Errorf("Service %s not defined", address)
				return nil
			} else {
				delete(current.Data, address)
				_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(current)
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

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(certName, metav1.GetOptions{})

	if err == nil && secret != nil {

		err = qdr.RemoveSslProfile(secret.Name, cli.Namespace, cli.KubeClient)
		if err != nil {
			log.Printf("Failed to remove sslProfile from the router: %v", err.Error())
		}

		err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Delete(secret.Name, &metav1.DeleteOptions{})

		if err != nil {
			log.Printf("Failed to remove secret from the site: %v", err.Error())
		}
	}

}
