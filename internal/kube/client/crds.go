package client

import (
	"context"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func IsCrdAvailable(client clientset.Interface, name string) bool {
	crd, err := getCrd(client, name)
	return err == nil && crd != nil
}

func IsCrdPathAvailable(client clientset.Interface, name string, path string) bool {
	crd, err := getCrd(client, name)
	if err != nil {
		return false
	}
	paths := strings.Split(path, ".")
	for _, crdVersion := range crd.Spec.Versions {
		if crdVersion.Schema == nil || crdVersion.Schema.OpenAPIV3Schema == nil {
			continue
		}
		schema := crdVersion.Schema.OpenAPIV3Schema
		properties := schema.Properties
		found := true
		for _, item := range paths {
			if prop, ok := properties[item]; !ok {
				found = false
				break
			} else {
				properties = prop.Properties
			}
		}
		if found {
			return true
		}
	}
	return false
}

func getCrd(client clientset.Interface, name string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(
		context.Background(), name, v1.GetOptions{})
	return crd, err
}
