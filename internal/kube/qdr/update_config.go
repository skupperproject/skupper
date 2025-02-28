package qdr

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/internal/qdr"
)

type Labelling interface {
	SetLabels(namespace string, name string, kind string, labels map[string]string) bool
	SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool
}

func UpdateRouterConfig(client kubernetes.Interface, name string, namespace string, ctxt context.Context, update qdr.ConfigUpdate, labelling Labelling) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return updateRouterConfig(client, name, namespace, ctxt, update, labelling)
	})
}

func updateRouterConfig(client kubernetes.Interface, name string, namespace string, ctxt context.Context, update qdr.ConfigUpdate, labelling Labelling) error {
	current, err := client.CoreV1().ConfigMaps(namespace).Get(ctxt, name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	config, err := qdr.GetRouterConfigFromConfigMap(current)
	if err != nil {
		return err
	}
	updated := false

	if update.Apply(config) {
		updated = true
	}
	if labelling != nil {
		if labelling.SetLabels(namespace, name, "ConfigMap", current.ObjectMeta.Labels) {
			updated = true
		}
		if labelling.SetAnnotations(namespace, name, "ConfigMap", current.ObjectMeta.Annotations) {
			updated = true
		}
	}
	if !updated {
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
