package main

import (
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
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

type CopyCerts func(string, string, string) error
type CreateSSlProfile func(profile qdr.SslProfile) error
type DeleteSslProfile func(string, string) error

const SHARED_TLS_DIRECTORY = "/etc/skupper-router-certs"

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
	err := c.checkCertFiles(SHARED_TLS_DIRECTORY)
	if err != nil {
		log.Printf("An error has ocurred when checking certification files for the router: %s", err)
	}

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
				err = c.checkCertFiles(SHARED_TLS_DIRECTORY)
				if err != nil {
					return fmt.Errorf("Error checking certificate files: %s", err)
				}

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
		return true, nil
	} else {
		differences.Print()

		configmap, err := kube.GetConfigMap(types.TransportConfigMapName, c.vanClient.Namespace, c.vanClient.GetKubeClient())
		if err != nil {
			return false, err
		}
		routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

		err = syncSecrets(routerConfig, differences, SHARED_TLS_DIRECTORY, c.copyCertsFilesToPath, agent.CreateSslProfile, agent.Delete)
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

	synced, err = syncConfig(agent, desired, c)

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

	err = syncRouterConfig(agent, desired, c)

	c.agentPool.Put(agent)
	if err != nil {
		return fmt.Errorf("Error while syncing router config : %s", err)
	}
	return nil
}

func syncRouterConfig(agent *qdr.Agent, desired *qdr.RouterConfig, c *ConfigSync) error {
	actual, err := agent.GetLocalConnectors()
	if err != nil {
		return fmt.Errorf("Error retrieving local connectors: %s", err)
	}

	ignorePrefix := "auto-mesh"
	differences := qdr.ConnectorsDifference(actual, desired, &ignorePrefix)

	if differences.Empty() {
		return nil
	} else {

		err := c.syncConnectorSecrets(differences, SHARED_TLS_DIRECTORY)
		if err != nil {
			return fmt.Errorf("error syncing secrets: %s", err)
		}

		if err = agent.UpdateConnectorConfig(differences); err != nil {
			return fmt.Errorf("Error syncing connectors: %s", err)
		}
		return nil
	}
}

func syncSecrets(routerConfig *qdr.RouterConfig, changes *qdr.BridgeConfigDifference, sharedPath string, copyCerts CopyCerts, newSSlProfile CreateSSlProfile, delSslProfile DeleteSslProfile) error {

	log.Printf("Sync profiles: Added %v  Deleted %v", changes.AddedSslProfiles, changes.DeletedSSlProfiles)

	for _, addedProfile := range changes.AddedSslProfiles {
		if len(addedProfile) > 0 {
			log.Printf("Copying cert files related to sslProfile %s", addedProfile)
			err := copyCerts(sharedPath, addedProfile, addedProfile)

			if err != nil {
				return err
			}

			log.Printf("Creating ssl profile %s", addedProfile)
			err = newSSlProfile(routerConfig.SslProfiles[addedProfile])
			if err != nil {
				return err
			}
		}
	}

	for _, deleted := range changes.DeletedSSlProfiles {
		if len(deleted) > 0 {

			log.Printf("Deleting cert files related to HTTP Listener sslProfile %s", deleted)

			if err := delSslProfile("io.skupper.router.sslProfile", deleted); err != nil {
				return fmt.Errorf("Error deleting ssl profile: #{err}")
			}

			err := os.RemoveAll(sharedPath + "/" + deleted)
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
			log.Printf("Synchronising secrets related to Connector %s", added.Name)
			secretName := strings.TrimSuffix(added.SslProfile, "-profile")
			err = c.copyCertsFilesToPath(sharedTlsFilesDir, added.SslProfile, secretName)
			if err != nil {
				return err
			}

			sslProfile := changes.AddedSslProfiles[added.SslProfile]
			log.Printf("Creating ssl profile %s", sslProfile.Name)
			err := agent.CreateSslProfile(sslProfile)
			if err != nil {
				return err
			}
		}
	}

	for _, deleted := range changes.Deleted {

		if len(deleted.SslProfile) > 0 {

			log.Printf("Deleting cert files related to connector sslProfile %s", deleted.SslProfile)

			if err = agent.Delete("io.skupper.router.sslProfile", deleted.SslProfile); err != nil {
				return fmt.Errorf("Error deleting ssl profile: #{err}")
			}

			err = os.RemoveAll(sharedTlsFilesDir + "/" + deleted.SslProfile)
			if err != nil {
				return err
			}

		}

	}

	return nil
}

func (c *ConfigSync) checkCertFiles(path string) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, c.vanClient.Namespace, c.vanClient.GetKubeClient())
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)

	for _, profile := range current.SslProfiles {
		secretName := profile.Name

		if strings.HasSuffix(profile.Name, "-profile") {
			secretName = strings.TrimSuffix(profile.Name, "-profile")
		}

		_, err = c.vanClient.GetKubeClient().CoreV1().Secrets(c.vanClient.Namespace).Get(secretName, metav1.GetOptions{})
		if err != nil {
			continue
		}

		err = c.copyCertsFilesToPath(path, profile.Name, secretName)
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

	_, err = os.Stat(path + "/" + profilename)

	if os.IsNotExist(err) {
		err = os.Mkdir(path+"/"+profilename, 0777)
		if err != nil {
			return err
		}
	}

	certFile := path + "/" + profilename + "/tls.crt"
	keyFile := path + "/" + profilename + "/tls.key"
	caCertFile := path + "/" + profilename + "/ca.crt"

	_, err = os.Stat(certFile)
	if secret.Data["tls.crt"] != nil && os.IsNotExist(err) {
		err = ioutil.WriteFile(certFile, secret.Data["tls.crt"], 0777)
		if err != nil {
			return err
		}
	}

	_, err = os.Stat(keyFile)
	if secret.Data["tls.key"] != nil && os.IsNotExist(err) {
		err = ioutil.WriteFile(keyFile, secret.Data["tls.key"], 0777)
		if err != nil {
			return err
		}
	}

	_, err = os.Stat(caCertFile)
	if secret.Data["ca.crt"] != nil && os.IsNotExist(err) {
		err = ioutil.WriteFile(caCertFile, secret.Data["ca.crt"], 0777)
		if err != nil {
			return err
		}
	}

	return nil
}
