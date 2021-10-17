package client

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

func (cli *VanClient) CreateServiceCA(ownerRef *metav1.OwnerReference) error {

	ca := types.CertAuthority{Name: types.ServiceCaSecret}

	caSecret, err := kube.NewCertAuthority(ca, ownerRef, cli.Namespace, cli.KubeClient)

	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	_, err = kube.NewSimpleSecret(types.ServiceClientSecret, caSecret, ownerRef, cli.Namespace, cli.KubeClient)

	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	err = cli.AppendSecretToRouter(types.ServiceClientSecret)
	if err != nil {
		return err
	}

	return nil
}

func (cli *VanClient) CreateSecretForService(serviceName string, hosts string, secretName string) (*corev1.Secret, error) {
	caCert, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(types.ServiceCaSecret, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	serviceSecret := certs.GenerateSecret(secretName, serviceName, hosts, caCert)
	createdSecret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(&serviceSecret)

	if err != nil {
		return nil, err
	}

	return createdSecret, nil
}

func (cli *VanClient) DeleteSecretForService(secretName string) error {
	_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(secretName, metav1.GetOptions{})

	if err != nil {
		return err
	}

	err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Delete(secretName, &metav1.DeleteOptions{})

	if err != nil {
		return err
	}

	return nil
}

func (cli *VanClient) AppendSecretToRouter(secretname string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.Namespace, cli.KubeClient)
		if err != nil {
			return err
		}
		current, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}

		if _, ok := current.SslProfiles[secretname]; !ok {
			current.AddSslProfile(qdr.SslProfile{
				Name: secretname,
			})
		}

		_, err = current.UpdateConfigMap(configmap)
		if err != nil {
			return err
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(configmap)
		if err != nil {
			return err
		}
		//need to mount the secret so router can access certs and key
		deployment, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)

		kube.AppendSecretVolume(&deployment.Spec.Template.Spec.Volumes, &deployment.Spec.Template.Spec.Containers[0].VolumeMounts, secretname, "/etc/qpid-dispatch-certs/"+secretname+"/")

		_, err = cli.KubeClient.AppsV1().Deployments(cli.Namespace).Update(deployment)
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

func (cli *VanClient) RemoveSecretFromRouter(secretname string) error {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.Namespace, cli.KubeClient)
		if err != nil {
			return err
		}
		current, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}

		if _, ok := current.SslProfiles[secretname]; ok {
			current.RemoveSslProfile(secretname)
		}

		_, err = current.UpdateConfigMap(configmap)
		if err != nil {
			return err
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(configmap)
		if err != nil {
			return err
		}
		//need to mount the secret so router can access certs and key
		deployment, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)

		kube.RemoveSecretVolumeForDeployment(secretname, deployment, 0)

		_, err = cli.KubeClient.AppsV1().Deployments(cli.Namespace).Update(deployment)
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
