package bundle

import (
	_ "embed"
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
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
}

func (s *SiteStateRenderer) Render(loadedSiteState *api.SiteState) error {
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
		Force:              false,
	}
	err = s.configRenderer.Render(s.siteState)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Serializing loaded and runtime site states
	if err = s.configRenderer.MarshalSiteStates(*s.loadedSiteState, *s.siteState); err != nil {
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
	if err = CreateStartupScripts(s.siteState); err != nil {
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
				Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", "config/router"),
				Destination: "/etc/skupper-router/config",
				Options:     []string{"z"},
			},
			{
				Source:      path.Join("{{.NamespacesPath}}", "{{.Namespace}}", "certificates"),
				Destination: "/etc/skupper-router/certificates",
				Options:     []string{"z"},
			},
		},
		RestartPolicy: "always",
		// TODO handle resource utilization with container sites
		//      use pkg.nonkube.cgroups.CgroupControllers to
		//      validate whether CPU and memory thresholds can be
		//      set to the container
	}
	return nil
}

func (s *SiteStateRenderer) createContainerScript() error {
	scriptsPath := api.GetInternalOutputPath(s.siteState.Site.Namespace, api.RuntimeScriptsPath)
	scriptContent := containersToShell(s.containers)
	err := os.WriteFile(path.Join(scriptsPath, "containers_create.sh"), scriptContent, 0755)
	if err != nil {
		return fmt.Errorf("failed to create containers script: %v", err)
	}
	return nil
}

func (s *SiteStateRenderer) createBundle() error {
	namespacesHomeDir := api.GetDefaultOutputNamespacesPath()
	siteHomeDir := api.GetDefaultOutputPath(s.siteState.Site.Namespace)
	tarball := utils.NewTarball()
	err := tarball.AddFiles(namespacesHomeDir, s.siteState.Site.Namespace)
	if err != nil {
		return fmt.Errorf("failed to add files to tarball (%q): %v", siteHomeDir, err)
	}
	var generator internalbundle.BundleGenerator
	if !config.GetPlatform().IsTarball() {
		generator = &internalbundle.SelfExtractingBundle{
			SiteName:   s.siteState.Site.Name,
			Namespace:  s.siteState.Site.Namespace,
			OutputPath: namespacesHomeDir,
		}
	} else {
		generator = &internalbundle.TarballBundle{
			SiteName:   s.siteState.Site.Name,
			Namespace:  s.siteState.Site.Namespace,
			OutputPath: namespacesHomeDir,
		}
	}
	err = generator.Generate(tarball)
	if err != nil {
		return fmt.Errorf("failed to generate site bundle (%q): %v", s.siteState.Site.Name, err)
	}
	return nil
}

func (s *SiteStateRenderer) removeSiteFiles() error {
	siteHomeDir := api.GetDefaultOutputPath(s.siteState.Site.Namespace)
	err := os.RemoveAll(siteHomeDir)
	if err != nil {
		return fmt.Errorf("file to remove temporary site directory %q: %v", siteHomeDir, err)
	}
	return nil
}

func (s *SiteStateRenderer) createFreePortScript() error {
	scriptsPath := api.GetInternalOutputPath(s.siteState.Site.Namespace, api.RuntimeScriptsPath)
	err := os.WriteFile(path.Join(scriptsPath, "router_free_port.py"), []byte(FreePortScript), 0755)
	if err != nil {
		return fmt.Errorf("failed to create router_free_port.py script: %v", err)
	}
	return nil
}
