package main

import (
	"context"
	jsonencoding "encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	amqp "github.com/interconnectedcloud/go-amqp"
	"github.com/skupperproject/skupper/pkg/utils"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
)

const (
	ServiceSyncServiceEvent string = "ServiceSyncServiceEvent"
	ServiceSyncSiteEvent    string = "ServiceSyncSiteEvent"
	ServiceSyncConnection   string = "ServiceSyncConnection"
	ServiceSyncError        string = "ServiceSyncError"
	serviceSyncSubjectV1    string = "service-sync-update"
	serviceSyncSubjectV2    string = "service-sync-update-v2"
)

func (c *Controller) pareByOrigin(service string) {
	for _, origin := range c.byOrigin {
		if _, ok := origin[service]; ok {
			delete(origin, service)
			return
		}
	}
}

func getAddresses(services []types.ServiceInterface) []string {
	addresses := []string{}
	for _, service := range services {
		addresses = append(addresses, service.Address)
	}
	return addresses
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
			Ports:        original.Ports,
			Origin:       original.Origin,
			Headless:     original.Headless,
			Labels:       original.Labels,
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
		event.Recordf(ServiceSyncServiceEvent, "Service interface(s) added %s", strings.Join(getAddresses(added), ","))
	}
	if len(removed) > 0 {
		event.Recordf(ServiceSyncServiceEvent, "Service interface(s) removed %s", strings.Join(getAddresses(removed), ","))
	}
	if len(modified) > 0 {
		event.Recordf(ServiceSyncServiceEvent, "Service interface(s) modified %s", strings.Join(getAddresses(modified), ","))
	}

	c.localServices = latest
	c.byName = byName
}

func equivalentServiceDefinition(a *types.ServiceInterface, b *types.ServiceInterface) bool {
	if a.Protocol != b.Protocol || !reflect.DeepEqual(a.Ports, b.Ports) || a.EventChannel != b.EventChannel || a.Aggregate != b.Aggregate || !reflect.DeepEqual(a.Labels, b.Labels) {
		return false
	}
	if a.Headless == nil && b.Headless == nil {
		return true
	} else if a.Headless != nil && b.Headless != nil {
		if a.Headless.Name != b.Headless.Name || a.Headless.Size != b.Headless.Size || !reflect.DeepEqual(a.Headless.TargetPorts, b.Headless.TargetPorts) {
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
		event.Recordf(ServiceSyncError, "Failed to create sender: %s", err.Error())
	}

	defer func() {
		sender.Close(ctx)
	}()

	tickerSend := time.NewTicker(5 * time.Second)
	tickerAge := time.NewTicker(30 * time.Second)

	properties.Subject = serviceSyncSubjectV2
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
				event.Recordf(ServiceSyncError, "Failed to create json for service definition sync: %s", err.Error())
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
				event.Recordf(ServiceSyncSiteEvent, "Service sync aged out service definitions from origin %s", originName)
				delete(c.heardFrom, originName)
				delete(c.byOrigin, originName)
			}
		}
	}
}

func (c *Controller) runServiceSync() {
	ctx := context.Background()

	event.Recordf(ServiceSyncConnection, "Establishing connection to %s service for service sync", types.LocalTransportServiceName)

	client, err := amqp.Dial("amqps://"+types.LocalTransportServiceName+":5671", amqp.ConnSASLExternal(), amqp.ConnMaxFrameSize(4294967295), amqp.ConnTLSConfig(c.tlsConfig))
	if err != nil {
		event.Recordf(ServiceSyncConnection, "Failed to create amqp connection %s", err.Error())
		utilruntime.HandleError(fmt.Errorf("Failed to create amqp connection %s", err.Error()))
		return
	}
	event.Recordf(ServiceSyncConnection, "Service sync connection to %s service established", types.LocalTransportServiceName)
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

	allowedSubjects := []string{
		serviceSyncSubjectV2,
	}

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

		// Validate if message is supported
		if utils.StringSliceContains(allowedSubjects, subject) {
			if origin, ok = msg.ApplicationProperties["origin"].(string); ok {
				if origin != c.origin {
					if updates, ok := msg.Value.(string); ok {
						defs := &types.ServiceInterfaceList{}
						err := defs.ConvertFrom(updates)
						if err == nil {
							indexed := make(map[string]types.ServiceInterface)
							for _, def := range *defs {
								def.Origin = origin
								indexed[def.Address] = def
							}
							c.ensureServiceInterfaceDefinitions(origin, indexed)
						} else {
							event.Recordf(ServiceSyncError, "Skupper service sync update from %s was not valid json: %s", origin, err)
						}
					} else {
						event.Recordf(ServiceSyncError, "Skupper service sync update from %s was not a string", origin)
					}
				}
			} else {
				event.Record(ServiceSyncError, "Skupper service sync update type assertion error")
			}
		} else {
			event.Record(ServiceSyncError, "Service sync subject not valid")
		}
	}
}
