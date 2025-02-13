package grants

import (
	"context"
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func Test_Initialise(t *testing.T) {
	grantUid := "0bde3bc8-a4a2-404a-bfbe-44fdf7bf3231"
	var tests = []struct {
		name           string
		config         GrantConfig
		k8sObjects     []runtime.Object
		extraSteps     int
		expectedStatus string
		expectedUrl    string
		endpoint       *v2alpha1.Endpoint
	}{
		{
			name:           "disabled",
			expectedStatus: "AccessGrants are not enabled",
		},
		{
			name: "manual",
			config: GrantConfig{
				Enabled:              true,
				BaseUrl:              "foo:5432",
				TlsCredentialsSecret: "my-creds",
			},
			k8sObjects:     []runtime.Object{tf.secret("my-creds", "test", "grant server", nil)},
			expectedStatus: "OK",
			expectedUrl:    "https://foo:5432/" + grantUid,
		},
		{
			name: "auto",
			config: GrantConfig{
				Enabled:              true,
				AutoConfigure:        true,
				Hostname:             "my-pod",
				TlsCredentialsSecret: "skupper-grant-server",
			},
			//add pregenerated secret as the certificate controller is not in action in this test
			k8sObjects: []runtime.Object{tf.pod("my-pod", "test", map[string]string{"foo": "bar"}, ref1), tf.secret("skupper-grant-server", "test", "grant server", nil)},
			endpoint: &v2alpha1.Endpoint{
				Host: "my-host",
				Port: "1234",
			},
			expectedStatus: "OK",
			expectedUrl:    "https://my-host:1234/" + grantUid,
		},
		{
			name: "failed auto",
			config: GrantConfig{
				Enabled:              true,
				AutoConfigure:        true,
				Hostname:             "my-pod",
				TlsCredentialsSecret: "skupper-grant-server",
			},
			//add pregenerated secret as the certificate controller is not in action in this test
			k8sObjects:     []runtime.Object{tf.secret("skupper-grant-server", "test", "grant server", nil)},
			expectedStatus: "Pending",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", tt.k8sObjects, []runtime.Object{tf.grant("my-grant", "test", grantUid)}, "")
			if err != nil {
				t.Error(err)
			}
			controller := internalclient.NewController("Controller", client)

			start := Initialise(controller, "test", metav1.NamespaceAll, &tt.config, nil, nil)
			if tt.endpoint != nil {
				err = updateSecuredAccessEndpoint(controller, "skupper-grant-server", "test", tt.endpoint)
				if err != nil {
					t.Error(err)
				}
			}
			stopCh := make(chan struct{})
			defer close(stopCh)
			controller.StartWatchers(stopCh)
			assert.Assert(t, controller.WaitForCacheSync(stopCh))
			if tt.config.Enabled {
				assert.Assert(t, start != nil)
				start()
			}
			for range tt.k8sObjects {
				assert.Assert(t, controller.TestProcess())
			}
			assert.Assert(t, controller.TestProcess())

			latest, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants("test").Get(context.TODO(), "my-grant", metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, latest.Status.Message, tt.expectedStatus)
			if tt.expectedUrl != "" {
				assert.Equal(t, latest.Status.Url, tt.expectedUrl)
			}

		})
	}

}

func updateSecuredAccessEndpoint(controller *internalclient.Controller, name string, namespace string, endpoint *v2alpha1.Endpoint) error {
	sa, err := controller.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if sa.Status.UpdateEndpoint(endpoint) {
		_, err = controller.GetSkupperClient().SkupperV2alpha1().SecuredAccesses(sa.Namespace).UpdateStatus(context.TODO(), sa, metav1.UpdateOptions{})
		return err
	}
	return nil
}
