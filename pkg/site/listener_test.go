package site

import (
	"testing"

	skupperv1alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"

	//	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateBridgeConfigForListener(t *testing.T) {
	type args struct {
		siteId   string
		listener *skupperv1alpha1.Listener
		config   qdr.BridgeConfig
	}
	tests := []struct {
		name                string
		args                args
		expectedTcpAdded    int
		expectedTcpDeleted  int
		expectedHttpAdded   int
		expectedHttpDeleted int
	}{
		{
			name: "no spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:    1,
			expectedTcpDeleted:  0,
			expectedHttpAdded:   0,
			expectedHttpDeleted: 0,
		},
		{
			name: "tcp spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:    1,
			expectedTcpDeleted:  0,
			expectedHttpAdded:   0,
			expectedHttpDeleted: 0,
		},
		{
			name: "http spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-web",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "my-web:8080",
						Host:       "10.10.10.1",
						Port:       8080,
						Type:       "http",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:    0,
			expectedTcpDeleted:  0,
			expectedHttpAdded:   1,
			expectedHttpDeleted: 0,
		},
		{
			name: "http2 spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-web",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "my-web:8080",
						Host:       "10.10.10.1",
						Port:       8080,
						Type:       "http2",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:    0,
			expectedTcpDeleted:  0,
			expectedHttpAdded:   1,
			expectedHttpDeleted: 0,
		},
		{
			name: "bad spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv1alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv1alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "sctp",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:    0,
			expectedTcpDeleted:  0,
			expectedHttpAdded:   0,
			expectedHttpDeleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configToUpdate := qdr.NewBridgeConfigCopy(tt.args.config)
			UpdateBridgeConfigForListener(tt.args.siteId, tt.args.listener, &configToUpdate)
			result := tt.args.config.Difference(&configToUpdate)
			assert.Assert(t, len(result.TcpListeners.Added) == tt.expectedTcpAdded)
			assert.Assert(t, len(result.TcpListeners.Deleted) == tt.expectedTcpDeleted)
			assert.Assert(t, len(result.HttpListeners.Added) == tt.expectedHttpAdded)
			assert.Assert(t, len(result.HttpListeners.Deleted) == tt.expectedHttpDeleted)
		})
	}
}
