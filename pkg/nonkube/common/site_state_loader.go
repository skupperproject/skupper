package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

type FileSystemSiteStateLoader struct {
	Path      string
	Namespace string
	Bundle    bool
}

func (f *FileSystemSiteStateLoader) Load() (*api.SiteState, error) {
	var siteState = api.NewSiteState(f.Bundle)
	filter := func(filename string) bool {
		return strings.HasSuffix(filename, ".yaml") || strings.HasSuffix(filename, ".yml")
	}
	dirReader := new(utils.DirectoryReader)
	yamlFileNames, err := dirReader.ReadDir(f.Path, filter)
	if err != nil {
		return nil, err
	}
	// Reading all yaml files found
	for _, yamlFileName := range yamlFileNames {
		yamlFile, err := os.Open(yamlFileName)
		if err != nil {
			return nil, err
		}
		reader := bufio.NewReader(yamlFile)
		err = LoadIntoSiteState(reader, siteState)
		if err != nil {
			return siteState, fmt.Errorf("error loading %q: %v", yamlFileName, err)
		}
		siteState.SetNamespace(f.Namespace)
	}
	return siteState, nil
}

func LoadIntoSiteState(reader *bufio.Reader, siteState *api.SiteState) error {
	var err error
	var namespacesFound = make(map[string]bool)
	yamlDecoder := yamlutil.NewYAMLOrJSONDecoder(reader, 1024)
	// allow reading multiple-document yaml
	for {
		var rawObj runtime.RawExtension
		err = yamlDecoder.Decode(&rawObj)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error decoding yaml: %s", err)
			}
			break
		}
		// Decoded object from rawObject, with gvk (Group Version Kind)
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return err
		}
		// Saving all namespaces found to ensure a unique namespace is used across all loaded resources
		var objectMeta metav1.ObjectMeta
		runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &objectMeta)
		if objectMeta.Namespace == "default" {
			objectMeta.Namespace = ""
		}
		namespacesFound[objectMeta.Namespace] = true

		// We only care about our v1alpha1 types
		if v1alpha1.SchemeGroupVersion == gvk.GroupVersion() {
			switch gvk.Kind {
			case "Site":
				if siteState.Site.Name != "" {
					return fmt.Errorf("multiple sites found, but only one site is allowed for bootstrapping")
				}
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), siteState.Site)
			case "Listener":
				var listener v1alpha1.Listener
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &listener)
				siteState.Listeners[listener.Name] = &listener
			case "Connector":
				var connector v1alpha1.Connector
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &connector)
				siteState.Connectors[connector.Name] = &connector
			case "RouterAccess":
				var routerAccess v1alpha1.RouterAccess
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &routerAccess)
				siteState.RouterAccesses[routerAccess.Name] = &routerAccess
			case "Grant":
				var grant v1alpha1.AccessGrant
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &grant)
				siteState.Grants[grant.Name] = &grant
			case "Link":
				var link v1alpha1.Link
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &link)
				siteState.Links[link.Name] = &link
			case "AccessToken":
				var claim v1alpha1.AccessToken
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &claim)
				siteState.Claims[claim.Name] = &claim
			case "Certificate":
				var certificate v1alpha1.Certificate
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &certificate)
				siteState.Certificates[certificate.Name] = &certificate
			case "SecuredAccess":
				var securedAccess v1alpha1.SecuredAccess
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &securedAccess)
				siteState.SecuredAccesses[securedAccess.Name] = &securedAccess
			}
		} else if corev1.SchemeGroupVersion == gvk.GroupVersion() {
			switch gvk.Kind {
			case "Secret":
				var secret corev1.Secret
				runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &secret)
				siteState.Secrets[secret.Name] = &secret
			}
		}
	}
	if len(namespacesFound) > 1 {
		return fmt.Errorf("multiple namespaces found, but only a unique namespace must be used across all "+
			"resources - namespaces found: %v", namespacesFound)
	}
	return nil
}
