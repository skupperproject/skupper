package site

import (
	"testing"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateBridgeConfigForConnector(t *testing.T) {
	type args struct {
		siteId    string
		connector *skupperv2alpha1.Connector
		config    qdr.BridgeConfig
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
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
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
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "echo",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
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
				connector: &skupperv2alpha1.Connector{
					ObjectMeta: v1.ObjectMeta{
						Name:      "my-web",
						Namespace: "test",
					},
					Spec: skupperv2alpha1.ConnectorSpec{
						RoutingKey: "my-web:8080",
						Host:       "10.10.10.1",
						Port:       8080,
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
			UpdateBridgeConfigForConnector(tt.args.siteId, tt.args.connector, &configToUpdate)
			result := tt.args.config.Difference(&configToUpdate)
			assert.Assert(t, len(result.TcpConnectors.Added) == tt.expectedTcpAdded)
			assert.Assert(t, len(result.TcpConnectors.Deleted) == tt.expectedTcpDeleted)
		})
	}
}
