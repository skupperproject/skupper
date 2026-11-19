package v2alpha1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func siteModePtr(m SiteMode) *SiteMode {
	return &m
}

func TestSiteSpec_GetModeAndIsEdge(t *testing.T) {
	tests := []struct {
		name     string
		spec     SiteSpec
		wantMode SiteMode
		wantEdge bool
	}{
		{
			name:     "unset mode and edge false defaults to dedicated",
			spec:     SiteSpec{},
			wantMode: SiteModeDedicated,
			wantEdge: false,
		},
		{
			name:     "unset mode and edge true maps to dedicatedEdge",
			spec:     SiteSpec{Edge: true},
			wantMode: SiteModeDedicatedEdge,
			wantEdge: true,
		},
		{
			name:     "empty mode pointer and edge false defaults to dedicated",
			spec:     SiteSpec{Mode: siteModePtr("")},
			wantMode: SiteModeDedicated,
			wantEdge: false,
		},
		{
			name:     "empty mode pointer and edge true maps to dedicatedEdge",
			spec:     SiteSpec{Edge: true, Mode: siteModePtr("")},
			wantMode: SiteModeDedicatedEdge,
			wantEdge: true,
		},
		{
			name:     "explicit dedicated",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeDedicated)},
			wantMode: SiteModeDedicated,
			wantEdge: false,
		},
		{
			name:     "explicit dedicatedEdge",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeDedicatedEdge)},
			wantMode: SiteModeDedicatedEdge,
			wantEdge: true,
		},
		{
			name:     "explicit dedicatedEdge with edge true",
			spec:     SiteSpec{Edge: true, Mode: siteModePtr(SiteModeDedicatedEdge)},
			wantMode: SiteModeDedicatedEdge,
			wantEdge: true,
		},
		{
			name:     "explicit transit",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeTransit)},
			wantMode: SiteModeTransit,
			wantEdge: false,
		},
		{
			name:     "explicit clusterShared",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeClusterShared)},
			wantMode: SiteModeClusterShared,
			wantEdge: false,
		},
		{
			name:     "explicit clusterSharedSmall",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeClusterSharedSmall)},
			wantMode: SiteModeClusterSharedSmall,
			wantEdge: false,
		},
		{
			name:     "explicit guest",
			spec:     SiteSpec{Mode: siteModePtr(SiteModeGuest)},
			wantMode: SiteModeGuest,
			wantEdge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spec.GetMode(); got != tt.wantMode {
				t.Errorf("GetMode() = %q, want %q", got, tt.wantMode)
			}
			if got := tt.spec.IsEdge(); got != tt.wantEdge {
				t.Errorf("IsEdge() = %v, want %v", got, tt.wantEdge)
			}
		})
	}
}

func TestSiteSpec_DeepCopyMode(t *testing.T) {
	t.Run("nil mode stays nil", func(t *testing.T) {
		orig := &Site{}
		copied := orig.DeepCopy()
		if copied.Spec.Mode != nil {
			t.Fatalf("expected nil Mode, got %v", *copied.Spec.Mode)
		}
	})

	t.Run("non-nil mode is copied independently", func(t *testing.T) {
		orig := &Site{
			Spec: SiteSpec{
				Mode: siteModePtr(SiteModeDedicated),
			},
		}
		copied := orig.DeepCopy()
		if copied.Spec.Mode == nil {
			t.Fatal("expected non-nil Mode in copy")
		}
		if copied.Spec.Mode == orig.Spec.Mode {
			t.Fatal("expected Mode pointer to be distinct after DeepCopy")
		}
		if *copied.Spec.Mode != SiteModeDedicated {
			t.Fatalf("expected %q, got %q", SiteModeDedicated, *copied.Spec.Mode)
		}
		*copied.Spec.Mode = SiteModeTransit
		if *orig.Spec.Mode != SiteModeDedicated {
			t.Fatalf("mutating copy affected original: got %q", *orig.Spec.Mode)
		}
	})
}

func TestSiteSpec_ModeJSON(t *testing.T) {
	t.Run("nil mode is omitted", func(t *testing.T) {
		data, err := json.Marshal(SiteSpec{})
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		if strings.Contains(string(data), `"mode"`) {
			t.Fatalf("expected mode to be omitted when nil, got %s", string(data))
		}
	})

	t.Run("set mode round-trips", func(t *testing.T) {
		orig := SiteSpec{Mode: siteModePtr(SiteModeClusterSharedSmall)}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}
		var decoded SiteSpec
		if err := json.Unmarshal(data, &decoded); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		if decoded.Mode == nil || *decoded.Mode != SiteModeClusterSharedSmall {
			t.Fatalf("expected %q, got %v", SiteModeClusterSharedSmall, decoded.Mode)
		}
	})
}

func TestSiteCRD_ModeSchema(t *testing.T) {
	crdPath := filepath.Join("..", "..", "..", "..", "config", "crd", "bases", "skupper_site_crd.yaml")
	raw, err := os.ReadFile(crdPath)
	if err != nil {
		t.Fatalf("failed reading Site CRD: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(raw, &crd); err != nil {
		t.Fatalf("failed unmarshaling Site CRD: %v", err)
	}

	if len(crd.Spec.Versions) == 0 || crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		t.Fatal("missing v2alpha1 openAPIV3Schema")
	}

	root := crd.Spec.Versions[0].Schema.OpenAPIV3Schema
	if len(root.XValidations) == 0 {
		t.Fatal("expected root x-kubernetes-validations for spec.mode immutability")
	}

	specSchema, ok := root.Properties["spec"]
	if !ok {
		t.Fatal("missing spec schema")
	}
	if len(specSchema.XValidations) == 0 {
		t.Fatal("expected spec x-kubernetes-validations for spec.edge/spec.mode consistency")
	}

	edgeSchema, ok := specSchema.Properties["edge"]
	if !ok {
		t.Fatal("missing spec.edge schema")
	}
	if !strings.Contains(strings.ToLower(edgeSchema.Description), "deprecated") {
		t.Errorf("expected spec.edge description to mention deprecation, got %q", edgeSchema.Description)
	}

	modeSchema, ok := specSchema.Properties["mode"]
	if !ok {
		t.Fatal("missing spec.mode schema")
	}
	if modeSchema.Type != "string" {
		t.Errorf("expected spec.mode type string, got %q", modeSchema.Type)
	}
	if modeSchema.Default != nil {
		t.Errorf("expected spec.mode to have no default, got %s", string(modeSchema.Default.Raw))
	}
	for _, req := range specSchema.Required {
		if req == "mode" {
			t.Error("expected spec.mode to be optional")
		}
	}

	wantEnum := []string{
		string(SiteModeDedicated),
		string(SiteModeDedicatedEdge),
		string(SiteModeTransit),
		string(SiteModeClusterShared),
		string(SiteModeClusterSharedSmall),
		string(SiteModeGuest),
	}
	var gotEnum []string
	for _, v := range modeSchema.Enum {
		var s string
		if err := json.Unmarshal(v.Raw, &s); err != nil {
			t.Fatalf("failed decoding enum value %s: %v", string(v.Raw), err)
		}
		gotEnum = append(gotEnum, s)
	}
	if !reflect.DeepEqual(gotEnum, wantEnum) {
		t.Errorf("spec.mode enum = %v, want %v", gotEnum, wantEnum)
	}
}
