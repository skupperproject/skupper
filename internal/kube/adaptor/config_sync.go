package adaptor

import (
	"fmt"
	"log/slog"
	"os"

	corev1 "k8s.io/api/core/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/secrets"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/qdr"
)

// Syncs the live router config with the configmap (bridge configuration,
// secrets for services with TLS enabled, and secrets and connectors for links)
type ConfigSync struct {
	agentPool       *qdr.AgentPool
	controller      *watchers.EventProcessor
	namespace       string
	profileSyncer   *secrets.Sync
	config          *watchers.ConfigMapWatcher
	path            string
	routerConfigMap string
	logger          *slog.Logger
}

func sslSecretsWatcher(namespace string, eventProcessor *watchers.EventProcessor) secrets.SecretsCacheFactory {
	return func(stopCh <-chan struct{}, handler func(string, *corev1.Secret) error) secrets.SecretsCache {
		m := eventProcessor.WatchAllSecrets(namespace, handler)
		m.Start(stopCh)
		return m
	}
}

func NewConfigSync(cli internalclient.Clients, namespace string, path string, routerConfigMap string, metrics watchers.MetricsProvider) *ConfigSync {
	controller := watchers.NewEventProcessor("config-sync", cli, watchers.WithMetricsProvider(metrics))
	configSync := &ConfigSync{
		agentPool:       qdr.NewAgentPool("amqp://localhost:5672", nil),
		controller:      controller,
		namespace:       namespace,
		path:            path,
		routerConfigMap: routerConfigMap,
		logger:          slog.New(slog.Default().Handler()).With(slog.String("component", "kube.adaptor.configSync")),
	}
	configSync.profileSyncer = secrets.NewSync(
		sslSecretsWatcher(namespace, controller),
		configSync.recheckProfile,
		slog.New(slog.Default().Handler()).With(slog.String("component", "kube.secrets")),
	)
	return configSync
}

func (c *ConfigSync) recheckProfile(_ string) {
	key := c.key(c.routerConfigMap)
	configmap, err := c.config.Get(key)
	if err != nil {
		return
	}
	if err := c.configEvent(key, configmap); err != nil {
		c.logger.Error("CONFIG_SYNC: Error handling configuration after secret change", slog.Any("error", err))
	}
}

func (c *ConfigSync) Start(stopCh <-chan struct{}) error {
	if err := mkdir(c.path); err != nil {
		return err
	}
	c.config = c.controller.WatchConfigMaps(watchers.ByName(c.routerConfigMap), c.namespace, c.configEvent)
	c.controller.StartWatchers(stopCh)
	c.logger.Info("CONFIG_SYNC: Waiting for informers to sync...")
	if ok := c.controller.WaitForCacheSync(stopCh); !ok {
		c.logger.Error("CONFIG_SYNC: Failed to wait for caches to sync")
	}
	if err := c.recoverTracking(); err != nil {
		c.logger.Error("CONFIG_SYNC: Error recovering tracked ssl profiles", slog.Any("error", err))
	}
	c.controller.Start(stopCh)
	return nil
}

func (c *ConfigSync) Stop() {
	c.controller.Stop()
}

func (c *ConfigSync) key(name string) string {
	return fmt.Sprintf("%s/%s", c.namespace, name)
}

func (c *ConfigSync) configEvent(key string, configmap *corev1.ConfigMap) error {
	if configmap == nil {
		return nil
	}
	desired, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}
	if err := c.syncSslProfileCredentialsToDisk(desired.SslProfiles); err != nil {
		return err
	}
	if err := qdr.SyncSslProfilesToRouter(c.agentPool, desired.SslProfiles); err != nil {
		return err
	}
	if err := qdr.SyncBridgeConfig(c.agentPool, &desired.Bridges); err != nil {
		c.logger.Error("sync failed", slog.Any("error", err))
		return err
	}
	if err := qdr.SyncRouterConfig(c.agentPool, desired, true); err != nil {
		c.logger.Error("sync failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *ConfigSync) syncSslProfileCredentialsToDisk(profiles map[string]qdr.SslProfile) error {
	delta := c.profileSyncer.Expect(profiles)
	return delta.Error()
}

func (c *ConfigSync) recoverTracking() error {
	configmap, err := c.config.Get(c.key(c.routerConfigMap))
	if err != nil {
		return err
	}
	if configmap == nil {
		return fmt.Errorf("No configmap %q", c.routerConfigMap)
	}

	if _, err := qdr.GetRouterConfigFromConfigMap(configmap); err != nil {
		return err
	}
	c.profileSyncer.Recover()
	return nil
}

func mkdir(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		err = os.Mkdir(path, 0777)
		if err != nil {
			return err
		}
	}
	return nil
}
