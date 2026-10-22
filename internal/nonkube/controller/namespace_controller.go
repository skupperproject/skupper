package controller

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/filesystem"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type NamespaceController struct {
	ns               string
	stopCh           chan struct{}
	logger           *slog.Logger
	watcher          *filesystem.FileWatcher
	prepare          func()
	pathProvider     fs.PathProvider
	checkRouterDrift func(namespace string) (bool, error)
	bootstrap        func(config *bootstrap.Config) (*api.SiteState, error)
	postBootstrap    func(config *bootstrap.Config, siteState *api.SiteState)
}

func NewNamespaceController(namespace string) (*NamespaceController, error) {
	nsw := &NamespaceController{
		ns:     namespace,
		stopCh: make(chan struct{}),
		pathProvider: fs.PathProvider{
			Namespace: namespace,
		},
		checkRouterDrift: bootstrap.CheckRouterImageDrift,
		bootstrap:        bootstrap.Bootstrap,
		postBootstrap:    bootstrap.PostBootstrap,
	}
	watcher, err := filesystem.NewWatcher(slog.String("namespace", namespace))
	if err != nil {
		return nil, fmt.Errorf("error creating watcher on namespace %s: %v", namespace, err)
	}
	nsw.watcher = watcher
	nsw.logger = slog.New(slog.Default().Handler()).
		With("component", "namespace.watcher").
		With("namespace", namespace)
	return nsw, nil
}

func (w *NamespaceController) Start() {
	if w.prepare == nil {
		w.upgradeDriftedRouter()

		routerConfigHandler := NewRouterConfigHandler(w.stopCh, w.ns)
		routerStateHandler := NewRouterStateHandler(w.ns)
		routerConfigHandler.AddCallback(routerStateHandler)
		collectorLifecycleHandler := NewCollectorLifecycleHandler(w.ns)
		routerStateHandler.AddCallback(collectorLifecycleHandler)
		inputResourceHandler := NewInputResourceHandler(w.stopCh, w.ns, w.pathProvider.GetNamespace(), w.bootstrap, w.postBootstrap, bootstrap.Teardown)
		systemAdaptorHandler := NewSystemAdaptorHandler(w.ns)
		if systemAdaptorHandler != nil {
			routerStateHandler.AddCallback(systemAdaptorHandler)
		}

		w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RouterConfigPath), routerConfigHandler)
		w.watcher.Add(api.GetInternalOutputPath(w.ns, api.RuntimeSiteStatePath), NewNetworkStatusHandler(w.ns))

		if inputResourceHandler != nil {
			w.watcher.Add(w.pathProvider.GetNamespace(), inputResourceHandler)
		}
	} else {
		w.prepare()
	}
	w.watcher.Start(w.stopCh)
	go w.run()
}

func (w *NamespaceController) upgradeDriftedRouter() {
	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)
	if systemReloadType != types.SystemReloadTypeAuto {
		return
	}

	sourcesPath := api.GetInternalOutputPath(w.ns, api.InputSiteStatePath)
	if _, err := os.Stat(sourcesPath); err != nil {
		return
	}

	if w.checkRouterDrift == nil || w.bootstrap == nil {
		return
	}

	drifted, err := w.checkRouterDrift(w.ns)
	if err != nil {
		w.logger.Debug("failed to check router image drift", slog.Any("error", err))
		return
	}
	if !drifted {
		return
	}

	w.logger.Info("Router image drift detected, re-bootstrapping router", slog.String("namespace", w.ns))
	platformLoader := common.NamespacePlatformLoader{}
	nsPlatform, err := platformLoader.Load(w.ns)
	if err != nil {
		w.logger.Error("failed to load platform for namespace", slog.Any("error", err))
		return
	}

	//bootstrap code takes care of backing up the namespace and restore it if necessary
	cfg := &bootstrap.Config{
		Namespace: w.ns,
		Platform:  types.Platform(nsPlatform),
		InputPath: sourcesPath,
	}
	siteState, err := w.bootstrap(cfg)
	if err != nil {
		w.logger.Error("failed to re-bootstrap drifted router", slog.Any("error", err))
		return
	}
	if w.postBootstrap != nil {
		w.postBootstrap(cfg, siteState)
	}
	w.logger.Info("Successfully upgraded drifted router", slog.String("namespace", w.ns))
}

func (w *NamespaceController) run() {
	<-w.stopCh
	w.logger.Info("stopped namespace watcher")
	return
}

func (w *NamespaceController) Stop() {
	close(w.stopCh)
}
