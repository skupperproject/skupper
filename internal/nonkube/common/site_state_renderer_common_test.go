package common

import (
	"reflect"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCopySiteState(t *testing.T) {
	ss := fakeSiteState()
	newSs := CopySiteState(ss)
	assert.DeepEqual(t, ss.Site, newSs.Site)
	assert.Assert(t, ss.Site != newSs.Site)
	assert.DeepEqual(t, ss.Listeners, newSs.Listeners)
	assert.DeepEqual(t, ss.Connectors, newSs.Connectors)
	assert.DeepEqual(t, ss.RouterAccesses, newSs.RouterAccesses)
	assert.DeepEqual(t, ss.Grants, newSs.Grants)
	assert.DeepEqual(t, ss.Links, newSs.Links)
	assert.DeepEqual(t, ss.Secrets, newSs.Secrets)
	assert.DeepEqual(t, ss.Claims, newSs.Claims)
	assert.DeepEqual(t, ss.Certificates, newSs.Certificates)
	assert.DeepEqual(t, ss.SecuredAccesses, newSs.SecuredAccesses)
	assert.DeepEqual(t, ss.MultiKeyListeners, newSs.MultiKeyListeners)
	assert.DeepEqual(t, ss.ConfigMaps, newSs.ConfigMaps)
	assert.Assert(t, equalsButNotShallowCopy(ss.Listeners, newSs.Listeners))
	assert.Assert(t, equalsButNotShallowCopy(ss.Connectors, newSs.Connectors))
	assert.Assert(t, equalsButNotShallowCopy(ss.RouterAccesses, newSs.RouterAccesses))
	assert.Assert(t, equalsButNotShallowCopy(ss.Grants, newSs.Grants))
	assert.Assert(t, equalsButNotShallowCopy(ss.Links, newSs.Links))
	assert.Assert(t, equalsButNotShallowCopy(ss.Secrets, newSs.Secrets))
	assert.Assert(t, equalsButNotShallowCopy(ss.Claims, newSs.Claims))
	assert.Assert(t, equalsButNotShallowCopy(ss.Certificates, newSs.Certificates))
	assert.Assert(t, equalsButNotShallowCopy(ss.SecuredAccesses, newSs.SecuredAccesses))
	assert.Assert(t, equalsButNotShallowCopy(ss.MultiKeyListeners, newSs.MultiKeyListeners))
	assert.Assert(t, equalsButNotShallowCopy(ss.ConfigMaps, newSs.ConfigMaps))
}

func equalsButNotShallowCopy[T comparable](oldMap, newMap map[string]T) bool {
	for k, v := range oldMap {
		newV := newMap[k]
		if v == newV {
			return false
		}
		if !reflect.DeepEqual(v, newV) {
			return false
		}
	}
	return true
}

func TestCreateSiteRouterAccess(t *testing.T) {
	tests := []struct {
		name                   string
		linkAccess             string
		existingRouterAccesses map[string]*v2alpha1.RouterAccess
		isBundle               bool
		expectRouterAccess     bool
		expectedName           string
	}{
		{
			name:                   "LinkAccess none does not create RouterAccess",
			linkAccess:             "none",
			existingRouterAccesses: map[string]*v2alpha1.RouterAccess{},
			isBundle:               false,
			expectRouterAccess:     false,
		},
		{
			name:                   "Empty linkAccess does not create RouterAccess",
			linkAccess:             "",
			existingRouterAccesses: map[string]*v2alpha1.RouterAccess{},
			isBundle:               false,
			expectRouterAccess:     false,
		},
		{
			name:                   "create router-access-west",
			linkAccess:             "default",
			existingRouterAccesses: map[string]*v2alpha1.RouterAccess{},
			expectedName:           "router-access-test-site",
			isBundle:               false,
			expectRouterAccess:     true,
		},
		{
			name:       "router-access-test-site already exists, not recreated",
			linkAccess: "default",
			existingRouterAccesses: map[string]*v2alpha1.RouterAccess{
				"router-access-test-site": {
					ObjectMeta: metav1.ObjectMeta{
						Name: "router-access-test-site",
					},
					Spec: v2alpha1.RouterAccessSpec{
						Roles: []v2alpha1.RouterAccessRole{
							{Name: "inter-router", Port: 99999},
						},
					},
				},
			},
			isBundle:           false,
			expectRouterAccess: false,
		},
		{
			name:                   "Bundle mode doesn't create RouterAccess",
			linkAccess:             "default",
			existingRouterAccesses: map[string]*v2alpha1.RouterAccess{},
			isBundle:               true,
			expectRouterAccess:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ss := api.NewSiteState(tt.isBundle)
			ss.Site = &v2alpha1.Site{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-site",
					Namespace: "test-namespace",
				},
				Spec: v2alpha1.SiteSpec{
					LinkAccess: tt.linkAccess,
				},
			}
			ss.RouterAccesses = tt.existingRouterAccesses

			err := EnableLinkAccess(ss)
			assert.NilError(t, err)

			if tt.expectRouterAccess {

				ra, exists := ss.RouterAccesses[tt.expectedName]
				assert.Assert(t, exists, "Expected RouterAccess to be created")
				assert.Equal(t, tt.expectedName, ra.Name)

				assert.Equal(t, 2, len(ra.Spec.Roles))

				var interRouterRole, edgeRole *v2alpha1.RouterAccessRole
				for i := range ra.Spec.Roles {
					if ra.Spec.Roles[i].Name == "inter-router" {
						interRouterRole = &ra.Spec.Roles[i]
					} else if ra.Spec.Roles[i].Name == "edge" {
						edgeRole = &ra.Spec.Roles[i]
					}
				}

				assert.Assert(t, interRouterRole != nil, "Expected inter-router role")
				assert.Assert(t, edgeRole != nil, "Expected edge role")

				if !tt.isBundle {
					// Non-bundle should have allocated ports
					assert.Assert(t, interRouterRole.Port >= 55671)
					assert.Assert(t, edgeRole.Port >= 45671)
				}
			}
		})
	}
}
