package service_sync

import (
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/messaging"
)

const (
	ServiceSyncEvent string = "ServiceSyncEvent"
)

type UpdateHandler func(changed []types.ServiceInterface, deleted []string, origin string) error

type ServiceSync struct {
	origin            string
	version           string
	handler           UpdateHandler
	connectionFactory messaging.ConnectionFactory
	outgoing          chan ServiceUpdate
	incoming          chan ServiceUpdate
	updates           chan map[string]types.ServiceInterface
	byOrigin          map[string]map[string]types.ServiceInterface
	localServices     map[string]types.ServiceInterface
	byName            map[string]types.ServiceInterface
	heardFrom         map[string]time.Time
}

type ServiceUpdate struct {
	origin      string
	version     string
	definitions map[string]types.ServiceInterface
}

func NewServiceSync(origin string, version string, connectionFactory messaging.ConnectionFactory, handler UpdateHandler) *ServiceSync {
	s := &ServiceSync{
		origin:            origin,
		version:           version,
		handler:           handler,
		connectionFactory: connectionFactory,
		outgoing:          make(chan ServiceUpdate, 10),
		incoming:          make(chan ServiceUpdate, 10),
		updates:           make(chan map[string]types.ServiceInterface, 10),
		byOrigin:          map[string]map[string]types.ServiceInterface{},
		localServices:     map[string]types.ServiceInterface{},
		byName:            map[string]types.ServiceInterface{},
		heardFrom:         map[string]time.Time{},
	}
	return s
}

func (c *ServiceSync) LocalDefinitionsUpdated(definitions map[string]types.ServiceInterface) {
	c.updates <- definitions
}

func (c *ServiceSync) pareByOrigin(service string) {
	for _, origin := range c.byOrigin {
		if _, ok := origin[service]; ok {
			delete(origin, service)
			return
		}
	}
}

func (c *ServiceSync) recordChanges(definitions map[string]types.ServiceInterface) {
	var added []types.ServiceInterface
	var modified []types.ServiceInterface
	var removed []types.ServiceInterface

	for _, def := range c.byName {
		latest, ok := definitions[def.Address]
		if !ok {
			removed = append(removed, def)
		} else if !equivalentServiceDefinition(&def, &latest) {
			modified = append(modified, def)
		}
	}
	for _, def := range definitions {
		if _, ok := c.byName[def.Address]; !ok {
			added = append(added, def)
		}
	}
	if len(added) > 0 {
		event.Recordf(ServiceSyncEvent, "Service interface(s) added %s", strings.Join(getAddresses(added), ","))
	}
	if len(removed) > 0 {
		event.Recordf(ServiceSyncEvent, "Service interface(s) removed %s", strings.Join(getAddresses(removed), ","))
	}
	if len(modified) > 0 {
		event.Recordf(ServiceSyncEvent, "Service interface(s) modified %s", strings.Join(getAddresses(modified), ","))
	}
}

func (c *ServiceSync) localDefinitionsUpdated(definitions map[string]types.ServiceInterface) ServiceUpdate {
	latest := make(map[string]types.ServiceInterface) // becomes c.localServices
	byName := make(map[string]types.ServiceInterface)

	for name, original := range definitions {
		service := types.ServiceInterface{
			Address:                  original.Address,
			Protocol:                 original.Protocol,
			Ports:                    original.Ports,
			Origin:                   original.Origin,
			Headless:                 original.Headless,
			Labels:                   original.Labels,
			Aggregate:                original.Aggregate,
			EventChannel:             original.EventChannel,
			Targets:                  []types.ServiceInterfaceTarget{},
			EnableTls:                original.EnableTls,
			TlsCredentials:           original.TlsCredentials,
			PublishNotReadyAddresses: original.PublishNotReadyAddresses,
		}
		if !service.IsOfLocalOrigin() {
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

	c.recordChanges(definitions)
	c.localServices = latest
	c.byName = byName

	return ServiceUpdate{
		origin:      c.origin,
		version:     c.version,
		definitions: latest,
	}
}

func (c *ServiceSync) updateRemoteDefinitions(origin string, serviceInterfaceDefs map[string]types.ServiceInterface) {
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

	for _, name := range deleted {
		delete(c.byOrigin[origin], name)
	}

	err := c.handler(changed, deleted, origin)
	if err != nil {
		event.Record(ServiceSyncEvent, err.Error())
	}
}

func (c *ServiceSync) removeStaleDefinitions() {
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
					err := c.handler([]types.ServiceInterface{}, deleted, origin)
					if err != nil {
						event.Record(ServiceSyncEvent, err.Error())
					}
				}
			}
		}
	}

	for _, originName := range agedOrigins {
		event.Recordf(ServiceSyncEvent, "Service sync aged out service definitions from site %s", originName)
		delete(c.heardFrom, originName)
		delete(c.byOrigin, originName)
	}
}

func (c *ServiceSync) update(stopCh <-chan struct{}) {
	tickerAge := time.NewTicker(30 * time.Second)
	defer tickerAge.Stop()
	for {
		select {
		case update := <-c.incoming:
			if update.origin != c.origin {
				c.updateRemoteDefinitions(update.origin, update.definitions)
			}

		case latest := <-c.updates:
			update := c.localDefinitionsUpdated(latest)
			c.outgoing <- update

		case <-tickerAge.C:
			c.removeStaleDefinitions()

		case <-stopCh:
			return
		}
	}
}

func (c *ServiceSync) Start(stopCh <-chan struct{}) {
	go c.run(stopCh)
}

func (c *ServiceSync) run(stopCh <-chan struct{}) {
	s := newSender(c.connectionFactory, c.outgoing)
	s.start()
	r := newReceiver(c.connectionFactory, c.incoming)
	r.start()
	c.update(stopCh)
	r.stop()
	s.stop()
	event.Record(ServiceSyncEvent, "Service sync stopped")
}

func getAddresses(services []types.ServiceInterface) []string {
	addresses := []string{}
	for _, service := range services {
		addresses = append(addresses, service.Address)
	}
	return addresses
}

func equivalentServiceDefinition(a *types.ServiceInterface, b *types.ServiceInterface) bool {
	if a.Protocol != b.Protocol || !reflect.DeepEqual(a.Ports, b.Ports) || a.EventChannel != b.EventChannel || a.Aggregate != b.Aggregate || !reflect.DeepEqual(a.Labels, b.Labels) {
		return false
	}
	if a.Headless == nil && b.Headless == nil {
		return true
	} else if a.Headless != nil && b.Headless != nil {
		return reflect.DeepEqual(a.Headless, b.Headless)
	} else {
		return false
	}
}
