package site

import (
	"testing"

	"github.com/skupperproject/skupper/internal/qdr"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"

	//	v1 "k8s.io/client-go/applyconfigurations/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateBridgeConfigForListener(t *testing.T) {
	type args struct {
		siteId   string
		listener *skupperv2alpha1.Listener
		config   qdr.BridgeConfig
	}
	tests := []struct {
		name               string
		args               args
		expectedTcpAdded   int
		expectedTcpDeleted int
	}{
		{
			name: "no spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:   1,
			expectedTcpDeleted: 0,
		},
		{
			name: "tcp spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "tcp",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:   1,
			expectedTcpDeleted: 0,
		},
		{
			name: "bad spec type",
			args: args{
				siteId: "my-site-123",
				listener: &skupperv2alpha1.Listener{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ListenerSpec{
						RoutingKey: "echo:9090",
						Host:       "10.10.10.1",
						Port:       9090,
						Type:       "sctp",
					},
				},
				config: qdr.NewBridgeConfig(),
			},
			expectedTcpAdded:   0,
			expectedTcpDeleted: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configToUpdate := qdr.NewBridgeConfigCopy(tt.args.config)
			UpdateBridgeConfigForListener(tt.args.siteId, tt.args.listener, &configToUpdate)
			result := tt.args.config.Difference(&configToUpdate)
			assert.Assert(t, len(result.TcpListeners.Added) == tt.expectedTcpAdded)
			assert.Assert(t, len(result.TcpListeners.Deleted) == tt.expectedTcpDeleted)
		})
	}
}
