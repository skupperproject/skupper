package linux

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type SiteStateRenderer struct {
	loadedSiteState *api.SiteState
	siteState       *api.SiteState
	configRenderer  *common.FileSystemConfigurationRenderer
}

func (s *SiteStateRenderer) Render(loadedSiteState *api.SiteState, reload bool) error {
	var err error
	var logger = common.NewLogger()
	var validator api.SiteStateValidator = &common.SiteStateValidator{}
	err = validator.Validate(loadedSiteState)
	if err != nil {
		return err
	}
	s.loadedSiteState = loadedSiteState
	var backupData []byte
	// Restore namespace data if reload fail and backupData is not nil
	defer func() {
		if !reload {
			return
		}
		// when reload is successful, backupData must be nil
		if backupData == nil {
			return
		}
		logger.Error("Bootstrap failed, restoring namespace")
		err := common.RestoreNamespaceData(backupData)
		if err != nil {
			logger.Error("Error restoring namespace data:",
				slog.String("namespace", loadedSiteState.GetNamespace()),
				slog.String("error", err.Error()),
			)
			return
		}
		err = s.createSystemdService()
		if err != nil {
			logger.Error("Error recovering systemd service info:",
				slog.String("error", err.Error()),
			)
		}
	}()

	if reload {
		backupData, err = common.BackupNamespace(loadedSiteState.GetNamespace())
		if err != nil {
			return fmt.Errorf("failed to backup namespace: %v", err)
		}
		err = s.loadExistingSiteId(loadedSiteState)
		if err != nil {
			return err
		}
		err = s.removeSystemdService()
		if err != nil {
			return err
		}
		err = common.CleanupNamespaceForReload(loadedSiteState.GetNamespace())
		if err != nil {
			return err
		}
	}

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
	siteHome := api.GetHostSiteHome(s.siteState.Site)
	s.configRenderer = &common.FileSystemConfigurationRenderer{
		SslProfileBasePath: siteHome,
		Platform:           string(types.PlatformLinux),
	}
	err = s.configRenderer.Render(s.siteState)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// Serializing loaded and runtime site states
	if err = s.configRenderer.MarshalSiteStates(loadedSiteState, s.siteState); err != nil {
		return err
	}
	// Create systemd service
	if err = s.createSystemdService(); err != nil {
		return err
	}
	// no need to restore anything
	backupData = nil
	return nil
}

func (s *SiteStateRenderer) loadExistingSiteId(siteState *api.SiteState) error {
	routerConfig, err := common.LoadRouterConfig(siteState.GetNamespace())
	if err != nil {
		return err
	}
	// loading site id
	siteState.SiteId = routerConfig.GetSiteMetadata().Id
	return nil
}

func (s *SiteStateRenderer) createSystemdService() error {
	// Creating systemd user service
	systemd, err := common.NewSystemdServiceInfo(s.siteState, string(types.PlatformLinux))
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

func (s *SiteStateRenderer) removeSystemdService() error {
	// Removing systemd user service
	systemd, err := common.NewSystemdServiceInfo(s.loadedSiteState, string(types.PlatformLinux))
	if err != nil {
		return err
	}
	if err = systemd.Remove(); err != nil {
		return fmt.Errorf("unable to remove startup service %q - %v\n", systemd.GetServiceName(), err)
	}
	return nil
}
