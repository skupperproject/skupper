package kube

import (
	"fmt"

	"github.com/skupperproject/skupper/api/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type LinkHandlerKube struct {
	namespace string
	cli       kubernetes.Interface
}

func NewLinkHandlerKube(namespace string, cli kubernetes.Interface) *LinkHandlerKube {
	return &LinkHandlerKube{
		namespace: namespace,
		cli:       cli,
	}
}

func (l *LinkHandlerKube) Create(secret *corev1.Secret, name string, cost int) error {
	// TODO implement me
	panic("implement me")
}

func (l *LinkHandlerKube) Delete(name string) error {
	// TODO implement me
	panic("implement me")
}

func (l *LinkHandlerKube) List() ([]*corev1.Secret, error) {
	currentSecrets, err := l.cli.CoreV1().Secrets(l.namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=connection-token"})
	if err != nil {
		return nil, fmt.Errorf("Could not retrieve secrets: %w", err)
	}
	var secrets []*corev1.Secret
	for _, s := range currentSecrets.Items {
		secrets = append(secrets, &s)
	}
	return secrets, nil
}

func (l *LinkHandlerKube) StatusAll() ([]types.LinkStatus, error) {
	// TODO implement me
	panic("implement me")
}

func (l *LinkHandlerKube) Status(name string) (types.LinkStatus, error) {
	// TODO implement me
	panic("implement me")
}
