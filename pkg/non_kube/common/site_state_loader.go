package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/non_kube/apis"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

type FileSystemSiteStateLoader struct {
	Path   string
	Bundle bool
}

func (f *FileSystemSiteStateLoader) Load() (*apis.SiteState, error) {
	var siteState = apis.NewSiteState(f.Bundle)
	yamlFileNames, err := f.readAllFiles(f.Path)
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
			return siteState, err
		}
	}
	return siteState, nil
}

func LoadIntoSiteState(reader *bufio.Reader, siteState *apis.SiteState) error {
	var err error
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
	return nil
}

func (f *FileSystemSiteStateLoader) readAllFiles(inputDir string) ([]string, error) {
	dir, err := os.Open(inputDir)
	if err != nil {
		return nil, err
	}
	dirInfo, err := dir.Stat()
	if err != nil {
		return nil, err
	}
	if !dirInfo.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", inputDir)
	}
	files, err := dir.ReadDir(0)
	if err != nil {
		return nil, err
	}
	var fileNames []string
	for _, file := range files {
		if file.IsDir() {
			recursiveFiles, err := f.readAllFiles(path.Join(inputDir, file.Name()))
			if err != nil {
				return nil, err
			}
			fileNames = append(fileNames, recursiveFiles...)
		} else {
			if strings.HasSuffix(file.Name(), ".yaml") || strings.HasSuffix(file.Name(), ".yml") {
				fileNames = append(fileNames, path.Join(inputDir, file.Name()))
			}
		}
	}
	return fileNames, nil
}
