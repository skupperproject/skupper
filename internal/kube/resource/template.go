package resource

import (
	"bytes"
	"context"
	"encoding/json"
	"text/template"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type Template struct {
	Name       string
	Template   string
	Parameters interface{}
	Resource   schema.GroupVersionResource
}

func (t Template) getYaml() ([]byte, error) {
	tmpl, err := template.New(t.Name).Parse(t.Template)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	err = tmpl.Execute(&buffer, t.Parameters)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

var decoder = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func (t Template) Apply(client dynamic.Interface, ctx context.Context, namespace string) (*unstructured.Unstructured, error) {
	raw, err := t.getYaml()
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{}
	_, _, err = decoder.Decode(raw, nil, obj)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	return client.Resource(t.Resource).Namespace(namespace).Patch(ctx, obj.GetName(), types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: "skupper-controller",
	})
}
