package runtime

import (
	"os"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetLocalRouterAddress(t *testing.T) {
	tempDir := t.TempDir()
	if os.Getuid() == 0 {
		api.DefaultRootDataHome = tempDir
	} else {
		t.Setenv("XDG_DATA_HOME", tempDir)
	}

	namespace := "test-get-local-router-address"
	var ss = api.NewSiteState(false)
	ss.RouterAccesses["skupper-local"] = fakeSkupperLocalRouterAccess(namespace)
	outputDir := api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath)

	// Try to get without skupper-local RouterAccess
	routerAddress, err := GetLocalRouterAddress(namespace)
	assert.ErrorContains(t, err, "unable to determine router port:")

	// Try again, now with skupper-local RouterAccess present
	assert.Assert(t, api.MarshalSiteState(*ss, outputDir))
	routerAddress, err = GetLocalRouterAddress(namespace)
	assert.Assert(t, err)
	assert.Equal(t, routerAddress, "amqps://127.0.0.1:5671")

	// Make router access invalid
	ss.RouterAccesses["skupper-local"].Spec.Roles = nil
	assert.Assert(t, api.MarshalSiteState(*ss, outputDir))
	routerAddress, err = GetLocalRouterAddress(namespace)
	assert.ErrorContains(t, err, "no roles defined on RouterAccess: skupper-local")
}

func fakeSkupperLocalRouterAccess(namespace string) *v2alpha1.RouterAccess {
	return &v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "skupper-local",
			Namespace: namespace,
		},
		Spec: v2alpha1.RouterAccessSpec{
			Roles: []v2alpha1.RouterAccessRole{
				{
					Name: "normal",
					Port: 5671,
				},
			},
			BindHost: "127.0.0.1",
		},
	}
}
