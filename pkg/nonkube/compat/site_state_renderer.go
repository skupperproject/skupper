package compat

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"github.com/skupperproject/skupper/pkg/utils"
)

type SiteStateRenderer struct {
	loadedSiteState   *api.SiteState
	siteState         *api.SiteState
	configRenderer    *common.FileSystemConfigurationRenderer
	containers        map[string]container.Container
	stoppedContainers map[string]string
	Platform          types.Platform
	cli               *internalclient.CompatClient
}

func (s *SiteStateRenderer) Render(loadedSiteState *api.SiteState, reload bool) error {
	var err error
	var validator api.SiteStateValidator = &common.SiteStateValidator{}
	err = validator.Validate(loadedSiteState)
	if err != nil {
		return err
	}
	s.loadedSiteState = loadedSiteState
	s.cli, err = internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}
	var backupData []byte
	// Restore namespace data if reload fail and backupData is not nil
	defer func() {
		if !reload {
			return
		}
		// when reload is successful, backupData must be nil
		if backupData == nil {
			for _, temporaryName := range s.stoppedContainers {
				err = s.cli.ContainerRemove(temporaryName)
				if err != nil {
					fmt.Printf("Failed to remove temporary container %s: %v\n", temporaryName, err)
				}
			}
			return
		}
		fmt.Println("Bootstrap failed, restoring previous state")
		err := common.RestoreNamespaceData(backupData)
		if err != nil {
			fmt.Printf("Error restoring namespace data for %q - %s\n", loadedSiteState.GetNamespace(), err)
			return
		}
		for originalName, temporaryName := range s.stoppedContainers {
			if temporaryName != originalName {
				err = s.cli.ContainerRename(temporaryName, originalName)
				if err != nil {
					fmt.Printf("Error restoring container name from %q to %q - %s\n", temporaryName, originalName, err)
				}
			}
			err = s.cli.ContainerStart(originalName)
			if err != nil {
				fmt.Printf("Error starting container %q - %s\n", originalName, err)
			}
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
		err = s.cleanupExistingNamespace(loadedSiteState)
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
	platform := types.PlatformPodman
	if s.Platform == types.PlatformDocker {
		platform = types.PlatformDocker
	}
	s.configRenderer = &common.FileSystemConfigurationRenderer{
		Platform: string(platform),
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

	ctx, cn := context.WithTimeout(context.Background(), time.Minute*10)
	defer cn()
	if err = s.prepareContainers(); err != nil {
		return err
	}
	if err = s.pullImages(ctx); err != nil {
		return err
	}
	if err = s.createContainers(); err != nil {
		return err
	}
	if err = s.startContainers(); err != nil {
		return err
	}

	// Create systemd service and scripts
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

func (s *SiteStateRenderer) cleanupExistingNamespace(siteState *api.SiteState) error {
	// stopping containers
	containers, err := s.cli.ContainerList()
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}
	s.stoppedContainers = map[string]string{}
	for _, stopContainer := range containers {
		if siteId, ok := stopContainer.Labels[types.SiteId]; ok && siteId == siteState.SiteId {
			err = s.cli.ContainerStop(stopContainer.Name)
			if err != nil {
				return fmt.Errorf("failed to stop container: %v", err)
			}
			s.stoppedContainers[stopContainer.Name] = stopContainer.Name
			temporaryName := fmt.Sprintf("%s-backup", stopContainer.Name)
			err = s.cli.ContainerRename(stopContainer.Name, temporaryName)
			if err != nil {
				return fmt.Errorf("failed to rename container %q to %q: %v", stopContainer.Name, temporaryName, err)
			}
			s.stoppedContainers[stopContainer.Name] = temporaryName
		}
	}
	return common.CleanupNamespaceForReload(siteState.GetNamespace())
}

func (s *SiteStateRenderer) prepareContainers() error {
	siteConfigPath, err := api.GetHostSiteHome(s.siteState.Site)
	if err != nil {
		return err
	}
	s.containers = make(map[string]container.Container)
	s.containers[types.RouterComponent] = container.Container{
		Name:  fmt.Sprintf("%s-skupper-router", s.siteState.GetNamespace()),
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
				Source:      path.Join(siteConfigPath, "config/router"),
				Destination: "/etc/skupper-router/config",
				Options:     []string{"z"},
			},
			{
				Source:      path.Join(siteConfigPath, "certificates"),
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

func (s *SiteStateRenderer) pullImages(ctx context.Context) error {
	var err error
	for component, skupperContainer := range s.containers {
		err = s.cli.ImagePull(ctx, skupperContainer.Image)
		if err != nil {
			return fmt.Errorf("failed to pull %s image %s: %w", component, skupperContainer.Image, err)
		}
	}
	return nil
}

func (s *SiteStateRenderer) cleanupContainers(err error) {
	if err == nil {
		return
	}
	for _, createdContainer := range s.containers {
		_ = s.cli.ContainerStop(createdContainer.Name)
		_ = s.cli.ContainerRemove(createdContainer.Name)
	}
}

func (s *SiteStateRenderer) createContainers() error {
	var err error
	defer s.cleanupContainers(err)
	// validate if containers already exist before creating anything
	for component, skupperContainer := range s.containers {
		existingContainer, err := s.cli.ContainerInspect(skupperContainer.Name)
		if err == nil && existingContainer != nil {
			return fmt.Errorf("container %s already exists (component: %s)", skupperContainer.Name, component)
		}
	}
	for component, skupperContainer := range s.containers {
		err = s.cli.ContainerCreate(&skupperContainer)
		if err != nil {
			return fmt.Errorf("failed to create %q container (%s): %w", component, skupperContainer.Name, err)
		}
	}
	return nil
}

func (s *SiteStateRenderer) startContainers() error {
	var err error
	defer s.cleanupContainers(err)
	for component, skupperContainer := range s.containers {
		err = s.cli.ContainerStart(skupperContainer.Name)
		if err != nil {
			return fmt.Errorf("failed to start %s container %q: %w", component, skupperContainer.Name, err)
		}
	}
	return nil
}

func (s *SiteStateRenderer) createSystemdService() error {
	// Creating startup scripts first
	platform := types.PlatformPodman
	if s.Platform == types.PlatformDocker {
		platform = types.PlatformDocker
	}
	startupArgs := common.StartupScriptsArgs{
		Namespace: s.siteState.GetNamespace(),
		SiteId:    s.configRenderer.RouterConfig.GetSiteMetadata().Id,
		Platform:  platform,
	}
	scripts, err := common.GetStartupScripts(startupArgs, api.GetInternalOutputPath)
	if err != nil {
		return fmt.Errorf("error getting startup scripts: %w", err)
	}
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w", err)
	}

	// Creating systemd user service
	systemd, err := common.NewSystemdServiceInfo(s.siteState, string(s.Platform))
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
