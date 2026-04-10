package bootstrap

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/skupperproject/skupper/api/types"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/nonkube/compat"
	"github.com/skupperproject/skupper/internal/nonkube/linux"
	"github.com/skupperproject/skupper/internal/utils"
	internalutils "github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type Config struct {
	InputPath      string
	Namespace      string
	BundleStrategy string
	BundleName     string
	IsBundle       bool
	Platform       types.Platform
	Binary         string
}

func PreBootstrap(config *Config) error {

	existingPath := api.GetInternalOutputPath(config.Namespace, api.InputSiteStatePath)
	inputSourcesDefined := false
	if _, err := os.Stat(existingPath); err == nil {
		dirReader := new(internalutils.DirectoryReader)
		filesFound, _ := dirReader.ReadDir(existingPath, nil)
		inputSourcesDefined = len(filesFound) > 0
	}
	if config.InputPath == "" {
		// when input path is empty, but a namespace is provided, try to reload an existing site definition
		if inputSourcesDefined {
			config.InputPath = existingPath
			fmt.Printf("Sources will be consumed from namespace %q\n", config.Namespace)
		} else {
			fmt.Printf("Namespace %q does not exist\n", config.Namespace)
			return fmt.Errorf("No sources found at: %s\n", path.Join(api.GetHostNamespaceHome(config.Namespace), string(api.InputSiteStatePath)))
		}
	} else if inputSourcesDefined && !api.IsRunningInContainer() {
		return fmt.Errorf("Input path has been provided, but namespace %s has input sources defined at:\n %s\n", config.Namespace, path.Join(api.GetHostNamespaceHome(config.Namespace), string(api.InputSiteStatePath)))
	}

	if api.IsRunningInContainer() {
		requiredPaths := []string{"/output"}
		if config.InputPath != "" {
			requiredPaths = append(requiredPaths, "/input")
		}
		for _, directory := range requiredPaths {
			stat, err := os.Stat(directory)
			if err != nil {
				return fmt.Errorf("Failed to stat %s: %s\n", directory, err)
			}
			if !stat.IsDir() {
				return fmt.Errorf("%s is not a directory\n", directory)
			}
		}
	} else if !config.IsBundle {

		_, err := exec.LookPath(config.Binary)
		if err != nil {
			return fmt.Errorf("Platform %q is not available:\n ERROR! Command not found: %s", config.Platform, config.Binary)
		}
	}

	return nil
}

func Bootstrap(config *Config) (*api.SiteState, error) {
	var siteStateLoader api.SiteStateLoader
	var reloadExisting bool

	sourcesPath := api.GetInternalOutputPath(config.Namespace, api.InputSiteStatePath)
	_, err := os.Stat(sourcesPath)
	if !config.IsBundle && err == nil {
		reloadExisting = true
		nsPlatformLoader := &common.NamespacePlatformLoader{}
		nsPlatform, err := nsPlatformLoader.Load(config.Namespace)
		if err != nil {
			_, runtimeStateErr := os.Stat(api.GetInternalOutputPath(config.Namespace, api.RuntimeSiteStatePath))
			if runtimeStateErr == nil {
				return nil, fmt.Errorf("unable to determine current platform used in namespace %q", config.Namespace)
			}
			// platform.yaml not present, which is ok if a site is not yet rendered
			nsPlatform = string(config.Platform)
			reloadExisting = false
		}
		currentPlatform := string(config.Platform)
		if config.Platform.IsKubernetes() {
			currentPlatform = "podman"
		}
		if nsPlatform != currentPlatform {
			return nil, fmt.Errorf("existing namespace uses %q platform and it cannot change to %q", nsPlatform, currentPlatform)
		}
	}
	siteStateLoader = &common.FileSystemSiteStateLoader{
		Path:   config.InputPath,
		Bundle: config.IsBundle,
	}

	siteState, err := siteStateLoader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load site state: %v", err)
	}
	// if sources are being consumed from namespace sources, they must be properly set
	crNamespace := siteState.GetNamespace()
	targetNamespace := utils.DefaultStr(config.Namespace, "default")
	if config.InputPath == sourcesPath {
		if crNamespace != targetNamespace {
			return nil, fmt.Errorf("namespace must be %q, but sources are defined using %q", targetNamespace, crNamespace)
		}
	} else if config.Namespace != "" {
		siteState.SetNamespace(config.Namespace)
	}

	var siteStateRenderer api.StaticSiteStateRenderer
	if config.IsBundle {
		siteStateRenderer = &internalbundle.SiteStateRenderer{
			Strategy: internalbundle.BundleStrategy(config.BundleStrategy),
			Platform: config.Platform,
			FileName: config.BundleName,
		}
	} else if config.Platform == types.PlatformLinux {
		siteStateRenderer = &linux.SiteStateRenderer{}
	} else {
		siteStateRenderer = &compat.SiteStateRenderer{
			Platform: config.Platform,
		}
	}
	err = siteStateRenderer.Render(siteState, reloadExisting)
	if err != nil {
		return nil, fmt.Errorf("failed to render site state: %v", err)
	}
	return siteState, nil
}

func PostBootstrap(config *Config, siteState *api.SiteState) {
	var bundleSuffix string
	if config.IsBundle {
		bundleSuffix = " (as a distributable bundle)"
	} else {
		bundleSuffix = fmt.Sprintf(" on namespace %q", siteState.GetNamespace())
		// create bootstrap.out file
		if api.IsRunningInContainer() {
			outFile, err := os.Stat("/bootstrap.out")
			if err == nil && !outFile.IsDir() {
				err = os.WriteFile("/bootstrap.out", []byte(siteState.GetNamespace()), 0644)
				if err != nil {
					fmt.Println("Failed to write to bootstrap.out:", err)
					fmt.Println("The systemd service will not be created.")
				}
			}
		}
	}
	fmt.Printf("Site %q has been created%s\n", siteState.Site.Name, bundleSuffix)
	if !config.IsBundle {
		fmt.Printf("Platform: %s\n", config.Platform)
		tokenPath := api.GetInternalOutputPath(siteState.Site.Namespace, api.RuntimeTokenPath)
		hostTokenPath := api.GetHostSiteInternalPath(siteState.Site, api.RuntimeTokenPath)
		tokens, _ := os.ReadDir(tokenPath)
		for _, token := range tokens {
			if !token.IsDir() {
				fmt.Println("Static links have been defined at:", hostTokenPath)
				break
			}
		}
		sourcesPath := api.GetHostSiteInternalPath(siteState.Site, api.InputSiteStatePath)
		fmt.Printf("Definition is available at: %s\n", sourcesPath)
	} else {
		siteHome := api.GetHostBundlesPath()
		installationFile := path.Join(siteHome, fmt.Sprintf("%s.sh", config.BundleName))
		if internalbundle.GetBundleStrategy(config.BundleStrategy) == string(internalbundle.BundleStrategyTarball) {
			installationFile = path.Join(siteHome, fmt.Sprintf("%s.tar.gz", config.BundleName))
		}
		fmt.Println("Installation bundle available at:", installationFile)
		fmt.Println("Default namespace:", siteState.GetNamespace())
		fmt.Println("Default platform:", string(config.Platform))
	}
}
