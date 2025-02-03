package bundle

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/images"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

var (
	//go:embed router_free_port.py
	FreePortScript string
)

type SiteStateRenderer struct {
	loadedSiteState *api.SiteState
	siteState       *api.SiteState
	configRenderer  *common.FileSystemConfigurationRenderer
	containers      map[string]container.Container
	Strategy        BundleStrategy
	Platform        types.Platform
}

func (s *SiteStateRenderer) Render(loadedSiteState *api.SiteState, reload bool) error {
	var err error
	var validator api.SiteStateValidator = &common.SiteStateValidator{}
	err = validator.Validate(loadedSiteState)
	if err != nil {
		return err
	}
	s.loadedSiteState = loadedSiteState
	// active (runtime) SiteState
	s.siteState = common.CopySiteState(s.loadedSiteState)
	err = common.RedeemClaims(s.siteState)
	if err != nil {
		return fmt.Errorf("failed to redeem claims: %v", err)
	}
	// router config needs to be updated, after generation
	// to add a template variable for local port to be determined
	// during bundle installation
	if err = common.CreateRouterAccess(s.siteState); err != nil {
		return err
	}
	s.siteState.CreateLinkAccessesCertificates()
	s.siteState.CreateBridgeCertificates()
	// rendering non-kube configuration files and certificates
	s.configRenderer = &common.FileSystemConfigurationRenderer{
		SslProfileBasePath: "{{.SslProfileBasePath}}",
		Platform:           string(s.Platform),
		Bundle:             true,
	}
	err = s.configRenderer.Render(s.siteState)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Serializing loaded and runtime site states
	if err = s.configRenderer.MarshalSiteStates(s.loadedSiteState, s.siteState); err != nil {
		return err
	}

	if err = s.prepareContainers(); err != nil {
		return err
	}
	if err = s.createContainerScript(); err != nil {
		return err
	}
	if err = s.createFreePortScript(); err != nil {
		return err
	}
	// Create systemd service and scripts
	if err = CreateSystemdServices(s.siteState); err != nil {
		return err
	}
	if err = CreateStartupScripts(s.siteState, s.Platform); err != nil {
		return err
	}
	if err = s.createBundle(); err != nil {
		return err
	}
	if err = s.removeSiteFiles(); err != nil {
		return err
	}
	return nil
}

func (s *SiteStateRenderer) prepareContainers() error {
	s.containers = make(map[string]container.Container)
	s.containers[types.RouterComponent] = container.Container{
		Name:  "{{.Namespace}}-skupper-router",
		Image: images.GetRouterImageName(),
		Env: map[string]string{
			"APPLICATION_NAME":      "skupper-router",
			"QDROUTERD_CONF":        "/etc/skupper-router/config/" + types.TransportConfigFile,
			"QDROUTERD_CONF_TYPE":   "json",
			"SKUPPER_SITE_ID":       s.configRenderer.RouterConfig.GetSiteMetadata().Id,
			"SSL_PROFILE_BASE_PATH": "/etc/skupper-router",
		},
		Labels: map[string]string{
			types.ComponentAnnotation: types.TransportDeploymentName,
			types.SiteId:              s.configRenderer.RouterConfig.GetSiteMetadata().Id,
		},
		FileMounts: []container.FileMount{
			{
				Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.RouterConfigPath)),
				Destination: "/etc/skupper-router/config",
				Options:     []string{"z"},
			},
			{
				Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", string(api.CertificatesPath)),
				Destination: "/etc/skupper-router/runtime/certs",
				Options:     []string{"z"},
			},
		},
		RestartPolicy: "always",
		// TODO handle resource utilization with container sites
		//      use pkg.nonkube.cgroups.CgroupControllers to
		//      validate whether CPU and memory thresholds can be
		//      set to the container
	}
	logger := common.NewLogger()
	if logger.Enabled(nil, slog.LevelDebug) {
		for name, newContainer := range s.containers {
			containerJson, _ := json.Marshal(newContainer)
			logger.Debug("container prepared:",
				slog.String("name", name),
				slog.String("container", string(containerJson)),
			)
		}
	}
	return nil
}

func (s *SiteStateRenderer) createContainerScript() error {
	logger := common.NewLogger()
	scriptsPath := api.GetInternalBundleOutputPath(s.siteState.Site.Namespace, api.ScriptsPath)
	scriptContent := containersToShell(s.containers)
	logger.Debug("writing containers_create.sh", slog.String("path", scriptsPath))
	err := os.WriteFile(path.Join(scriptsPath, "containers_create.sh"), scriptContent, 0755)
	if err != nil {
		return fmt.Errorf("failed to create containers script: %v", err)
	}
	return nil
}

func (s *SiteStateRenderer) createBundle() error {
	var logger = common.NewLogger()
	bundlesHomeDir := api.GetDefaultOutputBundlesPath()
	siteHomeDir := api.GetDefaultBundleOutputPath(s.siteState.Site.Namespace)
	tarball := utils.NewTarball()
	err := tarball.AddFiles(bundlesHomeDir, s.siteState.GetNamespace())
	if err != nil {
		return fmt.Errorf("failed to add files to tarball (%q): %v", siteHomeDir, err)
	}
	var generator BundleGenerator
	switch s.Strategy {
	case BundleStrategyTarball:
		generator = &TarballBundle{
			SiteName:   s.siteState.Site.Name,
			Namespace:  s.siteState.GetNamespace(),
			OutputPath: bundlesHomeDir,
		}
	default:
		generator = &SelfExtractingBundle{
			SiteName:   s.siteState.Site.Name,
			Namespace:  s.siteState.GetNamespace(),
			OutputPath: bundlesHomeDir,
		}
	}
	logger.Debug("generating bundle:", slog.String("path", bundlesHomeDir), slog.String("site", s.siteState.Site.Name))
	err = generator.Generate(tarball, string(s.Platform))
	if err != nil {
		return fmt.Errorf("failed to generate site bundle (%q): %v", s.siteState.Site.Name, err)
	}
	return nil
}

func (s *SiteStateRenderer) removeSiteFiles() error {
	logger := common.NewLogger()
	siteHomeDir := api.GetDefaultBundleOutputPath(s.siteState.Site.Namespace)
	logger.Debug("removing temporary bundle home", slog.String("path", siteHomeDir))
	err := os.RemoveAll(siteHomeDir)
	if err != nil {
		return fmt.Errorf("file to remove temporary site directory %q: %v", siteHomeDir, err)
	}
	return nil
}

func (s *SiteStateRenderer) createFreePortScript() error {
	logger := common.NewLogger()
	scriptsPath := api.GetInternalBundleOutputPath(s.siteState.Site.Namespace, api.ScriptsPath)
	logger.Debug("writing freeport.sh", slog.String("path", scriptsPath))
	err := os.WriteFile(path.Join(scriptsPath, "router_free_port.py"), []byte(FreePortScript), 0755)
	if err != nil {
		return fmt.Errorf("failed to create router_free_port.py script: %v", err)
	}
	return nil
}
