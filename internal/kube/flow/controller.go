package flow

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/skupperproject/skupper/pkg/vanflow"
	"github.com/skupperproject/skupper/pkg/vanflow/eventsource"
	"github.com/skupperproject/skupper/pkg/vanflow/session"
	"github.com/skupperproject/skupper/pkg/vanflow/store"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type ControllerConfig struct {
	Factory  session.ContainerFactory
	Site     vanflow.SiteRecord
	Informer cache.SharedIndexInformer
}

func NewController(cfg ControllerConfig) *Controller {
	source := store.SourceRef{
		ID:      cfg.Site.ID,
		Version: "1",
	}
	staticRecords := store.NewSyncMapStore(store.SyncMapStoreConfig{})
	staticRecords.Add(cfg.Site, source)

	pods := store.NewSyncMapStore(store.SyncMapStoreConfig{})

	container := cfg.Factory.Create()
	manager := eventsource.NewManager(container, eventsource.ManagerConfig{
		Source: eventsource.Info{
			ID:      cfg.Site.ID,
			Version: 1,
			Type:    "CONTROLLER",
			Address: fmt.Sprintf("mc/sfe.%s", cfg.Site.ID),
			Direct:  fmt.Sprintf("sfe.%s", cfg.Site.ID),
		},
		Stores: []store.Interface{staticRecords, pods},

		UseAlternateHeartbeatAddress: true,
		FlushDelay:                   time.Millisecond * 100,
		FlushBatchSize:               20,
		UpdateBufferTime:             time.Millisecond * 1000,
		UpdateBatchSize:              10,
	})

	ctrlr := &Controller{
		podStore:  pods,
		podCache:  cfg.Informer.GetStore(),
		container: container,
		source:    source,
		manager:   manager,
		queue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "kube-flow-controller"),
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.flow.controller"),
		),
	}

	cfg.Informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj any) {
			evnt, err := keyFunc(obj)
			if err != nil {
				ctrlr.logger.Error("add event error", slog.Any("error", err))
			}
			evnt.Type = eventTypeProcess
			ctrlr.queue.Add(evnt)
		},
		UpdateFunc: func(prev, obj any) {
			evnt, err := keyFunc(obj)
			if err != nil {
				ctrlr.logger.Error("update event error", slog.Any("error", err))
			}
			evnt.Type = eventTypeProcess
			ctrlr.queue.Add(evnt)
		},
		DeleteFunc: func(obj any) {
			if final, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = final.Obj
			}
			evnt, err := keyFunc(obj)
			if err != nil {
				ctrlr.logger.Error("update event error", slog.Any("error", err))
			}
			evnt.Type = eventTypeProcess
			ctrlr.queue.Add(evnt)
		},
	})
	return ctrlr
}

type Controller struct {
	container session.Container
	podStore  store.Interface
	podCache  cache.Store
	source    store.SourceRef
	manager   *eventsource.Manager
	queue     workqueue.RateLimitingInterface
	logger    *slog.Logger
}

func (c *Controller) handlePodEvent(e workEvent) error {
	obj, exists, err := c.podCache.GetByKey(e.CacheKey)
	if err != nil {
		return err
	}
	var record vanflow.ProcessRecord
	record.ID = e.StoreKey
	if exists {
		record = asProcessRecord(obj.(*v1.Pod))
	}
	c.updateProcess(!exists, record)
	return nil
}

func (c *Controller) Run(ctx context.Context) {
	c.container.Start(ctx)
	mgmtCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.container.OnSessionError(func(err error) {
		_, retryable := err.(session.RetryableError)
		if !retryable {
			cancel()
		}
		c.logger.Error("amqp session error", slog.Any("error", err), slog.Bool("retryable", retryable))
	})
	go c.manager.Run(mgmtCtx)
	// handle shutdown though work queue
	go func() {
		<-mgmtCtx.Done()
		c.queue.ShutDown()
	}()
	for {
		item, done := c.queue.Get()
		if done {
			c.logger.Info("workqueue shutdown")
			return
		}
		evnt := item.(workEvent)
		switch evnt.Type {
		case eventTypeProcess:
			if err := c.handlePodEvent(evnt); err != nil {
				c.queue.AddRateLimited(evnt)
			}
		}
		c.queue.Forget(item)
		c.queue.Done(item)
	}
}

func (c *Controller) updateProcess(deleted bool, process vanflow.ProcessRecord) {
	c.logger.Debug("update process", slog.Bool("deleted", deleted), slog.String("process", process.ID))
	process.Parent = &c.source.ID
	if deleted {
		entry, ok := c.podStore.Delete(process.ID)
		if !ok {
			c.logger.Debug("ignoring notification for unknown pod", slog.String("process", process.ID))
			return
		}
		terminalRecord := entry.Record.(vanflow.ProcessRecord)
		terminalRecord.EndTime = &vanflow.Time{Time: time.Now()}
		c.manager.PublishUpdate(eventsource.RecordUpdate{
			Prev: entry.Record,
			Curr: terminalRecord,
		})
		return
	}
	var prev vanflow.Record
	if curr, exists := c.podStore.Get(process.ID); exists {
		c.podStore.Update(process)
		prev = curr.Record
	} else {
		c.podStore.Add(process, c.source)
	}
	c.manager.PublishUpdate(eventsource.RecordUpdate{
		Prev: prev,
		Curr: process,
	})
}

type workEvent struct {
	CacheKey string
	StoreKey string
	Type     eventType
}

type eventType string

const (
	eventTypeProcess eventType = "ProcessRecord"
)

func keyFunc(obj any) (workEvent, error) {
	var (
		out workEvent
		err error
	)
	out.CacheKey, err = cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return out, fmt.Errorf("failed to get object key: %v", err)
	}
	ma, ok := obj.(metav1.ObjectMetaAccessor)
	if !ok {
		return out, fmt.Errorf("failed to get object metadata for type %T", obj)
	}
	meta := ma.GetObjectMeta()
	out.StoreKey = string(meta.GetUID())

	return out, nil
}
