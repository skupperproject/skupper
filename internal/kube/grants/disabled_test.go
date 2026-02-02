package grants

import (
	"context"
	"log/slog"
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func Test_markGrantNotEnabled(t *testing.T) {
	grant := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-grant",
			Namespace: "test",
		},
		Spec: v2alpha1.AccessGrantSpec{},
		Status: v2alpha1.AccessGrantStatus{
			Status: v2alpha1.Status{
				Message: "",
			},
		},
	}
	badGrant := &v2alpha1.AccessGrant{
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
		logger:  slog.Default(),
	}

	err = disabled.markGrantNotEnabled("", grant)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, grant.Status.Message, "AccessGrants are not enabled")
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
	grant := &v2alpha1.AccessGrant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-grant",
			Namespace: "test",
		},
		Spec: v2alpha1.AccessGrantSpec{},
		Status: v2alpha1.AccessGrantStatus{
			Status: v2alpha1.Status{
				Message: "",
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
	controller := watchers.NewEventProcessor("Controller", client)
	disabled(controller, "test")
	stopCh := make(chan struct{})
	defer close(stopCh)
	controller.StartWatchers(stopCh)
	assert.Assert(t, controller.WaitForCacheSync(stopCh))
	assert.Assert(t, controller.TestProcess())
	latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants(grant.Namespace).Get(context.TODO(), grant.Name, metav1.GetOptions{})
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, latest.Status.Message, "AccessGrants are not enabled")
}
