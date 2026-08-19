package qdr

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"

	"github.com/skupperproject/skupper/internal/qdr"
)

type Labelling interface {
	SetObjectMetadata(namespace string, name string, kind string, meta *metav1.ObjectMeta) bool
}

func UpdateRouterConfig(client kubernetes.Interface, name string, namespace string, ctxt context.Context, update qdr.ConfigUpdate, labelling Labelling, writer ConfigMapWriter) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return updateRouterConfig(client, name, namespace, ctxt, update, labelling, writer)
	})
}

func updateRouterConfig(client kubernetes.Interface, name string, namespace string, ctxt context.Context, update qdr.ConfigUpdate, labelling Labelling, writer ConfigMapWriter) error {
	current, err := client.CoreV1().ConfigMaps(namespace).Get(ctxt, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if current.ObjectMeta.Labels == nil {
		current.ObjectMeta.Labels = map[string]string{}
	}
	if current.ObjectMeta.Annotations == nil {
		current.ObjectMeta.Annotations = map[string]string{}
	}

	config, err := GetRouterConfigFromConfigMap(current)
	if err != nil {
		return err
	}
	updated := false

	if update.Apply(config) {
		updated = true
	}
	if labelling != nil {
		if labelling.SetObjectMetadata(namespace, name, "ConfigMap", &current.ObjectMeta) {
			updated = true
		}
	}
	if !updated {
		// A changed compression threshold can require a representation change
		// even when the router configuration itself is unchanged.
		usesDesiredRepresentation, err := writer.usesDesiredRepresentation(config, current)
		if err != nil {
			return err
		}
		if usesDesiredRepresentation {
			// no change required
			return nil
		}
	}

	err = writer.WriteConfigMap(config, current)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ConfigMaps(namespace).Update(ctxt, current, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}
