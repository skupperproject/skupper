package podman

import (
	"log"
	"time"

	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	resyncPeriod = time.Second * 5
)

//
// Base types
//

type informerBase struct {
	cli          *PodmanRestClient
	resyncPeriod time.Duration
}

func (i *informerBase) SetResyncPeriod(t time.Duration) {
	i.resyncPeriod = t
}

type updateNotification[T any] struct {
	oldObj T
	newObj T
}

type EventHandler[T any] interface {
	OnAdd(obj T)
	OnUpdate(oldObj, newObj T)
	OnDelete(obj T)
}

type EventHandlerBase[T any] struct {
	Add    func(obj T)
	Update func(oldObj, newObj T)
	Delete func(obj T)
}

func (e *EventHandlerBase[T]) OnAdd(obj T) {
	e.Add(obj)
}

func (e *EventHandlerBase[T]) OnUpdate(oldObj, newObj T) {
	e.Update(oldObj, newObj)
}

func (e *EventHandlerBase[T]) OnDelete(obj T) {
	e.Delete(obj)
}

//
// Container informer
//

func NewContainerInformer(cli *PodmanRestClient) *ContainerInformer {
	return &ContainerInformer{
		informerBase: informerBase{
			cli:          cli,
			resyncPeriod: resyncPeriod,
		},
		containers:    map[string]*container.Container{},
		eventHandlers: []EventHandler[*container.Container]{},
	}
}

type ContainerInformer struct {
	informerBase
	containers    map[string]*container.Container
	eventHandlers []EventHandler[*container.Container]
}

func (c *ContainerInformer) AddEventHandler(e EventHandler[*container.Container]) {
	c.eventHandlers = append(c.eventHandlers, e)
}

func (c *ContainerInformer) Start(stopCh chan struct{}) {
	go wait.Until(c.run, c.resyncPeriod, stopCh)
}

func (c *ContainerInformer) run() {
	cl, err := c.cli.ContainerList()
	var clNames []string
	if err != nil {
		log.Printf("unable to retrieve container list - %s", err)
		return
	}

	var added []*container.Container
	var updated []updateNotification[*container.Container]
	var deleted []*container.Container

	// Verifying new and updated containers
	for _, ci := range cl {
		clNames = append(clNames, ci.Name)
		if co, ok := c.containers[ci.Name]; !ok {
			cn, err := c.cli.ContainerInspect(ci.Name)
			if err != nil {
				log.Printf("error inspecting container: %s - %s", ci.Name, err)
				continue
			}
			c.containers[ci.Name] = cn
			added = append(added, cn)
		} else if co.ID != ci.ID || ci.StartedAt.After(co.StartedAt) {
			cn, err := c.cli.ContainerInspect(ci.Name)
			if err != nil {
				log.Printf("error inspecting container: %s - %s", ci.Name, err)
				continue
			}
			updated = append(updated, updateNotification[*container.Container]{
				oldObj: co,
				newObj: cn,
			})
			c.containers[ci.Name] = cn
		}
	}

	// Verifying deleted containers
	for ciName, ci := range c.containers {
		if !utils.StringSliceContains(clNames, ciName) {
			deleted = append(deleted, ci)
			delete(c.containers, ciName)
		}
	}

	// Notifying event handlers
	for _, ci := range added {
		for _, e := range c.eventHandlers {
			e.OnAdd(ci)
		}
	}
	for _, ci := range updated {
		for _, e := range c.eventHandlers {
			e.OnUpdate(ci.oldObj, ci.newObj)
		}
	}
	for _, ci := range deleted {
		for _, e := range c.eventHandlers {
			e.OnDelete(ci)
		}
	}
}
