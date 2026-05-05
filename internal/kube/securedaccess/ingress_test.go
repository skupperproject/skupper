package securedaccess

import "testing"

func TestIngressClassNameForDesiredIngress(t *testing.T) {
	tests := []struct {
		name         string
		nginx        bool
		defaultClass string
		settings     map[string]string
		want         string
	}{
		{
			name:  "ingress-nginx implicit nginx",
			nginx: true,
			want:  "nginx",
		},
		{
			name:         "ingress-nginx global default",
			nginx:        true,
			defaultClass: "openshift-public",
			want:         "openshift-public",
		},
		{
			name:         "settings override global",
			nginx:        true,
			defaultClass: "nginx",
			settings:     map[string]string{SettingIngressClassName: "public-ic"},
			want:         "public-ic",
		},
		{
			name:  "plain ingress no default",
			nginx: false,
			want:  "",
		},
		{
			name:         "plain ingress with global",
			nginx:        false,
			defaultClass: "openshift-public",
			want:         "openshift-public",
		},
		{
			name:     "plain ingress settings only",
			nginx:    false,
			settings: map[string]string{SettingIngressClassName: "custom"},
			want:     "custom",
		},
		{
			name:     "trim spaces",
			nginx:    false,
			settings: map[string]string{SettingIngressClassName: "  x  "},
			want:     "x",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ingressClassNameForDesiredIngress(tt.nginx, tt.defaultClass, tt.settings)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
