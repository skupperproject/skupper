package flow

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/flow"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils/tlscfg"
	"github.com/skupperproject/skupper/internal/version"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/vanflow"
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
		slog.String("namespace", s.namespace),
		slog.String("component", "nonkube.flow.statusSync"),
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
	platform, err := runtime.GetPlatform(namespace)
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

	return nil
}

func getLocalTLSConfig(namespace string) (*tls.Config, error) {
	certsPath := api.GetInternalOutputPath(namespace, api.CertificatesPath)
	localClientPath := path.Join(certsPath, "skupper-local-client")
	config := tlscfg.Modern()
	caFile := path.Join(localClientPath, "ca.crt")
	certFile := path.Join(localClientPath, "tls.crt")
	certKey := path.Join(localClientPath, "tls.key")

	certPool := x509.NewCertPool()
	if caData, err := os.ReadFile(caFile); err == nil {
		certPool.AppendCertsFromPEM(caData)
		config.RootCAs = certPool
	} else {
		return nil, err
	}
	if cert, err := tls.LoadX509KeyPair(certFile, certKey); err == nil {
		config.Certificates = []tls.Certificate{cert}
	} else {
		return nil, err
	}

	return config, nil
}

func startFlowController(ctx context.Context, namespace string) error {
	siteStateLoader := &common.FileSystemSiteStateLoader{
		Path: api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath),
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return err
	}
	siteID := siteState.SiteId
	siteName := siteState.Site.Name

	platform, err := runtime.GetPlatform(namespace)
	if err != nil {
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

	creation := time.Now()
	for _, condition := range siteState.Site.Status.Conditions {
		if condition.Type == string(v2alpha1.StatusReady) {
			creation = condition.LastTransitionTime.Time
		}
	}
	fc := NewController(ControllerConfig{
		Factory: session.NewContainerFactory(address, session.ContainerConfig{
			ContainerID: "nonkube-flow-controller",
			TLSConfig:   tlsConfig,
			SASLType:    session.SASLTypeExternal,
		}),
		Site: vanflow.SiteRecord{
			BaseRecord: vanflow.NewBase(siteID, creation),
			Name:       &siteName,
			Namespace:  &namespace,
			Platform:   &platform,
			Version:    &version.Version,
			Provider:   &platform, //todo(ck) Not really correct. involved with nodes access (below)
		},
	})
	go func() {
		fc.Run(ctx)
		if ctx.Err() == nil {
			slog.Error("nonkube flow controller unexpectedly quit")
		}
	}()
	return nil
}

func StartCollector(ctx context.Context, namespace string) error {
	log.Println("COLLECTOR: Starting site collection for:", namespace)
	if err := siteCollector(ctx, namespace); err != nil {
		return err
	}
	if err := startFlowController(ctx, namespace); err != nil {
		log.Printf("COLLECTOR: Failed to start controller for emitting site events: %s", err)
		return err
	}
	return nil
}
