package networkobserver

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type prometheusInstallerPorts struct {
	prometheus int
}

type PrometheusInstaller struct {
	Platform string
	ports    prometheusInstallerPorts
	logger   *slog.Logger
	cli      *compat.CompatClient
}

func NewPrometheusInstaller() (*PrometheusInstaller, error) {
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
	return &PrometheusInstaller{
		Platform: selectedPlatform,
		logger:   slog.Default().With("component", "prometheus.installer"),
		cli:      compatClient,
	}, nil
}

func (p *PrometheusInstaller) isContainerRunning(name string) bool {
	containers, err := p.cli.ContainerList()
	if err != nil {
		return false
	}
	for _, c := range containers {
		if c.Name == name {
			return c.Running
		}
	}
	return false
}

func (p *PrometheusInstaller) ValidatePrerequisitesForInstall() error {
	if p.isContainerRunning("skupper-prometheus") {
		return fmt.Errorf("container \"skupper-prometheus\" is already running in %s", p.Platform)
	}
	return nil
}

func (p *PrometheusInstaller) Install() error {
	p.logger.Info("Starting host-level prometheus installation")

	prometheusHome := api.GetHostPrometheusHome()
	targetsDir := api.GetPrometheusTargetsDir()
	dataDir := filepath.Join(prometheusHome, "data")

	for _, d := range []struct {
		path string
		perm os.FileMode
	}{
		{prometheusHome, 0755},
		{targetsDir, 0755},
		{dataDir, 0750},
	} {
		if err := os.MkdirAll(d.path, d.perm); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d.path, err)
		}
	}

	prometheusPort, err := utils.TcpPortNextFree(9090)
	if err != nil {
		return fmt.Errorf("failed to assign port to prometheus: %w", err)
	}
	p.ports = prometheusInstallerPorts{prometheus: prometheusPort}
	p.logger.Info("Assigned prometheus port", slog.Int("port", prometheusPort))

	configPath := filepath.Join(prometheusHome, "prometheus.yml")
	if err := os.WriteFile(configPath, []byte(RenderPrometheusConfig("/etc/prometheus/targets")), 0644); err != nil {
		return fmt.Errorf("failed to write prometheus config: %w", err)
	}

	if err := p.installContainer(GetHostPrometheusContainer(p.ports)); err != nil {
		return err
	}

	manager := &SystemdServiceManager{
		ContainerEngine: p.Platform,
		ServiceDir:      getSystemdServiceDir(),
	}
	if err := manager.CreatePrometheusService(); err != nil {
		return fmt.Errorf("failed to create prometheus systemd service: %w", err)
	}

	if err := WritePrometheusState(prometheusPort); err != nil {
		return err
	}

	p.logger.Info("Host-level prometheus installation completed", slog.Int("port", prometheusPort))
	return nil
}

func (p *PrometheusInstaller) ValidatePrerequisitesForUninstall() error {
	if !p.isContainerRunning("skupper-prometheus") {
		return fmt.Errorf("container \"skupper-prometheus\" is not running in %s; nothing to uninstall", p.Platform)
	}
	if namespaces := installedNetworkObservers(); len(namespaces) > 0 {
		return fmt.Errorf("network observers are still installed (%s); run \"skupper system network-observer --uninstall\" for each namespace first", joinStrings(namespaces))
	}
	return nil
}

func (p *PrometheusInstaller) Uninstall() error {
	p.logger.Info("Uninstalling host-level prometheus")

	manager := &SystemdServiceManager{
		ContainerEngine: p.Platform,
		ServiceDir:      getSystemdServiceDir(),
	}
	if err := manager.RemovePrometheusService(); err != nil {
		p.logger.Warn("Failed to remove prometheus systemd service", slog.Any("error", err))
	}

	const containerName = "skupper-prometheus"
	if p.isContainerRunning(containerName) {
		if err := p.cli.ContainerStop(containerName); err != nil {
			p.logger.Warn("Failed to stop container", slog.String("name", containerName), slog.Any("error", err))
		}
	}
	if err := p.cli.ContainerRemove(containerName); err != nil {
		p.logger.Warn("Failed to remove container", slog.String("name", containerName), slog.Any("error", err))
	}

	prometheusHome := api.GetHostPrometheusHome()
	if err := os.RemoveAll(prometheusHome); err != nil {
		p.logger.Warn("Failed to remove prometheus directory", slog.String("path", prometheusHome), slog.Any("error", err))
	}

	p.logger.Info("Host-level prometheus uninstalled successfully")
	return nil
}

func UninstallPrometheus() error {
	if !IsPrometheusInstalled() {
		return nil
	}
	installer, err := NewPrometheusInstaller()
	if err != nil {
		return err
	}
	return installer.Uninstall()
}

func (p *PrometheusInstaller) installContainer(newContainer container.Container) error {
	ctx, cn := context.WithTimeout(context.Background(), time.Minute*10)
	defer cn()
	if err := p.cli.ImagePull(ctx, newContainer.Image); err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}
	fmt.Printf("Pulled image: %s\n", newContainer.Image)
	if err := p.cli.ContainerCreate(&newContainer); err != nil {
		return fmt.Errorf("failed to create container %s: %v", newContainer.Name, err)
	}
	if err := p.cli.ContainerStart(newContainer.Name); err != nil {
		return fmt.Errorf("failed to start container %s: %v", newContainer.Name, err)
	}
	return nil
}
