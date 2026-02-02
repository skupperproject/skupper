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
	if err := c.syncSslProfilesToRouter(desired.SslProfiles); err != nil {
		return err
	}
	if err := c.syncBridgeConfig(&desired.Bridges); err != nil {
		c.logger.Error("sync failed", slog.Any("error", err))
		return err
	}
	if err := c.syncRouterConfig(desired); err != nil {
		c.logger.Error("sync failed", slog.Any("error", err))
		return err
	}
	return nil
}

func syncBridgeConfig(agent *qdr.Agent, desired *qdr.BridgeConfig) (bool, error) {
	actual, err := agent.GetLocalBridgeConfig()
	if err != nil {
		return false, fmt.Errorf("Error retrieving bridges: %s", err)
	}
	differences := actual.Difference(desired)
	if differences.Empty() {
		return true, nil
	} else {
		if err = agent.UpdateLocalBridgeConfig(differences); err != nil {
			return false, fmt.Errorf("Error syncing bridges: %s", err)
		}
		return false, nil
	}
}

func (c *ConfigSync) syncBridgeConfig(desired *qdr.BridgeConfig) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}
	var synced bool

	synced, err = syncBridgeConfig(agent, desired)

	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing bridge config : %s", err)
	}
	if !synced {
		return fmt.Errorf("Bridge config is not synchronised yet")
	}
	return nil
}

func (c *ConfigSync) syncRouterConfig(desired *qdr.RouterConfig) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}

	err = syncRouterConfig(agent, desired)

	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing router config : %s", err)
	}
	return nil
}

func syncRouterConfig(agent *qdr.Agent, desired *qdr.RouterConfig) error {
	if err := syncConnectors(agent, desired); err != nil {
		return err
	}
	if err := syncListeners(agent, desired); err != nil {
		return err
	}
	return nil
}

func syncConnectors(agent *qdr.Agent, desired *qdr.RouterConfig) error {
	actual, err := agent.GetLocalConnectors()
	if err != nil {
		return fmt.Errorf("Error retrieving local connectors: %s", err)
	}

	ignorePrefix := "auto-mesh"
	if differences := qdr.ConnectorsDifference(actual, desired, &ignorePrefix); !differences.Empty() {
		if err = agent.UpdateConnectorConfig(differences); err != nil {
			return fmt.Errorf("Error syncing connectors: %s", err)
		}
	}
	return nil
}

func syncListeners(agent *qdr.Agent, desired *qdr.RouterConfig) error {
	actual, err := agent.GetLocalListeners()
	if err != nil {
		return fmt.Errorf("Error retrieving local listeners: %s", err)
	}

	if differences := qdr.ListenersDifference(qdr.FilterListeners(actual, qdr.IsNotProtectedListener), desired.GetMatchingListeners(qdr.IsNotProtectedListener)); !differences.Empty() {
		if err := agent.UpdateListenerConfig(differences); err != nil {
			return fmt.Errorf("Error syncing listeners: %s", err)
		}
	}
	return nil
}

func (c *ConfigSync) syncSslProfilesToRouter(desired map[string]qdr.SslProfile) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return err
	}
	defer c.agentPool.Put(agent)
	actual, err := agent.GetSslProfiles()
	if err != nil {
		return err
	}

	for _, profile := range desired {
		current, ok := actual[profile.Name]
		if !ok {
			if err := agent.CreateSslProfile(profile); err != nil {
				return err
			}
		}
		if current != profile {
			if err := agent.UpdateSslProfile(profile); err != nil {
				return err
			}
		}
	}
	for _, profile := range actual {
		if _, ok := desired[profile.Name]; !ok {
			if err := agent.Delete("io.skupper.router.sslProfile", profile.Name); err != nil {
				return err
			}
		}
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
