package bundle

import (
	"fmt"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/non_kube/apis"
	"github.com/skupperproject/skupper/pkg/non_kube/common"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SiteStateRenderer struct {
	loadedSiteState *apis.SiteState
	siteState       *apis.SiteState
	configRenderer  *common.FileSystemConfigurationRenderer
	containers      map[string]container.Container
}

func (s *SiteStateRenderer) Render(loadedSiteState *apis.SiteState) error {
	var err error
	var validator apis.SiteStateValidator = &common.SiteStateValidator{}
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
		Force:              false, // TODO discuss how this should be handled?
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
	// Create systemd service and scripts
	if err = s.createSystemdService(); err != nil {
		return err
	}
	if err = s.createSedScript(); err != nil {
		return err
	}
	s.createBundle()
	s.removeSiteFiles()
	return nil
}

func (s *SiteStateRenderer) prepareContainers() error {
	s.containers = make(map[string]container.Container)
	s.containers[types.RouterComponent] = container.Container{
		Name:  fmt.Sprintf("%s-skupper-router", s.siteState.Site.Name),
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
				Source:      path.Join("{{.SitesPath}}", "{{.SiteName}}", "config/router"),
				Destination: "/etc/skupper-router/config",
				Options:     []string{"z"},
			},
			{
				Source:      path.Join("{{.SitesPath}}", "{{.SiteName}}", "certificates"),
				Destination: "/etc/skupper-router/certificates",
				Options:     []string{"z"},
			},
		},
		RestartPolicy: "always",
		// TODO handle resource utilization with container sites
		//      use pkg.non_kube.cgroups.CgroupControllers to
		//      validate whether CPU and memory thresholds can be
		//      set to the container
	}
	return nil
}

func (s *SiteStateRenderer) createContainerScript() error {
	return nil
}

func (s *SiteStateRenderer) createSystemdService() error {
	// TODO Modify logic to put Template vars in place
	// Creating startup scripts first
	scripts, err := common.GetStartupScripts(s.siteState.Site, s.configRenderer.RouterConfig.GetSiteMetadata().Id)
	if err != nil {
		return fmt.Errorf("error getting startup scripts: %w", err)
	}
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w", err)
	}

	// Creating systemd user service
	systemd, err := common.NewSystemdServiceInfo(s.siteState.Site)
	if err != nil {
		return err
	}
	if err = systemd.Create(); err != nil {
		return fmt.Errorf("unable to create startup service %q - %v\n", systemd.GetServiceName(), err)
	}

	// Validate if lingering is enabled for current user
	if !apis.IsRunningInContainer() {
		username := utils.ReadUsername()
		if os.Getuid() != 0 && !common.IsLingeringEnabled(username) {
			fmt.Printf("It is recommended to enable lingering for %s, otherwise Skupper may not start on boot.\n", username)
		}
	}

	return nil
}
