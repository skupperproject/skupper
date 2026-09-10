package networkobserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/config"
	"github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type ports struct {
	prometheus int
	netobs     int
	metrics    int
	router     string
}

type Installer struct {
	Namespace   string
	Platform    string
	ports       ports
	logger      *slog.Logger
	cli         *compat.CompatClient
	siteHandler *fs.SiteHandler
}

type InstallResult struct {
	URL string
}

func NewInstaller(namespace string) (*Installer, error) {
	selectedPlatform, err := detectPlatform()
	if err != nil {
		return nil, err
	}
	containerEndpoint, err := getContainerEndpoint(selectedPlatform)
	if err != nil {
		return nil, err
	}

	compatClient, err := compat.NewCompatClient(containerEndpoint, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container client: %v", err)
	}

	return &Installer{
		Namespace:   namespace,
		Platform:    selectedPlatform,
		logger:      slog.Default().With("component", "network.observer.installer"),
		siteHandler: fs.NewSiteHandler(namespace),
		cli:         compatClient,
	}, nil
}

func (i *Installer) ValidatePrerequisitesForInstall() error {
	i.logger.Info("Validating prerequisites", slog.String("namespace", i.Namespace))
	namespacePath := api.GetHostNamespaceHome(i.Namespace)

	if _, err := os.Stat(namespacePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("namespace %q not found", i.Namespace)
		}
		return err
	}

	if !IsPrometheusInstalled() {
		return fmt.Errorf("prometheus is not installed; run \"skupper system prometheus\" first")
	}

	netobsContainer := fmt.Sprintf("%s-skupper-network-observer", i.Namespace)
	if i.isContainerRunning(netobsContainer) {
		return fmt.Errorf("container %q is already running in %s", netobsContainer, i.Platform)
	}

	sites, err := i.siteHandler.List(fs.GetOptions{InputOnly: true})
	if err != nil {
		return err
	} else {
		if len(sites) == 0 {
			return fmt.Errorf("required site not found")
		}
	}

	clientCertsPath := filepath.Join(namespacePath, string(api.CertificatesPath), "skupper-local-client")
	requiredCerts := []string{"ca.crt", "tls.crt", "tls.key"}
	for _, cert := range requiredCerts {
		certPath := filepath.Join(clientCertsPath, cert)
		if _, err := os.Stat(certPath); os.IsNotExist(err) {
			return fmt.Errorf("required certificate not found: %s", certPath)
		}
	}

	return nil
}

func (i *Installer) Install() (*InstallResult, error) {

	i.logger.Info("Starting network observer installation", slog.String("namespace", i.Namespace))

	if err := i.generateConfigurations(); err != nil {
		return nil, fmt.Errorf("failed to generate configurations: %w", err)
	}

	systemdGlobal, err := common.NewSystemdGlobal(i.Platform)
	if err != nil {
		return nil, err
	}

	err = systemdGlobal.Enable()
	if err != nil {
		return nil, err
	}

	err = i.installContainer(GetNetworkObserverContainer(i.Namespace, i.ports))
	if err != nil {
		return nil, err
	}

	if err := WriteTargetFile(i.Namespace, i.ports.metrics); err != nil {
		return nil, fmt.Errorf("failed to write prometheus target file: %w", err)
	}

	err = i.createNetObsSystemdService()
	if err != nil {
		return nil, fmt.Errorf("failed to create systemd services: %w", err)
	}

	i.logger.Info("Network observer installation completed successfully")

	return &InstallResult{
		URL: fmt.Sprintf("http://localhost:%d", i.ports.netobs),
	}, nil
}

func (i *Installer) ValidatePrerequisitesForUninstall() error {
	netobsContainer := fmt.Sprintf("%s-skupper-network-observer", i.Namespace)
	if !i.isContainerRunning(netobsContainer) {
		return fmt.Errorf("network observer is not running in namespace %q, there is nothing to uninstall", i.Namespace)
	}
	return nil
}

func UninstallForNamespace(namespace string) error {
	targetFile := filepath.Join(api.GetPrometheusTargetsDir(), namespace+".json")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		return nil
	}

	installer, err := NewInstaller(namespace)
	if err != nil {
		return err
	}
	return installer.Uninstall()
}

func (i *Installer) Uninstall() error {
	i.logger.Info("Uninstalling network observer", slog.String("namespace", i.Namespace))

	if err := RemoveTargetFile(i.Namespace); err != nil {
		i.logger.Warn("Failed to remove prometheus target file", slog.Any("error", err))
	}

	manager := NewSystemdServiceManager(i.Namespace, i.Platform, ports{})
	if err := manager.RemoveNetworkObserverService(); err != nil {
		i.logger.Warn("Failed to remove systemd services", slog.Any("error", err))
	}

	containerNames := []string{
		fmt.Sprintf("%s-skupper-network-observer", i.Namespace),
	}
	for _, name := range containerNames {
		if i.isContainerRunning(name) {
			if err := i.cli.ContainerStop(name); err != nil {
				i.logger.Warn("Failed to stop container", slog.String("name", name), slog.String("error", err.Error()))
			}
		}

		if err := i.cli.ContainerRemove(name); err != nil {
			i.logger.Warn("Failed to remove container", slog.String("name", name), slog.Any("error", err))
		}
	}

	i.logger.Info("Network observer uninstalled successfully")
	return nil
}

func detectPlatform() (string, error) {
	platform := config.GetPlatform()

	if platform != types.PlatformDocker && platform != types.PlatformPodman {
		return "", fmt.Errorf("unsupported platform %q for network observer", platform)
	}

	switch platform {
	case "docker":
		_, err := exec.LookPath("docker")
		if err != nil {
			return "", fmt.Errorf("docker not found")
		}

	default:
		_, err := exec.LookPath("podman")
		if err != nil {
			return "", fmt.Errorf("podman not found")
		}

	}

	return string(platform), nil
}

func getContainerEndpoint(platform string) (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("Failed to get current user: %v", err)
	}
	uid := currentUser.Uid
	uidInt, _ := strconv.Atoi(uid)

	xdgRuntimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if xdgRuntimeDir == "" {
		xdgRuntimeDir = fmt.Sprintf("/run/user/%s", uid)
	}

	containerEndpointDefault := os.Getenv("CONTAINER_ENDPOINT")

	if containerEndpointDefault == "" {

		if platform == "docker" {
			containerEndpointDefault = "unix:///run/docker.sock"
		} else {

			containerEndpointDefault = fmt.Sprintf("unix://%s/podman/podman.sock", xdgRuntimeDir)

			if uidInt == 0 {
				if platform == "podman" {
					containerEndpointDefault = "unix:///run/podman/podman.sock"
				}
			}
		}
	}

	return containerEndpointDefault, nil
}

func (i *Installer) isContainerRunning(containerName string) bool {

	containers, err := i.cli.ContainerList()
	if err != nil {
		return false
	}

	for _, c := range containers {
		if c.Name == containerName {
			return c.Running
		}
	}

	return false
}

func (i *Installer) generateConfigurations() error {
	prometheusPort, err := ReadPrometheusPort()
	if err != nil {
		return err
	}
	metricsPort, err := NextFreeMetricsPort(9000)
	if err != nil {
		return fmt.Errorf("failing to assign port to metrics: %s", err)
	}
	netobsPort, err := utils.TcpPortNextFree(8080)
	if err != nil {
		return fmt.Errorf("failing to assign port to network observer: %s", err)
	}

	routerEndpoint, err := runtime.GetLocalRouterAddress(i.Namespace)
	if err != nil {
		return fmt.Errorf("failed to determine local router address: %w", err)
	}

	i.ports = ports{
		prometheus: prometheusPort,
		netobs:     netobsPort,
		metrics:    metricsPort,
		router:     routerEndpoint,
	}

	i.logger.Info("Assigned ports",
		slog.Int("prometheus", prometheusPort),
		slog.Int("netobs", netobsPort),
		slog.Int("metrics", metricsPort),
		slog.String("router", routerEndpoint),
	)

	return nil
}

func (i *Installer) installContainer(newContainer container.Container) error {
	ctx, cn := context.WithTimeout(context.Background(), time.Minute*10)
	defer cn()
	err := i.cli.ImagePull(ctx, newContainer.Image)
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}
	fmt.Printf("Pulled image: %s\n", newContainer.Image)

	err = i.cli.ContainerCreate(&newContainer)
	if err != nil {
		return fmt.Errorf("failed to create container %s: %v", newContainer.Name, err)
	}
	err = i.cli.ContainerStart(newContainer.Name)
	if err != nil {
		return fmt.Errorf("failed to start container %s: %v", newContainer.Name, err)
	}

	return nil
}

func (i *Installer) createNetObsSystemdService() error {
	i.logger.Info("Creating systemd service for Network Observer", slog.String("namespace", i.Namespace))

	manager := NewSystemdServiceManager(i.Namespace, i.Platform, i.ports)
	err := manager.CreateNetworkObserverService()
	if err != nil {
		return fmt.Errorf("failed to create systemd service: %w", err)
	}

	i.logger.Info("Systemd service created successfully")
	return nil
}
