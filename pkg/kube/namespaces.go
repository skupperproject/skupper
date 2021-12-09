package kube

import (
	"fmt"
	"io/ioutil"
	"os"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewNamespace(name string, cli kubernetes.Interface) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	return cli.CoreV1().Namespaces().Create(ns)
}

func DeleteNamespace(name string, cli kubernetes.Interface) error {
	err := cli.CoreV1().Namespaces().Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		return err
	} else if errors.IsNotFound(err) {
		return fmt.Errorf("Namepace %s does not exist.", name)
	} else {
		return fmt.Errorf("Failed to delete namesspace: %w", err)
	}
}

const NamespaceFile string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func CurrentNamespace() (string, error) {
	namespace := os.Getenv("NAMESPACE")
	if namespace != "" {
		return namespace, nil
	}
	_, err := os.Stat(NamespaceFile)
	if err == nil {
		raw, err := ioutil.ReadFile(NamespaceFile)
		if err != nil {
			return namespace, err
		}
		namespace = string(raw)
		return namespace, nil
	} else {
		return namespace, err
	}

}
