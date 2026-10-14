package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/user"

	"github.com/skupperproject/skupper/api/types"
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type CheckDrift func(string) (bool, error)
type CheckRouterDrift func(namespace string) (bool, error)
type BootstrapSite func(cfg *Config) (*api.SiteState, error)

func Upgrade(platform string) error {
	controllerDrifted, reloadType, err := CheckControllerImageDrift()
	if err != nil {
		return fmt.Errorf("upgrade: controller check: %w", err)
	}

	routersDrifted, err := anyRouterDriftedWith(CheckRouterImageDrift)
	if err != nil {
		return fmt.Errorf("upgrade: router check: %w", err)
	}

	if !controllerDrifted && !routersDrifted {
		fmt.Println("System is already up to date, nothing to upgrade")
		return nil
	}

	var backupContainerNames []string
	var routerBackups map[string]*container.Container
	if routersDrifted {
		routerBackups, backupContainerNames, err = backupRouterContainers(CheckRouterImageDrift)
		if err != nil {
			return restore("failed to back up router containers", err, routerBackups, nil, "")
		}
	}

	var controllerBackup *container.Container
	var controllerBackupName string
	if controllerDrifted {
		controllerBackup, controllerBackupName, err = backupController()
		if err != nil {
			return restore("failed to back up controller", err, routerBackups, controllerBackup, controllerBackupName)
		}

		backupContainerNames = append(backupContainerNames, controllerBackupName)
	}

	if routersDrifted {
		if routerErr := upgradeNamespaces(platform, routerBackups, Bootstrap); routerErr != nil {
			return restore("router upgrades failed", routerErr, routerBackups, controllerBackup, controllerBackupName)
		}
	}

	if controllerDrifted {
		if installErr := Install(platform, reloadType); installErr != nil {
			return restore("controller install failed", installErr, routerBackups, controllerBackup, controllerBackupName)
		}
	}

	removeBackupContainers(backupContainerNames)
	return nil
}

func restore(cause string, origErr error, routerBackups map[string]*container.Container, controllerBackup *container.Container, controllerBackupName string) error {
	slog.Error("upgrade: "+cause+", restoring previous state", slog.Any("error", origErr))
	if restoreErr := restoreRouterContainers(routerBackups); restoreErr != nil {
		slog.Error("upgrade: failed to restore router containers", slog.Any("error", restoreErr))
	}
	if controllerBackup != nil {
		if restoreErr := restoreController(controllerBackupName); restoreErr != nil {
			slog.Error("upgrade: failed to restore controller", slog.Any("error", restoreErr))
		}
	}
	return fmt.Errorf("upgrade: %s: %w", cause, origErr)
}

func backupRouterContainers(checkDrift CheckRouterDrift) (map[string]*container.Container, []string, error) {
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	entries, err := os.ReadDir(namespacesPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read namespaces directory: %w", err)
	}

	cli, err := internalclient.NewCompatClient(internalclient.GetDefaultContainerEndpoint(), "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create container client: %w", err)
	}

	backups := make(map[string]*container.Container)
	var backupContainerNames []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ns := entry.Name()
		drifted, err := checkDrift(ns)
		if err != nil || !drifted {
			continue
		}
		containerName := ns + "-skupper-router"
		backupName := fmt.Sprintf("%s-backup", containerName)
		c, err := cli.ContainerInspect(containerName)
		if err != nil || c == nil {
			continue
		}
		// Remove any leftover backup from a previous failed upgrade attempt
		if old, inspectErr := cli.ContainerInspect(backupName); inspectErr == nil && old != nil {
			_ = cli.ContainerStop(backupName)
			if removeErr := cli.ContainerRemove(backupName); removeErr != nil {
				return nil, nil, fmt.Errorf("namespace %q: failed to remove stale backup container: %w", ns, removeErr)
			}
		}
		if err := cli.ContainerStop(containerName); err != nil {
			return backups, backupContainerNames, fmt.Errorf("namespace %q: failed to stop router container: %w", ns, err)
		}
		if err := cli.ContainerRename(containerName, backupName); err != nil {
			// The container was stopped but rename failed; restart it so it is
			// not left dead under its original name and unreachable by restore.
			if startErr := cli.ContainerStart(containerName); startErr != nil {
				slog.Error("upgrade: failed to restart router container after rename failure",
					slog.String("namespace", ns), slog.Any("error", startErr))
			}
			return backups, backupContainerNames, fmt.Errorf("namespace %q: failed to rename router container to backup: %w", ns, err)
		}
		backups[ns] = c
		backupContainerNames = append(backupContainerNames, backupName)
	}
	return backups, backupContainerNames, nil
}

func restoreRouterContainers(backups map[string]*container.Container) error {
	if len(backups) == 0 {
		return nil
	}
	cli, err := internalclient.NewCompatClient(internalclient.GetDefaultContainerEndpoint(), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}
	var errs []error
	for ns := range backups {
		containerName := ns + "-skupper-router"
		backupName := fmt.Sprintf("%s-backup", containerName)
		// Remove the upgraded container if it exists (Bootstrap may have
		// created it before the overall upgrade failed), so the rename below
		// does not hit a UNIQUE constraint on the container name.
		if existing, inspectErr := cli.ContainerInspect(containerName); inspectErr == nil && existing != nil {
			_ = cli.ContainerStop(containerName)
			if removeErr := cli.ContainerRemove(containerName); removeErr != nil {
				errs = append(errs, fmt.Errorf("namespace %q: failed to remove upgraded router container: %w", ns, removeErr))
				continue
			}
		}
		if err := cli.ContainerRename(backupName, containerName); err != nil {
			errs = append(errs, fmt.Errorf("namespace %q: failed to rename backup router back to %q: %w", ns, containerName, err))
			continue
		}
		if err := cli.ContainerStart(containerName); err != nil {
			errs = append(errs, fmt.Errorf("namespace %q: failed to restart router container: %w", ns, err))
		}
	}
	return errors.Join(errs...)
}

func backupController() (*container.Container, string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get current user: %w", err)
	}
	containerName := fmt.Sprintf("%s-skupper-controller", currentUser.Username)
	backupName := fmt.Sprintf("%s-backup", containerName)

	cli, err := internalclient.NewCompatClient(internalclient.GetDefaultContainerEndpoint(), "")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create container client: %w", err)
	}

	c, err := cli.ContainerInspect(containerName)
	if err != nil || c == nil {
		return nil, "", nil
	}

	if err := cli.ContainerStop(containerName); err != nil {
		return nil, "", fmt.Errorf("failed to stop controller container: %w", err)
	}
	if err := cli.ContainerRename(containerName, backupName); err != nil {
		// The controller was stopped but rename failed; restart it so it is
		// not left dead under its original name and unreachable by restore.
		if startErr := cli.ContainerStart(containerName); startErr != nil {
			slog.Error("upgrade: failed to restart controller container after rename failure",
				slog.Any("error", startErr))
		}
		return nil, "", fmt.Errorf("failed to rename controller container to backup: %w", err)
	}

	return c, backupName, nil
}

func restoreController(backupName string) error {
	currentUser, err := user.Current()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	originalName := fmt.Sprintf("%s-skupper-controller", currentUser.Username)

	cli, err := internalclient.NewCompatClient(internalclient.GetDefaultContainerEndpoint(), "")
	if err != nil {
		return fmt.Errorf("failed to create container client: %w", err)
	}

	// Remove the partially created controller if Install created it before
	// failing, so the rename below does not hit a name collision.
	if existing, inspectErr := cli.ContainerInspect(originalName); inspectErr == nil && existing != nil {
		_ = cli.ContainerStop(originalName)
		if removeErr := cli.ContainerRemove(originalName); removeErr != nil {
			return fmt.Errorf("failed to remove partially created controller container: %w", removeErr)
		}
	}
	if err := cli.ContainerRename(backupName, originalName); err != nil {
		return fmt.Errorf("failed to rename backup controller back to %q: %w", originalName, err)
	}
	if err := cli.ContainerStart(originalName); err != nil {
		return fmt.Errorf("failed to restart controller container: %w", err)
	}

	return nil
}

func removeBackupContainers(backupNames []string) {
	if len(backupNames) == 0 {
		return
	}
	cli, err := internalclient.NewCompatClient(internalclient.GetDefaultContainerEndpoint(), "")
	if err != nil {
		slog.Warn("upgrade: failed to create client to remove backup container", slog.Any("error", err))
		return
	}
	for _, name := range backupNames {
		if err := cli.ContainerRemove(name); err != nil {
			slog.Error("upgrade: failed to remove backup container",
				slog.String("name", name), slog.Any("error", err))
		}
	}

}

func anyRouterDriftedWith(checkDrift CheckDrift) (bool, error) {
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	entries, err := os.ReadDir(namespacesPath)
	if err != nil {
		return false, fmt.Errorf("anyRouterDrifted: failed to read namespaces directory: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		drifted, err := checkDrift(entry.Name())
		if err != nil {
			return false, fmt.Errorf("router check for %q: %w", entry.Name(), err)
		}
		if drifted {
			return true, nil
		}
	}
	return false, nil
}

func upgradeNamespaces(platform string, namespaces map[string]*container.Container, bootstrapSite BootstrapSite) error {
	var errs []error
	for ns := range namespaces {
		slog.Info("upgrade: re-bootstrapping router", slog.String("namespace", ns))
		cfg := &Config{
			Namespace: ns,
			Platform:  types.Platform(platform),
			InputPath: api.GetInternalOutputPath(ns, api.InputSiteStatePath),
		}
		if _, err := bootstrapSite(cfg); err != nil {
			slog.Warn("upgrade: failed to upgrade router",
				slog.String("namespace", ns), slog.Any("error", err))
			errs = append(errs, fmt.Errorf("namespace %q: %w", ns, err))
			continue
		}

	}
	return errors.Join(errs...)
}
