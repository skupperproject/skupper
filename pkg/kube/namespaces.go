package kube

import (
	"os"
)

const NamespaceFile string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

func CurrentNamespace() (string, error) {
	namespace := os.Getenv("NAMESPACE")
	if namespace != "" {
		return namespace, nil
	}
	_, err := os.Stat(NamespaceFile)
	if err == nil {
		raw, err := os.ReadFile(NamespaceFile)
		if err != nil {
			return namespace, err
		}
		namespace = string(raw)
		return namespace, nil
	} else {
		return namespace, err
	}

}
