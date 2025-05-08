package fs

import (
	"bufio"
	"fmt"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"

	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"log/slog"
)

type InputFileResource struct {
	Site          []v2alpha1.Site
	Listener      []v2alpha1.Listener
	Connector     []v2alpha1.Connector
	RouterAccess  []v2alpha1.RouterAccess
	AccessGrant   []v2alpha1.AccessGrant
	Link          []v2alpha1.Link
	AccessToken   []v2alpha1.AccessToken
	Certificate   []v2alpha1.Certificate
	SecuredAccess []v2alpha1.SecuredAccess
	Secret        []corev1.Secret
}

func ParseInput(namespace string, reader *bufio.Reader, result *InputFileResource) error {

	var err error
	logger := slog.New(slog.Default().Handler())
	logInvalidResource := func(gvk *schema.GroupVersionKind) {
		logger.Warn("invalid resource ignored:", slog.String("resource", gvk.String()))
	}
	yamlJsonDecoder := yamlutil.NewYAMLOrJSONDecoder(reader, 1024)
	// allow reading multiple-document file
	for {
		var rawObj runtime.RawExtension
		err = yamlJsonDecoder.Decode(&rawObj)
		if err != nil {
			if err != io.EOF {
				return fmt.Errorf("error decoding file: %s", err)
			}
			break
		}
		// Decoded object from rawObject, with gvk (Group Version Kind)
		obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
		if err != nil {
			return err
		}

		if v2alpha1.SchemeGroupVersion == gvk.GroupVersion() {
			switch gvk.Kind {
			case "Site":
				var site v2alpha1.Site
				convertTo(obj, &site)
				site.Namespace = namespace
				result.Site = append(result.Site, site)
			case "Listener":
				var listener v2alpha1.Listener
				convertTo(obj, &listener)
				listener.Namespace = namespace
				result.Listener = append(result.Listener, listener)
			case "Connector":
				var connector v2alpha1.Connector
				convertTo(obj, &connector)
				connector.Namespace = namespace
				result.Connector = append(result.Connector, connector)
			case "RouterAccess":
				var routerAccess v2alpha1.RouterAccess
				convertTo(obj, &routerAccess)
				routerAccess.Namespace = namespace
				result.RouterAccess = append(result.RouterAccess, routerAccess)
			case "AccessGrant":
				var grant v2alpha1.AccessGrant
				convertTo(obj, &grant)
				grant.Namespace = namespace
				result.AccessGrant = append(result.AccessGrant, grant)
			case "Link":
				var link v2alpha1.Link
				convertTo(obj, &link)
				link.Namespace = namespace
				result.Link = append(result.Link, link)
			case "AccessToken":
				var claim v2alpha1.AccessToken
				convertTo(obj, &claim)
				claim.Namespace = namespace
				result.AccessToken = append(result.AccessToken, claim)
			case "Certificate":
				var certificate v2alpha1.Certificate
				convertTo(obj, &certificate)
				result.Certificate = append(result.Certificate, certificate)
			case "SecuredAccess":
				var securedAccess v2alpha1.SecuredAccess
				convertTo(obj, &securedAccess)
				securedAccess.Namespace = namespace
				result.SecuredAccess = append(result.SecuredAccess, securedAccess)
			default:
				logInvalidResource(gvk)
			}
		} else if corev1.SchemeGroupVersion == gvk.GroupVersion() {
			switch gvk.Kind {
			case "Secret":
				var secret corev1.Secret
				convertTo(obj, &secret)
				secret.Namespace = namespace
				result.Secret = append(result.Secret, secret)
			default:
				logInvalidResource(gvk)
			}
		} else {
			logInvalidResource(gvk)
		}
	}
	return nil

}

func convertTo(obj runtime.Object, target interface{}) {
	runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), target)
}
