package podman

import (
	"log"
	"time"

	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/utils"
	"k8s.io/apimachinery/pkg/util/wait"
)

const (
	resyncPeriod = time.Second * 5
)

//
// Base types
//

type informerCommon struct {
	cli          *PodmanRestClient
	resyncPeriod time.Duration
}

func (i *informerCommon) SetResyncPeriod(t time.Duration) {
	i.resyncPeriod = t
}

type updateNotification[T any] struct {
	oldObj T
	newObj T
}

//
// Container informer
//

func NewContainerInformer(cli *PodmanRestClient) *ContainerInformer {
	return &ContainerInformer{
		informerCommon: informerCommon{
			cli:          cli,
			resyncPeriod: resyncPeriod,
		},
		containers: map[string]*container.Container{},
		informers:  []container.Informer[*container.Container]{},
	}
}

type ContainerInformer struct {
	informerCommon
	containers map[string]*container.Container
	informers  []container.Informer[*container.Container]
}

func (c *ContainerInformer) AddInformer(i container.Informer[*container.Container]) {
	c.informers = append(c.informers, i)
}

func (c *ContainerInformer) Start(stopCh <-chan struct{}) {
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
			ci.Running = false
			ci.ExitedAt = time.Now()
			deleted = append(deleted, ci)
			delete(c.containers, ciName)
		}
	}

	// Notifying informers
	for _, ci := range added {
		for _, i := range c.informers {
			i.OnAdd(ci)
		}
	}
	for _, ci := range updated {
		for _, i := range c.informers {
			i.OnUpdate(ci.oldObj, ci.newObj)
		}
	}
	for _, ci := range deleted {
		for _, i := range c.informers {
			i.OnDelete(ci)
		}
	}
}
