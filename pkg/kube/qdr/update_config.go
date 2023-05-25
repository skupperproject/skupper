package qdr

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func UpdateRouterConfig(client kubernetes.Interface, namespace string, ctxt context.Context, update qdr.ConfigUpdate) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return updateRouterConfig(client, namespace, ctxt, update)
	})
}

func updateRouterConfig(client kubernetes.Interface, namespace string, ctxt context.Context, update qdr.ConfigUpdate) error {
	current, err := client.CoreV1().ConfigMaps(namespace).Get(ctxt, types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	config, err := qdr.GetRouterConfigFromConfigMap(current)
	if err != nil {
		return err
	}

	if !update.Apply(config) {
		// no change required
		return nil
	}

	err = config.WriteToConfigMap(current)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctxt, current, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
