package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/eventsource"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	"golang.org/x/time/rate"
	"gotest.tools/v3/poll"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func TestStatusSyncEndToEnd(t *testing.T) {
	ctx := context.Background()
	cm := fakeCmClient()

	factory := session.NewMockContainerFactory()
	conn := factory.Create()
	go conn.Start(ctx)

	storA := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	sourceA := eventsource.NewManager(conn, eventsource.ManagerConfig{
		Source: eventsource.Info{
			ID: "A", Type: "TESTING",
			Address: "mc/sfe.A", Direct: "sfe.A",
		},
		HeartbeatInterval: 5 * time.Millisecond, BeaconInterval: 50 * time.Millisecond,
		Stores: []store.Interface{storA},
	})
	storB := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	sourceB := eventsource.NewManager(conn, eventsource.ManagerConfig{
		Source: eventsource.Info{
			ID: "B", Type: "TESTING",
			Address: "mc/sfe.B", Direct: "sfe.B",
		},
		HeartbeatInterval: 5 * time.Millisecond, BeaconInterval: 50 * time.Millisecond,
		Stores: []store.Interface{storB},
	})

	go sourceA.Run(ctx)
	go sourceB.Run(ctx)

	testCases := []struct {
		Rate     time.Duration
		RecordsA []vanflow.Record
		RecordsB []vanflow.Record
		Expected network.NetworkStatusInfo
	}{
		{
			RecordsA: []vanflow.Record{
				vanflow.SiteRecord{BaseRecord: vanflow.NewBase("site-a")},
			},
			Expected: network.NetworkStatusInfo{SiteStatus: []network.SiteStatusInfo{
				{Site: network.SiteInfo{Identity: "site-a"}},
			}},
		},
		{
			Rate: time.Millisecond,
			RecordsA: []vanflow.Record{
				vanflow.SiteRecord{
					BaseRecord: vanflow.NewBase("site-a"),
				},
				vanflow.ProcessRecord{
					BaseRecord: vanflow.NewBase("p01"), Parent: ptrTo("site-a"), Name: ptrTo("web-01"),
				},
				vanflow.ProcessRecord{BaseRecord: vanflow.NewBase("p02"), Parent: ptrTo("site-a"),
					SourceHost: ptrTo("10.0.0.101"), Name: ptrTo("web-02"),
				},
			},
			RecordsB: []vanflow.Record{
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-a"), Parent: ptrTo("site-a"), Name: ptrTo("RouterA")},
				vanflow.RouterRecord{BaseRecord: vanflow.NewBase("router-b"), Parent: ptrTo("site-a"), Name: ptrTo("RouterB")},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("connector01"), Parent: ptrTo("router-a"),
					ProcessID: ptrTo("p01"), Address: ptrTo("web"), DestHost: ptrTo("10.0.0.100"),
				},
				vanflow.ConnectorRecord{
					BaseRecord: vanflow.NewBase("connector02"), Parent: ptrTo("router-a"),
					Address: ptrTo("web"), DestHost: ptrTo("10.0.0.101"),
				},
				vanflow.ListenerRecord{
					BaseRecord: vanflow.NewBase("listener01"), Parent: ptrTo("router-a"),
					Address: ptrTo("web"),
				},
				vanflow.LinkRecord{
					BaseRecord: vanflow.NewBase("automeshlink-BtoA"), Parent: ptrTo("router-b"),
					Peer: ptrTo("routeraccess-a"), Name: ptrTo("auto-mesh-connector/0"),
				},
			},
			Expected: network.NetworkStatusInfo{
				Addresses: []network.AddressInfo{
					{Name: "web", ListenerCount: 1, ConnectorCount: 2},
				},
				SiteStatus: []network.SiteStatusInfo{
					{
						Site: network.SiteInfo{Identity: "site-a"},
						RouterStatus: []network.RouterStatusInfo{
							{
								Router: network.RouterInfo{Name: "RouterA"},
								Links:  []network.LinkInfo{},
								Listeners: []network.ListenerInfo{
									{Address: "web"},
								},
								Connectors: []network.ConnectorInfo{
									{Process: "p01", Target: "web-01", Address: "web", DestHost: "10.0.0.100"},
									{Process: "p02", Target: "web-02", Address: "web", DestHost: "10.0.0.101"},
								},
							},
							{Router: network.RouterInfo{Name: "RouterB"},
								Links: []network.LinkInfo{{Name: "auto-mesh-connector/0", Peer: "routeraccess-a"}},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			ss := NewStatusSync(factory, nil, cm, "test")
			if tc.Rate > 0 {
				ss.limit = rate.Every(tc.Rate)
			} else {
				ss.limit = 0
			}
			storA.Replace(asEntries(tc.RecordsA))
			storB.Replace(asEntries(tc.RecordsB))

			tstCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go ss.Run(tstCtx)

			poll.WaitOn(t, func(log poll.LogT) poll.Result {
				actualCM, err := cm.Get(tstCtx)
				if err != nil {
					return poll.Error(fmt.Errorf("unexpected error condition: %s", err))
				}

				statusString, ok := actualCM.Data["NetworkStatus"]
				if !ok {
					return poll.Continue("NetworkStatus not set in configmap")
				}

				var actual network.NetworkStatusInfo
				if err := json.Unmarshal([]byte(statusString), &actual); err != nil {
					return poll.Error(fmt.Errorf("invalid json found in configmap: %s", err))
				}

				if !cmp.Equal(actual, tc.Expected, cmpopts.EquateEmpty()) {
					return poll.Continue("actual does not match expected: %s", cmp.Diff(actual, tc.Expected, cmpopts.EquateEmpty()))
				}
				return poll.Success()
			}, poll.WithDelay(time.Millisecond*10), poll.WithTimeout(time.Second*2))
		})
	}
}

type fakeKubeStatusSyncClient struct {
	cm corev1.ConfigMapInterface
}

func (f *fakeKubeStatusSyncClient) Logger() *slog.Logger {
	logger := slog.New(slog.Default().Handler()).With(
		slog.String("component", "kube.flow.statusSync"),
	)
	return logger
}

func (f *fakeKubeStatusSyncClient) Get(ctx context.Context) (*v1.ConfigMap, error) {
	return f.cm.Get(ctx, "test", metav1.GetOptions{})
}

func (f *fakeKubeStatusSyncClient) Update(ctx context.Context, latest *v1.ConfigMap) error {
	_, err := f.cm.Update(ctx, latest, metav1.UpdateOptions{})
	return err
}

func fakeCmClient() StatusSyncClient {
	client := &fakeKubeStatusSyncClient{}
	client.cm = fake.NewSimpleClientset(&v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
	}).CoreV1().ConfigMaps("test")
	return client
}

func asEntries(r []vanflow.Record) []store.Entry {
	entries := make([]store.Entry, len(r))
	for i := range r {
		entries[i] = store.Entry{
			Record: r[i],
		}
	}
	return entries
}

func ptrTo[T any, R *T](in T) R {
	return &in
}
