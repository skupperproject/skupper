package utils

import (
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func Test_Marshal(t *testing.T) {
	tests := []struct {
		outputType     string
		resource       any
		expectedOutput string
		hasError       bool
	}{
		{"json", v1alpha1.Site{
			TypeMeta: v1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Site",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "my-site",
				Namespace: "test",
			},
			Spec: v1alpha1.SiteSpec{
				LinkAccess: "default",
			},
		},
			`{
  "apiVersion": "skupper.io/v1alpha1",
  "kind": "Site",
  "metadata": {
    "name": "my-site",
    "namespace": "test"
  },
  "spec": {
    "linkAccess": "default"
  }
}`, false},
		{"yaml", v1alpha1.Site{
			TypeMeta: v1.TypeMeta{
				APIVersion: "skupper.io/v1alpha1",
				Kind:       "Site",
			},
			ObjectMeta: v1.ObjectMeta{
				Name:      "my-site",
				Namespace: "test",
			},
			Spec: v1alpha1.SiteSpec{
				LinkAccess: "default",
			},
		},
			`apiVersion: skupper.io/v1alpha1
kind: Site
metadata:
  name: my-site
  namespace: test
spec:
  linkAccess: default
`,
			false},
		{"unsupported", v1alpha1.Site{
			ObjectMeta: v1.ObjectMeta{
				Name:      "my-site",
				Namespace: "test",
			},
		}, ``, true},
	}

	for _, tt := range tests {
		output, err := Encode(tt.outputType, tt.resource)
		if err != nil && !tt.hasError {
			t.Fatalf("Expected no error, but got: %v", err)
		}
		if err == nil && tt.hasError {
			t.Fatal("Expected an error, but got none")
		}

		if output != tt.expectedOutput {
			t.Errorf("Expected output %v but got %v", tt.expectedOutput, output)
		}
	}
}
