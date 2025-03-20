package adaptor

import (
	"fmt"
	"log"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/qdr"
)

// Syncs the live router config with the configmap (bridge configuration,
// secrets for services with TLS enabled, and secrets and connectors for links)
type ConfigSync struct {
	agentPool       *qdr.AgentPool
	controller      *watchers.EventProcessor
	namespace       string
	profileSyncer   *SslProfileSyncer
	config          *watchers.ConfigMapWatcher
	secrets         *watchers.SecretWatcher
	path            string
	routerConfigMap string
}

func NewConfigSync(cli internalclient.Clients, namespace string, path string, routerConfigMap string) *ConfigSync {
	configSync := &ConfigSync{
		agentPool:       qdr.NewAgentPool("amqp://localhost:5672", nil),
		controller:      watchers.NewEventProcessor("config-sync", cli),
		namespace:       namespace,
		profileSyncer:   newSslProfileSyncer(path),
		path:            path,
		routerConfigMap: routerConfigMap,
	}
	return configSync
}

func (c *ConfigSync) Start(stopCh <-chan struct{}) error {
	if err := mkdir(c.path); err != nil {
		return err
	}
	c.config = c.controller.WatchConfigMaps(watchers.ByName(c.routerConfigMap), c.namespace, c.configEvent)
	c.secrets = c.controller.WatchAllSecrets(c.namespace, c.secretEvent)
	c.controller.StartWatchers(stopCh)
	log.Printf("CONFIG_SYNC: Waiting for informers to sync...")
	if ok := c.controller.WaitForCacheSync(stopCh); !ok {
		log.Print("CONFIG_SYNC: Failed to wait for caches to sync")
	}
	if err := c.recoverTracking(); err != nil {
		log.Printf("CONFIG_SYNC: Error recovering tracked ssl profiles: %s", err)
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

func (c *ConfigSync) trackSslProfile(profile string) (*SslProfile, bool) {
	return c.profileSyncer.get(profile)
}

func (c *ConfigSync) sync(target *SslProfile) error {
	secret, err := c.secrets.Get(c.key(target.name))
	if err != nil {
		return fmt.Errorf("CONFIG_SYNC: Error looking up secret for %s: %s", target.name, err)
	}
	if secret == nil {
		log.Printf("CONFIG_SYNC: No secret %q cached", target.name)
		return fmt.Errorf("No secret %q cached", target.name)
	}
	var wrote bool
	if err, wrote = target.sync(secret); err != nil {
		log.Printf("CONFIG_SYNC: Error syncing secret %q: %s", target.name, err)
		return err
	}
	log.Printf("CONFIG_SYNC: Secret %q synced", target.name)
	if wrote {
		if err := c.reloadSslProfileInRouter(target.name); err != nil {
			log.Printf("CONFIG_SYNC: Error reloading SslProfile %q: %s", target.name, err)
			return err
		}
		log.Printf("CONFIG_SYNC: SslProfile %q reloaded", target.name)
	}
	return nil
}

func (c *ConfigSync) secretEvent(key string, secret *corev1.Secret) error {
	if secret == nil {
		return nil
	}
	if current, ok := c.profileSyncer.bySecretName(secret.Name); ok {
		if current.secret != nil && reflect.DeepEqual(current.secret.Data, secret.Data) {
			log.Printf("CONFIG_SYNC: Secret %q already up to date", secret.Name)
			return nil
		}
		var err error
		var wrote bool
		if err, wrote = current.sync(secret); err != nil {
			log.Printf("CONFIG_SYNC: Error syncing secret %q: %s", secret.Name, err)
			return err
		}
		log.Printf("CONFIG_SYNC: Secret %q synced", secret.Name)
		if wrote {
			if err := c.reloadSslProfileInRouter(current.name); err != nil {
				log.Printf("CONFIG_SYNC: Error reloading SslProfile %q: %s", current.name, err)
				return err
			}
			log.Printf("CONFIG_SYNC: SslProfile %q reloaded", current.name)
		}
	} else {
		log.Printf("CONFIG_SYNC: Secret %q not being tracked", secret.Name)
	}
	return nil
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
		log.Printf("sync failed: %s", err)
		return err
	}
	if err := c.syncRouterConfig(desired); err != nil {
		log.Printf("sync failed: %s", err)
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

	if differences := qdr.ListenersDifference(qdr.FilterListeners(actual, qdr.IsNotNormalListener), desired.GetMatchingListeners(qdr.IsNotNormalListener)); !differences.Empty() {
		if err := agent.UpdateListenerConfig(differences); err != nil {
			return fmt.Errorf("Error syncing listeners: %s", err)
		}
	}
	return nil
}

func (c *ConfigSync) reloadSslProfileInRouter(sslProfileName string) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return err
	}
	return agent.ReloadSslProfile(sslProfileName)
}

func (c *ConfigSync) syncSslProfilesToRouter(desired map[string]qdr.SslProfile) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return err
	}
	actual, err := agent.GetSslProfiles()
	if err != nil {
		return err
	}

	for _, profile := range desired {
		if _, ok := actual[profile.Name]; !ok {
			if err := agent.CreateSslProfile(profile); err != nil {
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
	c.agentPool.Put(agent)
	return nil
}

func (c *ConfigSync) syncSslProfileCredentialsToDisk(profiles map[string]qdr.SslProfile) error {
	for _, profile := range profiles {
		if tracker, sync := c.trackSslProfile(profile.Name); sync {
			if err := c.sync(tracker); err != nil {
				return fmt.Errorf("Error synchronising secret for profile %s: %s", profile.Name, err)
			}
		}
	}
	return nil
}

func (c *ConfigSync) recoverTracking() error {
	configmap, err := c.config.Get(c.key(c.routerConfigMap))
	if err != nil {
		return err
	}
	if configmap == nil {
		return fmt.Errorf("No configmap %q", c.routerConfigMap)
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}
	for _, profile := range current.SslProfiles {
		c.trackSslProfile(profile.Name)
	}
	return nil
}
