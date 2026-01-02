package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/images"
	"github.com/skupperproject/skupper/internal/nonkube/bootstrap/controller"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type ControllerConfig struct {
	containerEngine          string
	containerEndpointDefault string
	username                 string
	hostDataHome             string
	xdgDataHome              string
	containerEndpoint        string
}

func Install(platform string) error {

	systemdGlobal, err := common.NewSystemdGlobal(platform)
	if err != nil {
		return err
	}

	err = systemdGlobal.Enable()
	if err != nil {
		return err
	}

	config, err := configEnvVariables(platform)
	if err != nil {
		return err
	}

	containerName := fmt.Sprintf("%s-skupper-controller", config.username)

	isContainerAlreadyRunningInPodman := IsContainerRunning(containerName, types.PlatformPodman)

	if isContainerAlreadyRunningInPodman {
		fmt.Printf("Warning: The system controller container %q is already running in Podman.\n", containerName)
		return nil
	}

	isContainerAlreadyRunningInDocker := IsContainerRunning(containerName, types.PlatformDocker)

	if isContainerAlreadyRunningInDocker {
		fmt.Printf("Warning: The system controller container %q is already running in Docker.\n", containerName)
		return nil
	}

	cli, err := internalclient.NewCompatClient(config.containerEndpoint, "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %v", err)
	}

	err = cli.ImagePull(context.TODO(), images.GetSystemControllerImageName())
	if err != nil {
		return fmt.Errorf("failed to pull system-controller image: %v", err)
	}
	fmt.Printf("Pulled system-controller image: %s\n", images.GetSystemControllerImageName())

	env := map[string]string{
		"CONTAINER_ENDPOINT":  config.containerEndpoint,
		"SKUPPER_OUTPUT_PATH": config.hostDataHome,
		"CONTAINER_ENGINE":    config.containerEngine,
		"SKUPPER_SYSTEM_RELOAD_TYPE": utils.DefaultStr(os.Getenv(types.ENV_SYSTEM_AUTO_RELOAD),
			types.SystemReloadTypeManual),
	}

	//To mount a volume as a bind, the host path must be specified in the Name field
	//instead of the Source field. If the values Name/Destination are empty, volumes will be ignored, not mounted,
	//and the system-controller container will fail to start.

	mounts := []container.Volume{}

	volumeDestination := fmt.Sprintf("/var/run/%s.sock", platform)

	if strings.HasPrefix(config.containerEndpoint, "unix://") {
		socketPath := strings.TrimPrefix(config.containerEndpoint, "unix://")
		mounts = append(mounts, container.Volume{
			Name:        socketPath,
			Destination: volumeDestination,
			Mode:        "z",
			RW:          true,
		})
	} else if strings.HasPrefix(config.containerEndpoint, "/") {

		mounts = append(mounts, container.Volume{
			Name:        config.containerEndpoint,
			Destination: volumeDestination,
			Mode:        "z",
			RW:          true,
		})
	}

	mounts = append(mounts, container.Volume{
		Name:        config.hostDataHome,
		Destination: "/output",
		Mode:        "z",
		RW:          true,
	})

	//This is necessary to start the container with an equivalent to "--security-opt label=disable"
	annotations := map[string]string{
		"io.podman.annotations.label": "disable",
	}

	sysControllerContainer := container.Container{
		Name:        containerName,
		Image:       images.GetSystemControllerImageName(),
		Env:         env,
		Mounts:      mounts,
		Annotations: annotations,
	}

	err = cli.ContainerCreate(&sysControllerContainer)
	if err != nil {
		return fmt.Errorf("failed to create system-controller container: %v", err)
	}
	err = cli.ContainerStart(sysControllerContainer.Name)
	if err != nil {
		return fmt.Errorf("failed to start system-controller container: %v", err)
	}

	err = createSystemdService(sysControllerContainer, platform)
	if err != nil {
		return fmt.Errorf("failed to create system-controller systemd service: %v", err)
	}

	fmt.Printf("Platform %s is now configured for Skupper\n", platform)

	return nil
}

func configEnvVariables(platform string) (*ControllerConfig, error) {

	controllerConfig := ControllerConfig{}
	var err error

	// Set CONTAINER_ENGINE
	switch platform {
	case "docker":
		_, err = exec.LookPath("docker")
		if err != nil {
			return nil, fmt.Errorf("docker not found")
		}

	default:
		_, err = exec.LookPath("podman")
		if err != nil {
			return nil, fmt.Errorf("podman not found")
		}

	}
	err = os.Setenv("CONTAINER_ENGINE", platform)
	if err != nil {
		return nil, err
	}

	controllerConfig.containerEngine = platform

	// Get current username
	currentUser, err := user.Current()
	if err != nil {
		return nil, fmt.Errorf("Failed to get current user: %v", err)
	}
	uid := currentUser.Uid
	controllerConfig.username = currentUser.Username

	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%s", uid)
	}

	containerEndpointDefault := fmt.Sprintf("unix://%s/podman/podman.sock", xdgRuntimeDir)

	if platform == "docker" {
		containerEndpointDefault = "unix:///run/docker.sock"
	}

	uidInt, _ := strconv.Atoi(uid)
	xdgDataHome := "/output"
	hostDataHome := api.GetHostDataHome()
	if uidInt == 0 {
		if platform == "podman" {
			containerEndpointDefault = "unix:///run/podman/podman.sock"
		}
		hostDataHome = "/var/lib/skupper"
	}

	if err := os.MkdirAll(api.GetDefaultOutputNamespacesPath(), 0755); err != nil {
		return nil, fmt.Errorf("Failed to create directory: %v", err)
	}

	containerEndpoint := os.Getenv("CONTAINER_ENDPOINT")
	if containerEndpoint == "" {
		containerEndpoint = containerEndpointDefault
	}

	err = os.Setenv("CONTAINER_ENDPOINT", containerEndpoint)
	if err != nil {
		return nil, err
	}

	controllerConfig.containerEndpoint = containerEndpoint
	controllerConfig.containerEndpointDefault = containerEndpointDefault
	controllerConfig.hostDataHome = hostDataHome
	controllerConfig.xdgDataHome = xdgDataHome

	return &controllerConfig, nil
}

func createSystemdService(container container.Container, platform string) error {

	// Creating startup scripts
	startupArgs := controller.StartupScriptsArgs{
		Name:     container.Name,
		Platform: types.Platform(platform),
	}
	scripts, err := controller.GetStartupScripts(startupArgs, api.GetSystemControllerPath())
	if err != nil {
		return fmt.Errorf("error getting startup scripts: %w", err)
	}
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w", err)
	}

	// Creating systemd user service
	systemd, err := controller.NewSystemdServiceInfo(container, platform)
	if err != nil {
		return err
	}
	if err = systemd.Create(); err != nil {
		return fmt.Errorf("unable to create startup service %q - %v", systemd.GetServiceName(), err)
	}

	return nil
}

func IsContainerRunning(containerName string, platform types.Platform) bool {

	endpoint := fmt.Sprintf("unix://%s/podman/podman.sock", api.GetRuntimeDir())
	if platform == types.PlatformDocker {
		endpoint = "unix:///run/docker.sock"
	}

	cli, err := internalclient.NewCompatClient(endpoint, "")
	if err != nil {
		return false
	}

	containers, err := cli.ContainerList()
	if err != nil {
		return false
	}

	for _, container := range containers {
		if container.Name == containerName {
			return true
		}
	}

	return false
}
