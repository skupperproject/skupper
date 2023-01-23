package k8s

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"

	"github.com/skupperproject/skupper/client"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
)

// CreateResourcesFromYAML creates all resources from the provided YAML file
// or URL using an initialized VanClient instance.
func CreateResourcesFromYAML(vanClient *client.VanClient, fileOrUrl string) error {
	var yamlData []byte
	var err error

	// Load YAML from an http/https url or local file
	isUrl, _ := regexp.Compile("http[s]*://")
	if isUrl.MatchString(fileOrUrl) {
		yamlData, err = readYAMLFromUrl(fileOrUrl)
		if err != nil {
			return err
		}
	} else {
		// Read YAML file
		yamlData, err = ioutil.ReadFile(fileOrUrl)
		if err != nil {
			return fmt.Errorf("error reading yaml file: %s", err)
		}
	}

	// Creating a dynamic client
	kubeClient := vanClient.KubeClient
	dynClient, err := dynamic.NewForConfig(vanClient.RestConfig)
	if err != nil {
		return fmt.Errorf("error creating a dynamic k8s client: %s", err)
	}
	// Read YAML file removing all comments and blank lines
	// otherwise yamlDecoder does not work
	yamlBuffer, err := readYAMLIgnoringComments(yamlData)
	if err != nil {
		return err
	}

	// Creating decoder
	yamlDecoder := yamlutil.NewYAMLOrJSONDecoder(bufio.NewReader(&yamlBuffer), 1024)
	for {
		// Decoding raw object from yaml
		var rawObj runtime.RawExtension
		if err = yamlDecoder.Decode(&rawObj); err != nil {
			if err != io.EOF {
				return fmt.Errorf("error decoding yaml: %s", err)
			}
			break
		}
		// Decoded object from rawObject, with gvk (Group Version Kind)
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return fmt.Errorf("unable to decode object: %s", err)
		}
		// Converts unstructured object into a map[string]interface{}
		unstructureMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return fmt.Errorf("unable to convert to unstructured map: %s", err)
		}
		// Create a generic unstructured object from map
		unstructuredObj := &unstructured.Unstructured{Object: unstructureMap}
		// Getting API Group Resources using discovery client
		gr, err := restmapper.GetAPIGroupResources(kubeClient.Discovery())
		if err != nil {
			return fmt.Errorf("error getting APIGroupResources: %s", err)
		}
		// Unstructured object mapper for the provided group and kind
		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("error obtaining mapping for: %s - %s", gvk.GroupVersion().String(), err)
		}
		// Dynamic resource handler
		var k8sResource dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			if unstructuredObj.GetNamespace() == "" {
				unstructuredObj.SetNamespace(vanClient.GetNamespace())
			}
			k8sResource = dynClient.Resource(mapping.Resource).Namespace(unstructuredObj.GetNamespace())
		} else {
			k8sResource = dynClient.Resource(mapping.Resource)
		}
		// Creating the dynamic resource
		_, err = k8sResource.Create(context.TODO(), unstructuredObj, v1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating resource [group=%s - kind=%s] - %s", gvk.Group, gvk.Kind, err)
		}
	}

	return nil
}

// readYAMLFromUrl returns the content for the provided url
func readYAMLFromUrl(url string) ([]byte, error) {
	var yamlData []byte
	// Load from URL if url provided
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("error loading yaml from url [%s]: %s", url, err)
	}
	defer resp.Body.Close()
	yamlData, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading yaml from url [%s]: %s", url, err)
	}
	return yamlData, nil
}

// readYAMLIgnoringComments returns a bytes.Buffer that contains the
// content from the loaded yamlData removing all comments and empty lines
// that exists before the beginning of the yaml data. This is needed
// for the k8s yaml decoder to properly identify if content is a YAML
// or JSON.
func readYAMLIgnoringComments(yamlData []byte) (bytes.Buffer, error) {
	var yamlNoComments bytes.Buffer

	// We must strip all comments and blank lines from yaml file
	// otherwise the k8s yaml decoder might fail
	yamlBytesReader := bytes.NewReader(yamlData)
	yamlBufReader := bufio.NewReader(yamlBytesReader)
	yamlBufWriter := bufio.NewWriter(&yamlNoComments)

	// Regexp to exclude empty lines and lines with comments only
	// till beginning of yaml content (otherwise yaml decoder
	// won't be able to identify whether it is JSON or YAML).
	ignoreRegexp, _ := regexp.Compile("^\\s*(#|$)")
	yamlStarted := false
	for {
		line, err := yamlBufReader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if !yamlStarted && ignoreRegexp.MatchString(line) {
			continue
		}
		yamlStarted = true
		yamlBufWriter.WriteString(line)
	}
	yamlBufWriter.Flush()
	return yamlNoComments, nil
}
