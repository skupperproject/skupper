package client

import (
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	fakeCrdClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// routerAccessCRD returns a minimal RouterAccess CRD whose OpenAPI schema
// contains a "status" property with a nested "roles" property, mirroring the
// real routeraccesses.skupper.io shape used by IsCrdPathAvailable.
func routerAccessCRD(allocatedPort bool) *apiextensionsv1.CustomResourceDefinition {
	str := apiextensionsv1.JSONSchemaProps{Type: "string"}
	rolesProp := apiextensionsv1.JSONSchemaProps{
		Type: "array",
		Items: &apiextensionsv1.JSONSchemaPropsOrArray{
			Schema: &str,
		},
	}
	statusProp := apiextensionsv1.JSONSchemaProps{
		Type:       "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{},
	}
	if allocatedPort {
		statusProp.Properties["roles"] = rolesProp
	}
	schema := &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"status": statusProp,
		},
	}
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "routeraccesses.skupper.io",
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name: "v2alpha1",
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: schema,
					},
				},
			},
		},
	}
}

func newFakeCrdClient(objects ...runtime.Object) clientset.Interface {
	return fakeCrdClient.NewSimpleClientset(objects...)
}

func TestIsCrdAvailable(t *testing.T) {
	crdWithRoles := routerAccessCRD(true)

	tests := []struct {
		name    string
		client  clientset.Interface
		crdName string
		want    bool
	}{
		{
			name:    "CRD not registered",
			client:  newFakeCrdClient(),
			crdName: "routeraccesses.skupper.io",
			want:    false,
		},
		{
			name:    "CRD registered",
			client:  newFakeCrdClient(crdWithRoles),
			crdName: "routeraccesses.skupper.io",
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCrdAvailable(tt.client, tt.crdName); got != tt.want {
				t.Errorf("IsCrdAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsCrdPathAvailable(t *testing.T) {
	crdWithRoles := routerAccessCRD(true)
	crdWithoutRoles := routerAccessCRD(false)

	tests := []struct {
		name    string
		client  clientset.Interface
		crdName string
		path    string
		want    bool
	}{
		{
			name:    "CRD not registered",
			client:  newFakeCrdClient(),
			crdName: "routeraccesses.skupper.io",
			path:    "status.roles",
			want:    false,
		},
		{
			name:    "path does not exist in schema",
			client:  newFakeCrdClient(crdWithRoles),
			crdName: "routeraccesses.skupper.io",
			path:    "spec.nonexistent",
			want:    false,
		},
		{
			name:    "first segment exists but second is missing",
			client:  newFakeCrdClient(crdWithRoles),
			crdName: "routeraccesses.skupper.io",
			path:    "status.missing",
			want:    false,
		},
		{
			name:    "nested path status.roles does not exists",
			client:  newFakeCrdClient(crdWithoutRoles),
			crdName: "routeraccesses.skupper.io",
			path:    "status.roles",
			want:    false,
		},
		{
			name:    "nested path status.roles exists",
			client:  newFakeCrdClient(crdWithRoles),
			crdName: "routeraccesses.skupper.io",
			path:    "status.roles",
			want:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsCrdPathAvailable(tt.client, tt.crdName, tt.path); got != tt.want {
				t.Errorf("IsCrdPathAvailable() = %v, want %v", got, tt.want)
			}
		})
	}
}
