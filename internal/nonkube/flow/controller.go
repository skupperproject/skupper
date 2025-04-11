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
)

type ControllerConfig struct {
	Factory session.ContainerFactory
	Site    vanflow.SiteRecord
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
	manager.SetLoggerArgs(map[string]interface{}{
		"namespace": cfg.Site.Namespace,
	})

	ctrlr := &Controller{
		container: container,
		source:    source,
		manager:   manager,
		logger: slog.New(slog.Default().Handler()).With(
			slog.String("namespace", *cfg.Site.Namespace),
			slog.String("component", "nonkube.flow.controller"),
		),
	}
	return ctrlr
}

type Controller struct {
	container session.Container
	source    store.SourceRef
	manager   *eventsource.Manager
	logger    *slog.Logger
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
	<-ctx.Done()
}
