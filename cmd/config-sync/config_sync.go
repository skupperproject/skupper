package main

import (
	"fmt"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/utils"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"math"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/skupperproject/skupper/pkg/qdr"
)

// Syncs the live router config with the configmap (bridge configuration,
//secrets for services with TLS enabled, and secrets and connectors for links)
type ConfigSync struct {
	informer  cache.SharedIndexInformer
	events    workqueue.RateLimitingInterface
	agentPool *qdr.AgentPool
	vanClient *client.VanClient
}

const SHARED_TLS_DIRECTORY = "/etc/qpid-dispatch-certs"

func enqueue(events workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err == nil {
		events.Add(key)
	} else {
		log.Printf("Error getting key: %s", err)
	}

}

func newEventHandler(events workqueue.RateLimitingInterface) *cache.ResourceEventHandlerFuncs {
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			enqueue(events, obj)
		},
		UpdateFunc: func(old, new interface{}) {
			enqueue(events, new)
		},
		DeleteFunc: func(obj interface{}) {
			enqueue(events, obj)
		},
	}
}

func newConfigSync(configInformer cache.SharedIndexInformer, cli *client.VanClient) *ConfigSync {
	configSync := &ConfigSync{
		informer:  configInformer,
		agentPool: qdr.NewAgentPool("amqp://localhost:5672", nil),
		vanClient: cli,
	}
	configSync.events = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "skupper-config-sync")
	configSync.informer.AddEventHandler(newEventHandler(configSync.events))
	return configSync
}

func (c *ConfigSync) start(stopCh <-chan struct{}) error {
	go wait.Until(c.runConfigSync, time.Second, stopCh)

	return nil
}

func (c *ConfigSync) stop() {
	c.events.ShutDown()
}

func (c *ConfigSync) runConfigSync() {
	for c.processNextEvent() {
	}
}

const (
	ConfigSyncEvent string = "ConfigSyncEvent"
	ConfigSyncError string = "ConfigSyncError"
)

func (c *ConfigSync) processNextEvent() bool {
	log.Println("getting sync event")
	obj, shutdown := c.events.Get()
	log.Println("sync triggered")

	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.events.Done(obj)

		var ok bool
		var key string
		if key, ok = obj.(string); !ok {
			// invalid item
			log.Printf("expected string in events but got %#v", obj)
			c.events.Forget(obj)
			return fmt.Errorf("expected string in events but got %#v", obj)
		} else {
			obj, exists, err := c.informer.GetStore().GetByKey(key)
			if err != nil {
				return fmt.Errorf("Error reading pod from cache: %s", err)
			}
			if exists {
				configmap, ok := obj.(*corev1.ConfigMap)
				if !ok {
					return fmt.Errorf("Expected ConfigMap for %s but got %#v", key, obj)
				}
				bridges, err := qdr.GetBridgeConfigFromConfigMap(configmap)
				if err != nil {
					return fmt.Errorf("Error parsing bridge configuration from %s: %s", key, err)
				}
				err = c.syncConfig(bridges)
				if err != nil {
					log.Printf("sync failed: %s", err)
					return err
				}

				routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
				if err != nil {
					return err
				}

				err = c.syncRouterConfig(routerConfig)
				if err != nil {
					log.Printf("sync failed: %s", err)
					return err
				}
			}
		}
		log.Println("sync succeeded")
		c.events.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		if c.events.NumRequeues(obj) < 5 {
			log.Printf("Requeuing %v after error: %v", obj, err)
			c.events.AddRateLimited(obj)
		} else {
			log.Printf("Delayed requeue %v after error: %v", obj, err)
			c.events.AddAfter(obj, time.Duration(math.Min(float64(c.events.NumRequeues(obj)/5), 10))*time.Minute)
		}
		utilruntime.HandleError(err)
	}

	return true
}

func syncConfig(agent *qdr.Agent, desired *qdr.BridgeConfig, c *ConfigSync) (bool, error) {
	actual, err := agent.GetLocalBridgeConfig()
	if err != nil {
		return false, fmt.Errorf("Error retrieving bridges: %s", err)
	}
	differences := actual.Difference(desired)
	if differences.Empty() {
		err = c.checkSecrets(desired, SHARED_TLS_DIRECTORY)
		if err != nil {
			return false, err
		}
		return true, nil
	} else {
		differences.Print()

		err := syncSecrets(c, differences, SHARED_TLS_DIRECTORY)
		if err != nil {
			return false, fmt.Errorf("error syncing secrets: %s", err)
		}

		if err = agent.UpdateLocalBridgeConfig(differences); err != nil {
			return false, fmt.Errorf("Error syncing bridges: %s", err)
		}
		return false, nil
	}
}

func (c *ConfigSync) syncConfig(desired *qdr.BridgeConfig) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}
	var synced bool
	for i := 0; i < 3 && err == nil && !synced; i++ {
		synced, err = syncConfig(agent, desired, c)
	}
	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing bridge config : %s", err)
	}
	if !synced {
		return fmt.Errorf("Failed to sync bridge config")
	}
	return nil
}

func (c *ConfigSync) syncRouterConfig(desired *qdr.RouterConfig) error {
	agent, err := c.agentPool.Get()
	if err != nil {
		return fmt.Errorf("Could not get management agent : %s", err)
	}
	var synced bool
	for i := 0; i < 3 && err == nil && !synced; i++ {
		synced, err = syncRouterConfig(agent, desired, c)
	}
	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing router config : %s", err)
	}
	if !synced {
		return fmt.Errorf("Failed to sync router config")
	}
	return nil
}

func syncRouterConfig(agent *qdr.Agent, desired *qdr.RouterConfig, c *ConfigSync) (bool, error) {
	actual, err := agent.GetLocalConnectorStatus()
	if err != nil {
		return false, fmt.Errorf("Error retrieving local connector status: %s", err)
	}

	differences := qdr.ConnectorsDifference(actual, desired)
	if differences.Empty() {
		err = c.checkConnectorSecrets(actual, SHARED_TLS_DIRECTORY)
		if err != nil {
			return false, err
		}
		return true, nil
	} else {

		err := c.syncConnectorSecrets(differences, SHARED_TLS_DIRECTORY)
		if err != nil {
			return false, fmt.Errorf("error syncing secrets: %s", err)
		}

		if err = agent.UpdateConnectorConfig(differences); err != nil {
			return false, fmt.Errorf("Error syncing connectors: %s", err)
		}
		return false, nil
	}
}

func syncSecrets(configSync *ConfigSync, changes *qdr.BridgeConfigDifference, sharedTlsFilesDir string) error {
	for _, added := range changes.HttpListeners.Added {
		if len(added.SslProfile) > 0 {
			log.Printf("Copying cert files related to HTTP Connector sslProfile %s", added.SslProfile)
			return configSync.ensureSslProfile(added.SslProfile, added.SslProfile, sharedTlsFilesDir)
		}
	}

	for _, added := range changes.HttpConnectors.Added {
		if len(added.SslProfile) > 0 {
			log.Printf("Copying cert files related to HTTP Connector sslProfile %s", added.SslProfile)
			return configSync.ensureSslProfile(added.SslProfile, added.SslProfile, sharedTlsFilesDir)
		}
	}

	for _, deleted := range changes.HttpListeners.Deleted {
		if len(deleted.SslProfile) > 0 {
			log.Printf("Deleting cert files related to HTTP Listener sslProfile %s", deleted.SslProfile)

			agent, err := configSync.agentPool.Get()
			if err != nil {
				return err
			}

			if err = agent.Delete("org.apache.qpid.dispatch.sslProfile", deleted.SslProfile); err != nil {
				return fmt.Errorf("Error deleting ssl profile: #{err}")
			}

			err = os.RemoveAll(sharedTlsFilesDir + "/" + deleted.SslProfile)
			if err != nil {
				return err
			}

		}

	}

	for _, deleted := range changes.HttpConnectors.Deleted {
		if len(deleted.SslProfile) > 0 {
			log.Printf("Deleting cert files related to HTTP Connector sslProfile %s", deleted.SslProfile)
			err := os.RemoveAll(sharedTlsFilesDir + "/" + deleted.SslProfile)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *ConfigSync) checkSecrets(desired *qdr.BridgeConfig, sharedTlsFilesDir string) error {

	for _, listener := range desired.HttpListeners {
		if len(listener.SslProfile) > 0 {
			err := c.ensureSslProfile(listener.SslProfile, listener.SslProfile, sharedTlsFilesDir)
			if err != nil {
				return err
			}
		}
	}

	for _, connector := range desired.HttpConnectors {
		if len(connector.SslProfile) > 0 {
			err := c.ensureSslProfile(connector.SslProfile, connector.SslProfile, sharedTlsFilesDir)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *ConfigSync) checkConnectorSecrets(desired map[string]qdr.ConnectorStatus, sharedTlsFilesDir string) error {

	agent, err := c.agentPool.Get()
	if err != nil {
		return err
	}

	for _, connectorStatus := range desired {
		connector, err := agent.GetConnectorByName(connectorStatus.Name)
		if err != nil {
			return err
		}

		if len(connector.SslProfile) > 0 {
			secretName := strings.TrimSuffix(connector.SslProfile, "-profile")
			err := c.ensureSslProfile(connector.SslProfile, secretName, sharedTlsFilesDir)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *ConfigSync) syncConnectorSecrets(changes *qdr.ConnectorDifference, sharedTlsFilesDir string) error {

	agent, err := c.agentPool.Get()
	if err != nil {
		return err
	}

	for _, added := range changes.Added {
		if len(added.SslProfile) > 0 {
			sslProfile := changes.AddedSslProfiles[added.SslProfile]
			log.Printf("Creating ssl profile %s", sslProfile.Name)
			err := agent.CreateSslProfile(sslProfile)
			if err != nil {
				return err
			}

			log.Printf("Copying cert files related to Connector sslProfile %s", added.SslProfile)
			secretName := strings.TrimSuffix(added.SslProfile, "-profile")

			return c.ensureSslProfile(added.SslProfile, secretName, sharedTlsFilesDir)
		}
	}

	for _, deleted := range changes.Deleted {

		deletedConnector, err := agent.GetConnectorByName(deleted)
		if err != nil {
			return err
		}

		if len(deletedConnector.SslProfile) > 0 {

			log.Printf("Deleting cert files related to connector sslProfile %s", deletedConnector.SslProfile)

			if err = agent.Delete("org.apache.qpid.dispatch.sslProfile", deletedConnector.SslProfile); err != nil {
				return fmt.Errorf("Error deleting ssl profile: #{err}")
			}

			err = os.RemoveAll(sharedTlsFilesDir + "/" + deletedConnector.SslProfile)
			if err != nil {
				return err
			}

		}

	}

	return nil
}

func (c *ConfigSync) ensureSslProfile(sslProfile string, secretname string, sharedTlsFilesDir string) error {

	_, err := os.Stat(sharedTlsFilesDir + "/" + sslProfile)
	missingDir := os.IsNotExist(err)

	dirIsEmpty := false

	if !missingDir {
		dirIsEmpty, err = utils.IsDirEmpty(sharedTlsFilesDir + "/" + sslProfile)
		if err != nil {
			return err
		}
	}

	if missingDir || dirIsEmpty {
		log.Printf("Copying cert files related to HTTP Connector sslProfile %s", sslProfile)
		err := c.copyCertsFilesToPath(sharedTlsFilesDir, sslProfile, secretname)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ConfigSync) copyCertsFilesToPath(path string, profilename string, secretname string) error {
	secret, err := c.vanClient.KubeClient.CoreV1().Secrets(c.vanClient.Namespace).Get(secretname, metav1.GetOptions{})
	if err != nil {
		return err
	}

	err = os.Mkdir(path+"/"+profilename, 0777)
	if err != nil {
		return err
	}

	if secret.Data["tls.crt"] != nil {
		err = ioutil.WriteFile(path+"/"+profilename+"/tls.crt", secret.Data["tls.crt"], 0777)
		if err != nil {
			return err
		}
	}

	if secret.Data["tls.key"] != nil {
		err = ioutil.WriteFile(path+"/"+profilename+"/tls.key", secret.Data["tls.key"], 0777)
		if err != nil {
			return err
		}
	}

	if secret.Data["ca.crt"] != nil {
		err = ioutil.WriteFile(path+"/"+profilename+"/ca.crt", secret.Data["ca.crt"], 0777)
		if err != nil {
			return err
		}
	}

	return nil
}
