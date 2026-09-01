package v2alpha1

import (
	"testing"
)

func TestLink_IsInterVAN(t *testing.T) {
	tests := []struct {
		name      string
		endpoints []Endpoint
		want      bool
	}{
		{
			name:      "no endpoints",
			endpoints: nil,
			want:      false,
		},
		{
			name: "inter-router endpoint only",
			endpoints: []Endpoint{
				{Name: "inter-router", Host: "10.0.0.1", Port: "55671"},
			},
			want: false,
		},
		{
			name: "edge endpoint only",
			endpoints: []Endpoint{
				{Name: "edge", Host: "10.0.0.1", Port: "45671"},
			},
			want: false,
		},
		{
			name: "inter-network endpoint only",
			endpoints: []Endpoint{
				{Name: "inter-network", Host: "10.0.0.1", Port: "35671"},
			},
			want: true,
		},
		{
			name: "inter-network and inter-router endpoints",
			endpoints: []Endpoint{
				{Name: "inter-router", Host: "10.0.0.1", Port: "55671"},
				{Name: "inter-network", Host: "10.0.0.2", Port: "35671"},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &Link{
				Spec: LinkSpec{
					Endpoints: tt.endpoints,
				},
			}
			if got := l.IsInterVAN(); got != tt.want {
				t.Errorf("Link.IsInterVAN() = %v, want %v", got, tt.want)
			}
		})
	}
}
