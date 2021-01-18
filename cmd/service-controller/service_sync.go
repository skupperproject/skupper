package main

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"log"
	"reflect"
	"time"

	amqp "github.com/interconnectedcloud/go-amqp"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
)

func (c *Controller) pareByOrigin(service string) {
	for _, origin := range c.byOrigin {
		if _, ok := origin[service]; ok {
			delete(origin, service)
			return
		}
	}
}

func (c *Controller) serviceSyncDefinitionsUpdated(definitions map[string]types.ServiceInterface) {
	latest := make(map[string]types.ServiceInterface) // becomes c.localServices
	byName := make(map[string]types.ServiceInterface)
	var added []types.ServiceInterface
	var modified []types.ServiceInterface
	var removed []types.ServiceInterface

	for name, original := range definitions {
		service := types.ServiceInterface{
			Address:      original.Address,
			Protocol:     original.Protocol,
			Port:         original.Port,
			Origin:       original.Origin,
			Headless:     original.Headless,
			Aggregate:    original.Aggregate,
			EventChannel: original.EventChannel,
			Targets:      []types.ServiceInterfaceTarget{},
		}
		if service.Origin != "" && service.Origin != "annotation" {
			if _, ok := c.byOrigin[service.Origin]; !ok {
				c.byOrigin[service.Origin] = make(map[string]types.ServiceInterface)
			}
			c.byOrigin[service.Origin][name] = service
		} else {
			latest[service.Address] = service
			// may have previously been tracked by origin
			c.pareByOrigin(service.Address)
		}
		byName[service.Address] = service
	}

	for _, def := range c.localServices {
		if _, ok := latest[def.Address]; !ok {
			removed = append(removed, def)
		} else if !reflect.DeepEqual(def, latest[def.Address]) {
			modified = append(modified, def)
		}
	}
	for _, def := range latest {
		if _, ok := c.localServices[def.Address]; !ok {
			added = append(added, def)
		}
	}

	// TODO: detect and describe changes
	if len(added) > 0 {
		log.Println("Service interface(s) added", added)
	}
	if len(removed) > 0 {
		log.Println("Service interface(s) removed", removed)
	}
	if len(modified) > 0 {
		log.Println("Service interface(s) modified", modified)
	}

	c.localServices = latest
	c.byName = byName
}

func equivalentServiceDefinition(a *types.ServiceInterface, b *types.ServiceInterface) bool {
	if a.Protocol != b.Protocol || a.Port != b.Port || a.EventChannel != b.EventChannel || a.Aggregate != b.Aggregate {
		return false
	}
	if a.Headless == nil && b.Headless == nil {
		return true
	} else if a.Headless != nil && b.Headless != nil {
		if a.Headless.Name != b.Headless.Name || a.Headless.Size != b.Headless.Size || a.Headless.TargetPort != b.Headless.TargetPort {
			return false
		} else {
			return true
		}
	} else {
		return false
	}
}

func (c *Controller) ensureServiceInterfaceDefinitions(origin string, serviceInterfaceDefs map[string]types.ServiceInterface) {
	var changed []types.ServiceInterface
	var deleted []string

	c.heardFrom[origin] = time.Now()

	for _, def := range serviceInterfaceDefs {
		existing, ok := c.byName[def.Address]
		if !ok || (existing.Origin == origin && !equivalentServiceDefinition(&def, &existing)) {
			changed = append(changed, def)
		}
	}

	if _, ok := c.byOrigin[origin]; !ok {
		c.byOrigin[origin] = make(map[string]types.ServiceInterface)
	} else {
		current := c.byOrigin[origin]
		for name, _ := range current {
			if _, ok := serviceInterfaceDefs[name]; !ok {
				deleted = append(deleted, name)
			}
		}
	}

	kube.UpdateSkupperServices(changed, deleted, origin, c.vanClient.Namespace, c.vanClient.KubeClient)

	for _, name := range deleted {
		delete(c.byOrigin[origin], name)
	}
}

func (c *Controller) syncSender(sendLocal chan bool) {
	var request amqp.Message
	var properties amqp.MessageProperties

	ctx := context.Background()
	sender, err := c.amqpSession.NewSender(amqp.LinkTargetAddress(types.ServiceSyncAddress))
	if err != nil {
		log.Fatal("Failed to create sender: ", err.Error())
	}

	defer func() {
		sender.Close(ctx)
	}()

	tickerSend := time.NewTicker(5 * time.Second)
	tickerAge := time.NewTicker(30 * time.Second)

	properties.Subject = "service-sync-update"
	request.Properties = &properties
	request.ApplicationProperties = make(map[string]interface{})
	request.ApplicationProperties["origin"] = c.origin
	request.ApplicationProperties["version"] = client.Version

	for {
		select {
		case <-tickerSend.C:
			local := make([]types.ServiceInterface, 0)

			for _, si := range c.localServices {
				local = append(local, si)
			}

			encoded, err := jsonencoding.Marshal(local)
			if err != nil {
				log.Println("Failed to create json for service definition sync: ", err.Error())
				return
			}
			request.Value = string(encoded)
			err = sender.Send(ctx, &request)

		case <-tickerAge.C:
			var agedOrigins []string

			now := time.Now()

			for origin, _ := range c.byOrigin {
				var deleted []string

				if lastHeard, ok := c.heardFrom[origin]; ok {
					if now.Sub(lastHeard) >= 60*time.Second {
						agedOrigins = append(agedOrigins, origin)
						agedDefinitions := c.byOrigin[origin]
						for name, _ := range agedDefinitions {
							deleted = append(deleted, name)
						}
						if len(deleted) > 0 {
							kube.UpdateSkupperServices([]types.ServiceInterface{}, deleted, origin, c.vanClient.Namespace, c.vanClient.KubeClient)
						}
					}
				}
			}

			for _, originName := range agedOrigins {
				log.Println("Service sync aged out service definitions from origin ", originName)
				delete(c.heardFrom, originName)
				delete(c.byOrigin, originName)
			}
		}
	}
}

func (c *Controller) runServiceSync() {
	ctx := context.Background()

	log.Println("Establishing connection to skupper-messaging service for service sync")

	client, err := amqp.Dial("amqps://skupper-messaging:5671", amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnTLSConfig(c.tlsConfig))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp connection %s", err.Error()))
		return
	}
	log.Println("Service sync connection to skupper-messaging service established")
	c.amqpClient = client
	defer c.amqpClient.Close()

	c.amqpSession, err = c.amqpClient.NewSession()
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp session %s", err.Error()))
		return
	}

	receiver, err := c.amqpSession.NewReceiver(
		amqp.LinkSourceAddress(types.ServiceSyncAddress),
		amqp.LinkCredit(10),
	)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp receiver %s", err.Error()))
		return
	}
	defer func() {
		ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		receiver.Close(ctx)
		cancel()
	}()

	sendLocal := make(chan bool)
	go c.syncSender(sendLocal)

	for {
		var ok bool
		var origin string
		msg, err := receiver.Receive(ctx)
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("Failed reading message from service sync %s", err.Error()))
			return
		}
		// Decode message as it is either a request to send update
		// or it is a receipt that needs to be reconciled
		msg.Accept()
		subject := msg.Properties.Subject

		if subject == "service-sync-request" {
			//sendLocal <- true
			log.Println("Controller received service sync request")
		} else if subject == "service-sync-update" {
			if origin, ok = msg.ApplicationProperties["origin"].(string); ok {
				if origin != c.origin {
					if updates, ok := msg.Value.(string); ok {
						defs := []types.ServiceInterface{}
						err := jsonencoding.Unmarshal([]byte(updates), &defs)
						if err == nil {
							indexed := make(map[string]types.ServiceInterface)
							for _, def := range defs {
								def.Origin = origin
								indexed[def.Address] = def
							}
							c.ensureServiceInterfaceDefinitions(origin, indexed)
						} else {
							log.Printf("Skupper service sync update from %s was not valid json: %s", origin, err)
						}
					} else {
						log.Printf("Skupper service sync update from %s was not a string", origin)
					}
				}
			} else {
				log.Println("Skupper service sync update type assertion error")
			}
		} else {
			log.Println("Service sync subject not valid")
		}
	}
}
