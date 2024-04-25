package common

import (
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/non_kube/apis"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSiteStateValidator_Validate(t *testing.T) {
	tests := []struct {
		info          string
		siteState     *apis.SiteState
		valid         bool
		errorContains string
	}{
		{
			info: "invalid-site-name",
			siteState: customize(func(siteState *apis.SiteState) {
				siteState.Site.Name = "bad_name"
			}),
			valid:         false,
			errorContains: "invalid site name:",
		},
		{
			info: "invalid-link-access-name",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, la := range siteState.RouterAccesses {
					la.Name = "bad_name"
					break
				}
			}),
			valid:         false,
			errorContains: "invalid router access name:",
		},
		{
			info: "invalid-link-access-tlsCredentials-required",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, la := range siteState.RouterAccesses {
					la.Spec.TlsCredentials = ""
					break
				}
			}),
			valid:         false,
			errorContains: "invalid router access tls credentials: empty",
		},
		{
			info: "invalid-link-access-roles-required",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, la := range siteState.RouterAccesses {
					la.Spec.Roles = make([]v1alpha1.RouterAccessRole, 0)
					break
				}
			}),
			valid:         false,
			errorContains: "invalid router access: roles are required",
		},
		{
			info: "invalid-link-access-roles-invalid-role",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, la := range siteState.RouterAccesses {
					la.Spec.Roles[0].Name = "bad-role"
					break
				}
			}),
			valid:         false,
			errorContains: "invalid role: ",
		},
		{
			info: "invalid-links-no-secrets",
			siteState: customize(func(siteState *apis.SiteState) {
				siteState.Secrets = make(map[string]*corev1.Secret)
			}),
			valid:         false,
			errorContains: "no secrets found",
		},
		{
			info: "invalid-links-name",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, link := range siteState.Links {
					link.Name = "bad_name"
					break
				}
			}),
			valid:         false,
			errorContains: "invalid link name: ",
		},
		{
			info: "invalid-links-secret-not-found",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, link := range siteState.Links {
					link.Spec.TlsCredentials = "invalid"
					break
				}
			}),
			valid:         false,
			errorContains: "secret \"invalid\" not found",
		},
		{
			info: "invalid-listener-name",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, listener := range siteState.Listeners {
					listener.Name = "bad_name"
				}
			}),
			valid:         false,
			errorContains: "invalid listener name: ",
		},
		{
			info: "invalid-listener-host",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, listener := range siteState.Listeners {
					listener.Spec.Host = ""
				}
			}),
			valid:         false,
			errorContains: "host and port are required",
		},
		{
			info: "invalid-listener-host",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, listener := range siteState.Listeners {
					listener.Spec.Host = "1.2.3.4"
					listener.Spec.Port = 0
				}
			}),
			valid:         false,
			errorContains: "host and port are required",
		},
		{
			info: "invalid-listener-host-invalid-ip",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, listener := range siteState.Listeners {
					listener.Spec.Host = "invalid_hostname"
				}
			}),
			valid:         false,
			errorContains: "invalid listener host: ",
		},
		{
			info: "invalid-listener-port-already-mapped",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, listener := range siteState.Listeners {
					listener.Spec.Host = "1.2.3.4"
				}
			}),
			valid:         false,
			errorContains: "is already mapped for host",
		},
		{
			info: "invalid-connector-name",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, connector := range siteState.Connectors {
					connector.Name = "bad_name"
				}
			}),
			valid:         false,
			errorContains: "invalid connector name: ",
		},
		{
			info: "invalid-connector-host",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, connector := range siteState.Connectors {
					connector.Spec.Host = ""
				}
			}),
			valid:         false,
			errorContains: "host and port are required",
		},
		{
			info: "invalid-connector-host",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, connector := range siteState.Connectors {
					connector.Spec.Host = "1.2.3.4"
					connector.Spec.Port = 0
				}
			}),
			valid:         false,
			errorContains: "host and port are required",
		},
		{
			info: "invalid-connector-host-invalid-ip",
			siteState: customize(func(siteState *apis.SiteState) {
				for _, connector := range siteState.Connectors {
					connector.Spec.Host = "invalid_hostname"
				}
			}),
			valid:         false,
			errorContains: "invalid connector host: ",
		},
		{
			info: "invalid-claim-name",
			siteState: customize(func(siteState *apis.SiteState) {
				siteState.Claims["bad_name"] = &v1alpha1.AccessToken{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Claim",
						APIVersion: v1alpha1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad_name",
					},
				}
			}),
			valid:         false,
			errorContains: "invalid access token name: ",
		},
		{
			info: "invalid-grant-name",
			siteState: customize(func(siteState *apis.SiteState) {
				siteState.Grants["bad_name"] = &v1alpha1.AccessGrant{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Grant",
						APIVersion: v1alpha1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "bad_name",
					},
				}
			}),
			valid:         false,
			errorContains: "invalid grant name: ",
		},
		{
			info:      "valid-site-state",
			siteState: fakeSiteState(),
			valid:     true,
		},
	}
	validator := &SiteStateValidator{}
	for _, test := range tests {
		t.Run(test.info, func(t *testing.T) {
			err := validator.Validate(test.siteState)
			assert.Equal(t, err == nil, test.valid, err)
			if !test.valid {
				assert.ErrorContains(t, err, test.errorContains)
			}
		})
	}
}

func customize(fn func(siteState *apis.SiteState)) *apis.SiteState {
	siteState := fakeSiteState()
	fn(siteState)
	return siteState
}
