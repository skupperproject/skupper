package controller

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap"
	common2 "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

// This feature is responsible for handling the creation of input resources and
// execute the start/reload of the site configuration automatically.
type InputResourceHandler struct {
	logger          *slog.Logger
	namespace       string
	inputPath       string
	Bootstrap       func(config *bootstrap.Config) (*api.SiteState, error)
	PostExec        func(config *bootstrap.Config, siteState *api.SiteState)
	TearDown        func(namespace string, platform string) error
	ConfigBootstrap bootstrap.Config
	lock            sync.Mutex
}

func NewInputResourceHandler(namespace string, inputPath string, bStrap func(config *bootstrap.Config) (*api.SiteState, error), postBootStrap func(config *bootstrap.Config, siteState *api.SiteState), tearDown func(namespace string, platform string) error) *InputResourceHandler {

	systemReloadType := utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
		types.SystemReloadTypeManual)

	if systemReloadType == types.SystemReloadTypeManual {
		slog.Default().Debug("Automatic reloading is not configured.")
		return nil
	}

	handler := &InputResourceHandler{
		namespace: namespace,
		inputPath: inputPath,
	}

	handler.Bootstrap = bStrap
	handler.PostExec = postBootStrap
	handler.TearDown = tearDown

	var binary string

	platform := types.Platform(utils.DefaultStr(os.Getenv("CONTAINER_ENGINE"),
		string(types.PlatformPodman)))

	// TODO: add support for linux platform
	switch common.Platform(platform) {
	case common.PlatformDocker:
		binary = "docker"
	case common.PlatformPodman:
		binary = "podman"
	case common.PlatformLinux:
		slog.Default().Error("Linux platform is not supported yet")
		return nil
	default:
		slog.Default().Error("This platform value is not supported: ", slog.String("platform", string(platform)))
		return nil
	}

	handler.ConfigBootstrap = bootstrap.Config{
		Namespace: namespace,
		InputPath: inputPath,
		Platform:  platform,
		Binary:    binary,
	}

	handler.logger = slog.Default().With("component", "input.resource.handler", "namespace", namespace)
	return handler
}

func (h *InputResourceHandler) OnCreate(name string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.logger.Info(fmt.Sprintf("Resource has been created: %s", name))
	err := h.processInputFile()
	if err != nil {
		h.logger.Error(err.Error())
	}
}

// This function does not need to be implemented, given that when a file is updated,
// the event OnCreate is triggered anyway. Having it implemented would cause
// the resources to be reloaded multiple times, stopping and starting a router pod.
// (issue: the router pod is still active while going to be deleted, and the controller
// tries to create a new router pod, failing on this)
func (h *InputResourceHandler) OnUpdate(name string) {}
func (h *InputResourceHandler) OnRemove(name string) {
	h.lock.Lock()
	defer h.lock.Unlock()

	h.logger.Info(fmt.Sprintf("Resource has been deleted: %s", name))

	siteStateLoader := &common2.FileSystemSiteStateLoader{
		Path:   h.ConfigBootstrap.InputPath,
		Bundle: h.ConfigBootstrap.IsBundle,
	}
	siteState, err := siteStateLoader.Load()

	//If there is no site configured, the namespace needs to be removed
	if err != nil || siteState == nil || siteState.Site == nil {
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
	return strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
}

func (h *InputResourceHandler) OnBasePathAdded(basePath string) {}

func (h *InputResourceHandler) processInputFile() error {
	siteState, err := h.Bootstrap(&h.ConfigBootstrap)
	if err != nil {
		return fmt.Errorf("Failed to bootstrap: %s", err)
	}

	h.PostExec(&h.ConfigBootstrap, siteState)

	return nil
}

func (h *InputResourceHandler) tearDownNamespace() error {
	h.logger.Info("No site configured, tearing down namespace")
	err := h.TearDown(h.namespace, string(h.ConfigBootstrap.Platform))
	if err != nil {
		return err
	}

	return nil
}
