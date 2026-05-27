package controller

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/skupperproject/skupper/api/types"
	cmd "github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/nonkube/compat"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

// This feature is responsible for handling the creation of input resources and
// execute the start/reload of the site configuration automatically.
type InputResourceHandler struct {
	stopCh            chan struct{}
	logger            *slog.Logger
	namespace         string
	inputPath         string
	Bootstrap         func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec          func(config *bootstrap.Config, siteState *api.SiteState)
	TearDown          func(namespace string) error
	ConfigBootstrap   bootstrap.Config
	lock              sync.Mutex
	siteStateRenderer *compat.SiteStateRenderer
	siteStateLoader   api.SiteStateLoader
	siteHandler       *fs.SiteHandler
	deduplicator      *EventDeduplicator
}

type Bootstrap func(config *bootstrap.Config) (*api.SiteState, error)
type PostBootstrap func(config *bootstrap.Config, siteState *api.SiteState)
type TearDown func(namespace string) error

func NewInputResourceHandler(stopCh chan struct{}, namespace string, inputPath string, bs Bootstrap, pbs PostBootstrap, td TearDown) *InputResourceHandler {

	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)

	if systemReloadType == types.SystemReloadTypeManual {
		slog.Default().Debug("Automatic reloading is not configured.")
		return nil
	}

	handler := &InputResourceHandler{
		namespace: namespace,
		inputPath: inputPath,
		stopCh:    stopCh,
	}

	handler.logger = slog.Default().With("component", "input.resource.handler", "namespace", namespace)

	handler.Bootstrap = bs
	handler.PostExec = pbs
	handler.TearDown = td

	var binary string

	platform := types.Platform(utils.DefaultStr(os.Getenv("CONTAINER_ENGINE"),
		string(types.PlatformPodman)))

	// TODO: add support for linux platform
	switch cmd.Platform(platform) {
	case cmd.PlatformDocker:
		binary = "docker"
	case cmd.PlatformPodman:
		binary = "podman"
	case cmd.PlatformLinux:
		handler.logger.Error("Linux platform is not supported yet")
		return nil
	default:
		handler.logger.Error("This platform value is not supported: ", slog.String("platform", string(platform)))
		return nil
	}

	handler.ConfigBootstrap = bootstrap.Config{
		Namespace: namespace,
		InputPath: inputPath,
		Platform:  platform,
		Binary:    binary,
	}

	handler.siteStateRenderer = &compat.SiteStateRenderer{
		Platform: platform,
	}

	handler.siteStateLoader = &common.FileSystemSiteStateLoader{
		Path:   api.GetInternalOutputPath(namespace, api.InputSiteStatePath),
		Bundle: false,
	}

	handler.siteHandler = fs.NewSiteHandler(namespace)

	handler.deduplicator = NewEventDeduplicator(
		stopCh,
		func(filename string) {
			handler.lock.Lock()
			defer handler.lock.Unlock()

			err := handler.processInputFile()
			if err != nil {
				handler.logger.Error(err.Error())
			}
		},
		handler.logger,
	)

	return handler
}

func (h *InputResourceHandler) OnCreate(name string) {
	h.logger.Info(fmt.Sprintf("Resource has been created: %s", name))
	h.deduplicator.QueueEvent(name)
}

func (h *InputResourceHandler) OnUpdate(name string) {
	h.logger.Info(fmt.Sprintf("Resource has been updated: %s", name))
	h.deduplicator.QueueEvent(name)
}
func (h *InputResourceHandler) OnRemove(name string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.logger.Info(fmt.Sprintf("Resource has been deleted: %s", name))

	sites, err := h.siteHandler.List(fs.GetOptions{InputOnly: true})
	if err != nil {
		h.logger.Error(err.Error())
	}

	//If there is no site configured or running, the namespace needs to be removed
	if err != nil || len(sites) == 0 {
		err = h.tearDownNamespace()
		if err != nil {
			h.logger.Error(err.Error())
		}
		return
	}

	err = h.processInputFile()
	if err != nil {
		h.logger.Error(err.Error())
	}
}
func (h *InputResourceHandler) Filter(name string) bool {

	filename := filepath.Base(name)

	filenameValidator := validator.NewInputResourceFilenameValidator()
	valid, err := filenameValidator.Evaluate(filename)

	if !valid {
		h.logger.Warn("File does not follow the required pattern {ResourceType}-name.yaml",
			slog.String("file", filename),
			slog.String("error", err.Error()))
	}

	return valid
}

func (h *InputResourceHandler) OnBasePathAdded(basePath string) {}

func (h *InputResourceHandler) processInputFile() error {
	var siteState *api.SiteState
	var inputSiteNames []string
	var runtimeSiteNames []string

	inputSites, err := h.siteHandler.List(fs.GetOptions{InputOnly: true})
	if err != nil {
		h.logger.Debug("Trying to list input sites:", slog.Any("error", err))
	}
	for _, site := range inputSites {
		inputSiteNames = append(inputSiteNames, site.Name)
	}

	runtimeSites, err := h.siteHandler.List(fs.GetOptions{RuntimeOnly: true})
	if err != nil {
		h.logger.Debug("Trying to list runtime sites:", slog.Any("error", err))
	}
	for _, site := range runtimeSites {
		runtimeSiteNames = append(runtimeSiteNames, site.Name)
	}

	inputSitesMatchRuntimeSites := utils.StringSlicesEqual(inputSiteNames, runtimeSiteNames)

	if !inputSitesMatchRuntimeSites {
		siteState, err = h.Bootstrap(&h.ConfigBootstrap)
		if err != nil {
			return fmt.Errorf("Failed to bootstrap: %s", err)
		}
		h.PostExec(&h.ConfigBootstrap, siteState)
		return nil
	}

	siteState, err = h.siteStateLoader.Load()
	if err != nil || siteState == nil {
		return fmt.Errorf("Failed to load site state: %s", err)
	}
	if !siteState.IsBundle() {
		err = h.siteStateRenderer.Refresh(siteState)
		if err != nil {
			return fmt.Errorf("Failed to refresh site state: %s", err)
		}
	}

	return nil
}

func (h *InputResourceHandler) tearDownNamespace() error {
	h.logger.Info("No site configured, tearing down namespace")
	err := h.TearDown(h.namespace)
	if err != nil {
		return err
	}

	if h.deduplicator != nil {
		h.logger.Info("Stopping deduplicator")
		h.deduplicator.Stop()
	}
	return nil
}
