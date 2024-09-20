package resources

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"fmt"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"

	skuppertypes "github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/images"
)

var decoder = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

//go:embed skupper-router-deployment.yaml
var routerDeploymentTemplate string

//go:embed skupper-router-local-service.yaml
var routerLocalServiceTemplate string

type NamedTemplate struct {
	name     string
	value    string
	params   interface{}
	resource schema.GroupVersionResource
}

func (t NamedTemplate) getYaml() ([]byte, error) {
	tmpl, err := template.New(t.name).Parse(t.value)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, t.params)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func resourceTemplates(site *skupperv1alpha1.Site, group string) []NamedTemplate {
	options := getCoreParams(site, group)
	templates := []NamedTemplate{
		{
			name:   "deployment",
			value:  routerDeploymentTemplate,
			params: options,
			resource: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
		{
			name:   "localService",
			value:  routerLocalServiceTemplate,
			params: options,
			resource: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
		},
	}
	return templates
}

type CoreParams struct {
	SiteId          string
	SiteName        string
	Group           string
	Replicas        int
	ServiceAccount  string
	ConfigDigest    string
	RouterImage     skuppertypes.ImageDetails
	ConfigSyncImage skuppertypes.ImageDetails
}

func configDigest(config *skupperv1alpha1.SiteSpec) string {
	if config != nil {
		// add any values from spec which require a router restart if changed:
		h := sha256.New()
		h.Write([]byte(config.RouterMode))
		if dcc := config.GetRouterDataConnectionCount(); dcc != "" {
			h.Write([]byte(dcc))
		}
		if logging := config.GetRouterLogging(); logging != "" {
			h.Write([]byte(logging))
		}
		return fmt.Sprintf("%x", h.Sum(nil))
	}
	return ""
}

func getCoreParams(site *skupperv1alpha1.Site, group string) CoreParams {
	return CoreParams{
		SiteId:          site.GetSiteId(),
		SiteName:        site.Name,
		Group:           group,
		Replicas:        1,
		ServiceAccount:  site.Spec.GetServiceAccount(),
		ConfigDigest:    configDigest(&site.Spec),
		RouterImage:     images.GetRouterImageDetails(),
		ConfigSyncImage: images.GetConfigSyncImageDetails(),
	}
}

func Apply(clients internalclient.Clients, ctx context.Context, site *skupperv1alpha1.Site, group string) error {
	for _, t := range resourceTemplates(site, group) {
		raw, err := t.getYaml()
		if err != nil {
			return err
		}
		err = apply(clients, ctx, site.Namespace, raw, t.resource)
		if err != nil {
			return err
		}
	}
	return nil
}

func apply(clients internalclient.Clients, ctx context.Context, namespace string, raw []byte, resource schema.GroupVersionResource) error {
	obj := &unstructured.Unstructured{}
	_, _, err := decoder.Decode(raw, nil, obj)
	if err != nil {
		return err
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = clients.GetDynamicClient().Resource(resource).Namespace(namespace).Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "skupper-controller",
	})
	return err
}
