package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bundle"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/compat"
	"github.com/skupperproject/skupper/pkg/nonkube/systemd"
	"github.com/skupperproject/skupper/pkg/version"
)

var (
	platform      = config.GetPlatform()
	inputPath     string
	userNamespace string
)

const (
	description = `
Bootstraps a nonkube Skupper site base on the provided flags.
When the path (-p) flag is provided, it will be used as the source
directory containing the Skupper custom resources to be processed,
generating a local Skupper site using the "default" namespace, unless
a namespace is set in the custom resources, or if the namespace (-n)
flag is provided.

A namespace is just a directory in the file system where site specific
files are stored, like certificates, configurations, the original sources
(original custom resources used to bootstrap the nonkube site) and
the runtime files generated during initialization.

In case the path (-p) flag is omitted, Skupper will try to process
custom resources stored at the sources directory of the default namespace,
or from the namespace provided through the namespace (-n) flag.
`
)

func main() {
	// if -version used, report and exit
	flag.Usage = func() {
		fmt.Println("Skupper bootstrap")
		fmt.Printf("%s\n", description)
		fmt.Printf("Usage:\n  %s [options...]\n\n", os.Args[0])
		fmt.Printf("Flags:\n")
		flag.PrintDefaults()
	}
	flag.StringVar(&inputPath, "p", "", "Custom resources location on the file system")
	flag.StringVar(&userNamespace, "n", "", "The target namespace used for installation")
	force := flag.Bool("f", false, "Forces to overwrite an existing namespace")
	isVersion := flag.Bool("v", false, "Report the version of the Skupper bootstrap command")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}
	// if user overrides and force empty, use it as default
	if inputPath != "" {
		var err error
		inputPath, err = filepath.Abs(inputPath)
		if err != nil {
			log.Fatalf("Unable to determine absolute path of %s: %v", inputPath, err)
		}
	}
	//
	// NOTE FOR CONTAINERS
	// When running bootstrap process through a container
	// the /input path must be mapped to a directory containing a site
	// definition based on CR files.
	// It also expects the /output path to be mapped to the
	// Host's XDG_DATA_HOME/skupper or $HOME/.local/share/skupper (non-root)
	// and /usr/local/share/skupper (root).
	//
	fmt.Printf("Skupper nonkube bootstrap (version: %s)\n", version.Version)

	if platform.IsKubernetes() {
		// Bootstrap uses podman as the default platform
		_ = os.Setenv(types.ENV_PLATFORM, "podman")
	}
	if api.IsRunningInContainer() {
		inputPath = "/input"
		outputPath := "/output"
		for _, directory := range []string{inputPath, outputPath} {
			stat, err := os.Stat(directory)
			if err != nil {
				fmt.Printf("Failed to stat %s: %s\n", directory, err)
				os.Exit(1)
			}
			if !stat.IsDir() {
				fmt.Printf("%s is not a directory\n", directory)
				os.Exit(1)
			}
		}
	} else if !platform.IsBundle() {
		binary := "podman"
		if platform == types.PlatformSystemd {
			binary = "skrouterd"
		} else if platform == types.PlatformDocker {
			binary = "docker"
		}
		_, err := exec.LookPath(binary)
		if err != nil {
			fmt.Printf("Platform %q is not available.\n", platform)
			fmt.Printf("ERROR! Command not found: %s.\n", binary)
			os.Exit(1)
		}
	}
	namespace := userNamespace
	if namespace == "" {
		namespace = "default"
	}
	if inputPath == "" {
		// when input path is empty, but a namespace is provided, try to reload an existing site definition
		existingPath := api.GetInternalOutputPath(namespace, api.LoadedSiteStatePath)
		if _, err := os.Stat(existingPath); err == nil {
			inputPath = existingPath
			fmt.Printf("Sources will consumed from namespace %q\n", namespace)
		} else {
			fmt.Printf("Input path has not been provided and namespace %s does not exist\n", namespace)
			os.Exit(1)
		}
	}

	// if namespace already exists, fail if force is not set
	_, err := os.Stat(api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath))
	if !platform.IsBundle() && err == nil && !*force {
		fmt.Printf("Namespace already exists: %s\n", namespace)
		os.Exit(1)
	}

	siteState, err := bootstrap(inputPath, userNamespace)
	if err != nil {
		fmt.Println("Failed to bootstrap:", err)
		os.Exit(1)
	}
	var bundleSuffix string
	if platform.IsBundle() {
		bundleSuffix = " (as a distributable bundle)"
	} else {
		bundleSuffix = fmt.Sprintf(" on namespace %q", siteState.GetNamespace())
	}
	fmt.Printf("Site %q has been created%s\n", siteState.Site.Name, bundleSuffix)
	if !platform.IsBundle() {
		tokenPath := api.GetInternalOutputPath(siteState.Site.Namespace, api.RuntimeTokenPath)
		hostTokenPath, err := api.GetHostSiteInternalPath(siteState.Site, api.RuntimeTokenPath)
		if err != nil {
			fmt.Println("Failed to get site's static links path:", err)
		}
		tokens, _ := os.ReadDir(tokenPath)
		for _, token := range tokens {
			if !token.IsDir() {
				fmt.Println("Static links have been defined at:", hostTokenPath)
				break
			}
		}
	} else {
		siteHome, err := api.GetHostBundlesPath()
		if err != nil {
			fmt.Println("Failed to get site bundle base directory:", err)
		}
		installationFile := path.Join(siteHome, fmt.Sprintf("skupper-install-%s.sh", siteState.Site.Name))
		if platform.IsTarball() {
			installationFile = path.Join(siteHome, fmt.Sprintf("skupper-install-%s.tar.gz", siteState.Site.Name))
		}
		fmt.Println("Installation bundle available at:", installationFile)
		fmt.Println("Default namespace:", siteState.GetNamespace())
	}
}

func bootstrap(inputPath string, namespace string) (*api.SiteState, error) {
	var siteStateLoader api.SiteStateLoader
	var reloadExisting bool
	if !platform.IsBundle() && inputPath == api.GetInternalOutputPath(namespace, api.LoadedSiteStatePath) {
		reloadExisting = true
		nsPlatformLoader := &common.NamespacePlatformLoader{}
		nsPlatform, err := nsPlatformLoader.Load(namespace)
		if err != nil {
			return nil, err
		}
		currentPlatform := string(platform)
		if platform.IsKubernetes() {
			currentPlatform = "podman"
		}
		if nsPlatform != currentPlatform {
			return nil, fmt.Errorf("existing namespace uses %q platform and it cannot change to %q", nsPlatform, currentPlatform)
		}
	}
	siteStateLoader = &common.FileSystemSiteStateLoader{
		Path:      inputPath,
		Namespace: namespace,
		Bundle:    platform.IsBundle(),
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load site state: %v", err)
	}

	var siteStateRenderer api.StaticSiteStateRenderer
	if platform == types.PlatformSystemd {
		siteStateRenderer = &systemd.SiteStateRenderer{}
	} else if platform.IsBundle() {
		siteStateRenderer = &bundle.SiteStateRenderer{}
	} else {
		siteStateRenderer = &compat.SiteStateRenderer{}
	}
	err = siteStateRenderer.Render(siteState, reloadExisting)
	if err != nil {
		return nil, fmt.Errorf("failed to render site state: %v", err)
	}
	return siteState, nil
}
