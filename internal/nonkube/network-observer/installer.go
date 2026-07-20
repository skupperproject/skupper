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
	nginx      int
	prometheus int
	netobs     int
	metrics    int
	router     string
}

type Installer struct {
	Namespace   string
	Username    string
	Password    string
	Platform    string
	ports       ports
	logger      *slog.Logger
	cli         *compat.CompatClient
	siteHandler *fs.SiteHandler
}

type InstallResult struct {
	URL      string
	Username string
	Password string
}

func NewInstaller(namespace string, username string, password string) (*Installer, error) {
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
		Username:    username,
		Password:    password,
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

	containerNames := []string{
		fmt.Sprintf("%s-skupper-prometheus", i.Namespace),
		fmt.Sprintf("%s-skupper-network-observer", i.Namespace),
		fmt.Sprintf("%s-skupper-nginx", i.Namespace),
	}

	for _, containerName := range containerNames {
		if i.isContainerRunning(containerName) {
			return fmt.Errorf("container %q is already running in %s", containerName, i.Platform)
		}
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

	if err := i.createDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	if err := i.generateConfigurations(); err != nil {
		return nil, fmt.Errorf("failed to generate configurations: %w", err)
	}

	if err := i.generateCertificates(); err != nil {
		return nil, fmt.Errorf("failed to generate certificates: %w", err)
	}

	generatedPassword, err := i.generateHtpasswd()
	if err != nil {
		return nil, fmt.Errorf("failed to generate htpasswd: %w", err)
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
	err = i.installContainer(GetPrometheusContainer(i.Namespace, i.ports))
	if err != nil {
		return nil, err
	}
	err = i.installContainer(GetNginxContainer(i.Namespace))
	if err != nil {
		return nil, err
	}

	err = i.createSystemdServices()
	if err != nil {
		return nil, fmt.Errorf("failed to create systemd services: %w", err)
	}

	i.logger.Info("Network observer installation completed successfully")

	return &InstallResult{
		URL:      fmt.Sprintf("https://localhost:%d", i.ports.nginx),
		Username: i.Username,
		Password: generatedPassword,
	}, nil
}

func (i *Installer) ValidatePrerequisitesForUninstall() error {

	containerNames := []string{
		fmt.Sprintf("%s-skupper-prometheus", i.Namespace),
		fmt.Sprintf("%s-skupper-network-observer", i.Namespace),
		fmt.Sprintf("%s-skupper-nginx", i.Namespace),
	}

	containersAreRunning := false
	for _, containerName := range containerNames {
		if i.isContainerRunning(containerName) {
			containersAreRunning = true
		}
	}

	if !containersAreRunning {
		return fmt.Errorf("network observer containers not running in namespace %q, there is nothing to uninstall", i.Namespace)
	}

	return nil
}

func (i *Installer) Uninstall() error {
	i.logger.Info("Uninstalling network observer", slog.String("namespace", i.Namespace))

	manager := NewSystemdServiceManager(i.Namespace, i.Platform, ports{})
	if err := manager.RemoveServices(); err != nil {
		i.logger.Warn("Failed to remove systemd services", slog.Any("error", err))
	}

	containerNames := []string{
		fmt.Sprintf("%s-skupper-nginx", i.Namespace),
		fmt.Sprintf("%s-skupper-network-observer", i.Namespace),
		fmt.Sprintf("%s-skupper-prometheus", i.Namespace),
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

	namespacePath := api.GetHostNamespaceHome(i.Namespace)
	dataDir := filepath.Join(namespacePath, "network-observer")
	if err := os.RemoveAll(dataDir); err != nil {
		i.logger.Warn("Failed to remove network-observer data directory", slog.String("path", dataDir), slog.Any("error", err))
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

func (i *Installer) createDirectories() error {
	namespacePath := api.GetHostNamespaceHome(i.Namespace)
	dirs := []string{
		filepath.Join(namespacePath, "network-observer"),
		filepath.Join(namespacePath, "network-observer", "prometheus"),
		filepath.Join(namespacePath, "network-observer", "nginx"),
		filepath.Join(namespacePath, "network-observer", "nginx", "conf.d"),
		filepath.Join(namespacePath, "network-observer", "htpasswd"),
		filepath.Join(namespacePath, "network-observer", "certs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	dataDir := filepath.Join(namespacePath, "network-observer", "prometheus", "data")
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dataDir, err)
	}

	return nil
}

func (i *Installer) generateConfigurations() error {
	namespacePath := api.GetHostNamespaceHome(i.Namespace)

	nginxPort, err := utils.TcpPortNextFree(8443)
	if err != nil {
		return fmt.Errorf("failing to assign port to nginx: %s", err)
	}
	prometheusPort, err := utils.TcpPortNextFree(9090)
	if err != nil {
		return fmt.Errorf("failing to assign port to prometheus: %s", err)
	}
	metricsPort, err := utils.TcpPortNextFree(9000)
	if err != nil {
		return fmt.Errorf("failing to assign port to prometheus API: %s", err)
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
		nginx:      nginxPort,
		prometheus: prometheusPort,
		netobs:     netobsPort,
		metrics:    metricsPort,
		router:     routerEndpoint,
	}

	i.logger.Info("Assigned ports",
		slog.Int("nginx", nginxPort),
		slog.Int("prometheus", prometheusPort),
		slog.Int("netobs", netobsPort),
		slog.Int("metrics", metricsPort),
		slog.String("router", routerEndpoint),
	)

	prometheusPath := filepath.Join(namespacePath, "network-observer", "prometheus", "prometheus.yml")
	if err := os.WriteFile(prometheusPath, []byte(RenderPrometheusConfig(netobsPort)), 0644); err != nil {
		return fmt.Errorf("failed to write prometheus config: %w", err)
	}

	nginxPath := filepath.Join(namespacePath, "network-observer", "nginx", "conf.d", "default.conf")
	if err := os.WriteFile(nginxPath, []byte(RenderNginxConfig(nginxPort, netobsPort)), 0644); err != nil {
		return fmt.Errorf("failed to write nginx config: %w", err)
	}

	return nil
}

func (i *Installer) generateCertificates() error {
	namespacePath := api.GetHostNamespaceHome(i.Namespace)
	caDir := filepath.Join(namespacePath, string(api.IssuersPath), "skupper-local-ca")
	certDir := filepath.Join(namespacePath, "network-observer", "certs")

	return GenerateNginxCert(caDir, certDir)
}

func (i *Installer) generateHtpasswd() (string, error) {
	namespacePath := api.GetHostNamespaceHome(i.Namespace)
	htpasswdPath := filepath.Join(namespacePath, "network-observer", "htpasswd", "htpasswd")

	username, password, htpasswdContent, err := GenerateHtpasswdCredentials(i.Username, i.Password)
	if err != nil {
		return "", fmt.Errorf("failed to generate htpasswd credentials: %w", err)
	}

	if err := os.WriteFile(htpasswdPath, []byte(htpasswdContent), 0600); err != nil {
		return "", fmt.Errorf("failed to write htpasswd file: %w", err)
	}

	i.logger.Info("Generated htpasswd credentials", "username", username)
	return password, nil
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

func (i *Installer) createSystemdServices() error {
	i.logger.Info("Creating systemd services", slog.String("namespace", i.Namespace))

	manager := NewSystemdServiceManager(i.Namespace, i.Platform, i.ports)
	err := manager.CreateServices()
	if err != nil {
		return fmt.Errorf("failed to create systemd services: %w", err)
	}

	i.logger.Info("Systemd services created successfully")
	return nil
}
