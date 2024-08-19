package grants

import (
	"context"
	"testing"

	"gotest.tools/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
)

func Test_markGrantNotEnabled(t *testing.T) {
	grant := &v1alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-grant",
			Namespace: "test",
		},
		Spec: v1alpha1.AccessGrantSpec{},
		Status: v1alpha1.AccessGrantStatus{
			Status: v1alpha1.Status{
				StatusMessage: "",
			},
		},
	}
	badGrant := &v1alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i-do-not-exist-in-api-server",
			Namespace: "test",
		},
	}
	skupperObjects := []runtime.Object{
		grant,
	}
	client, err := fake.NewFakeClient("test", nil, skupperObjects, "")
	if err != nil {
		t.Error(err)
	}
	disabled := &GrantsDisabled{
		clients: client,
	}

	err = disabled.markGrantNotEnabled("", grant)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, grant.Status.StatusMessage, "AccessGrants are not enabled")
	err = disabled.markGrantNotEnabled("", nil)
	if err != nil {
		t.Error(err)
	}
	err = disabled.markGrantNotEnabled("", badGrant)
	if err != nil {
		t.Error(err)
	}
}

func Test_disabled(t *testing.T) {
	grant := &v1alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-grant",
			Namespace: "test",
		},
		Spec: v1alpha1.AccessGrantSpec{},
		Status: v1alpha1.AccessGrantStatus{
			Status: v1alpha1.Status{
				StatusMessage: "",
			},
		},
	}
	skupperObjects := []runtime.Object{
		grant,
	}
	client, err := fake.NewFakeClient("test", nil, skupperObjects, "")
	if err != nil {
		t.Error(err)
	}
	controller := kube.NewController("Controller", client)
	disabled(controller, "test")
	stopCh := make(chan struct{})
	defer close(stopCh)
	controller.StartWatchers(stopCh)
	assert.Assert(t, controller.WaitForCacheSync(stopCh))
	assert.Assert(t, controller.TestProcess())
	latest, err := client.GetSkupperClient().SkupperV1alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, latest.Status.StatusMessage, "AccessGrants are not enabled")
}
