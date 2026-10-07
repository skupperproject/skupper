package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type CheckDrift func(string) (bool, error)

func Upgrade(platform string, reloadType string) error {
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

	if controllerDrifted {
		if err := Uninstall(platform); err != nil {
			return fmt.Errorf("upgrade: failed to remove existing controller: %w", err)
		}
	}

	if routersDrifted {
		if err := upgradeRoutersWith(platform, CheckRouterImageDrift, Bootstrap); err != nil {
			return fmt.Errorf("upgrade: router upgrades failed: %w", err)
		}
	}

	if controllerDrifted {
		return Install(platform, reloadType)
	}
	return nil
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

type CheckRouterDrift func(namespace string) (bool, error)
type BootstrapSite func(cfg *Config) (*api.SiteState, error)

func upgradeRoutersWith(platform string, checkRouterDrift CheckRouterDrift, bootstrapSite BootstrapSite) error {
	namespacesPath := api.GetDefaultOutputNamespacesPath()
	entries, err := os.ReadDir(namespacesPath)
	if err != nil {
		return fmt.Errorf("upgradeRouters: failed to read namespaces directory: %w", err)
	}
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ns := entry.Name()
		drifted, err := checkRouterDrift(ns)
		if err != nil || !drifted {
			continue
		}
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
		}
	}
	return errors.Join(errs...)
}
