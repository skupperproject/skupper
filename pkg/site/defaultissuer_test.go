package site

import (
	"testing"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultIssuer(t *testing.T) {
	type args struct {
		site *skupperv1alpha1.Site
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "nil site",
			args: args{
				site: nil,
			},
			want: "skupper-site-ca",
		},
		{
			name: "spec issuer",
			args: args{
				site: &skupperv1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SiteSpec{
						DefaultIssuer: "skupper-spec-issuer-ca",
					},
					Status: skupperv1alpha1.SiteStatus{
						DefaultIssuer: "skupper-status-issuer-ca",
					},
				},
			},
			want: "skupper-spec-issuer-ca",
		},
		{
			name: "status issuer",
			args: args{
				site: &skupperv1alpha1.Site{
					ObjectMeta: v1.ObjectMeta{
						Name:      "site1",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.SiteSpec{
						DefaultIssuer: "",
					},
					Status: skupperv1alpha1.SiteStatus{
						DefaultIssuer: "skupper-status-issuer-ca",
					},
				},
			},
			want: "skupper-status-issuer-ca",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultIssuer(tt.args.site); got != tt.want {
				t.Errorf("DefaultIssuer() = %v, want %v", got, tt.want)
			}
		})
	}
}
