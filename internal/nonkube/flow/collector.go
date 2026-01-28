package flow

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/flow"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusSyncClient struct {
	client    *fs.ConfigMapHandler
	namespace string
}

func (s *StatusSyncClient) Logger() *slog.Logger {
	logger := slog.New(slog.Default().Handler()).With(
		slog.String("component", "nonkube.flow.statusSync"),
		slog.String("namespace", s.namespace),
	)
	return logger
}

func (s *StatusSyncClient) Get(ctx context.Context) (*corev1.ConfigMap, error) {
	return s.client.Get(types.NetworkStatusConfigMapName, fs.GetOptions{
		RuntimeFirst: true,
		LogWarning:   false,
	})
}

func (s *StatusSyncClient) Update(ctx context.Context, latest *corev1.ConfigMap) error {
	return s.client.Update(*latest, true)
}

func siteCollector(ctx context.Context, namespace string) error {
	platformLoader := &common.NamespacePlatformLoader{}
	platform, err := platformLoader.Load(namespace)
	if err != nil {
		return err
	}
	if platform == string(types.PlatformKubernetes) {
		return fmt.Errorf("Invalid platform: %s", types.PlatformKubernetes)
	}

	client := fs.NewConfigMapHandler(namespace)
	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      types.NetworkStatusConfigMapName,
			Namespace: namespace,
		},
		Data: map[string]string{},
	}
	if err := client.Add(cm, true); err != nil {
		return err
	}

	address, err := runtime.GetLocalRouterAddress(namespace)
	if err != nil {
		return err
	}
	tlsConfig, err := getLocalTLSConfig(namespace)
	if err != nil {
		return err
	}
	factory := session.NewContainerFactory(address, session.ContainerConfig{
		ContainerID: "nonkube-flow-collector",
		TLSConfig:   tlsConfig,
		SASLType:    session.SASLTypeExternal,
	})
	statusSyncClient := &StatusSyncClient{
		client:    fs.NewConfigMapHandler(namespace),
		namespace: namespace,
	}
	statusSync := flow.NewStatusSync(factory, nil, statusSyncClient, types.NetworkStatusConfigMapName)
	go statusSync.Run(ctx)
	go func() {
		<-ctx.Done()
		_ = client.Delete(cm.Name, true)
	}()
	return nil
}

func getLocalTLSConfig(namespace string) (*tls.Config, error) {
	tlsCert := runtime.GetRuntimeTlsCert(namespace, "skupper-local-client")
	config, err := tlsCert.GetTlsConfig()
	if err == nil {
		config.MinVersion = tls.VersionTLS13
	}
	return config, err
}

func StartCollector(ctx context.Context, namespace string) error {
	slog.Info("COLLECTOR: Starting site collection", slog.String("namespace", namespace))
	if err := siteCollector(ctx, namespace); err != nil {
		return err
	}
	return nil
}
