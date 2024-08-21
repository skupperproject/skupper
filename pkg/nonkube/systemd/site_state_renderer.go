package systemd

import (
	"fmt"
	"os"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SiteStateRenderer struct {
	loadedSiteState *api.SiteState
	siteState       *api.SiteState
	configRenderer  *common.FileSystemConfigurationRenderer
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
	if err = common.CreateRouterAccess(s.siteState); err != nil {
		return err
	}
	s.siteState.CreateLinkAccessesCertificates()
	s.siteState.CreateBridgeCertificates()
	// rendering non-kube configuration files and certificates
	siteHome, err := api.GetHostSiteHome(s.siteState.Site)
	if err != nil {
		return fmt.Errorf("failed to get site home: %w", err)
	}
	s.configRenderer = &common.FileSystemConfigurationRenderer{
		Force:              false,
		SslProfileBasePath: siteHome,
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

	// Create systemd service
	if err = s.createSystemdService(); err != nil {
		return err
	}
	return nil
}

func (s *SiteStateRenderer) createSystemdService() error {
	// Creating systemd user service
	systemd, err := common.NewSystemdServiceInfo(s.siteState.Site)
	if err != nil {
		return err
	}
	if err = systemd.Create(); err != nil {
		return fmt.Errorf("unable to create startup service %q - %v\n", systemd.GetServiceName(), err)
	}

	// Validate if lingering is enabled for current user
	if !api.IsRunningInContainer() {
		username := utils.ReadUsername()
		if os.Getuid() != 0 && !common.IsLingeringEnabled(username) {
			fmt.Printf("It is recommended to enable lingering for %s, otherwise Skupper may not start on boot.\n", username)
		}
	}

	return nil
}
