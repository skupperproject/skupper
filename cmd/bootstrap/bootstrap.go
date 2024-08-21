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

func main() {
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of the Skupper bootstrap command")
	flag.StringVar(&inputPath, "path", "", "Custom resources location on the file system")
	flag.StringVar(&userNamespace, "namespace", "", "The target namespace used for installation (overrides the namespace from custom resources when --path is provided)")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
	}
	// if user overrides and force empty, use it as default
	namespace := userNamespace
	if namespace == "" {
		namespace = "default"
	}
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
	if inputPath == "" && userNamespace == "" {
		fmt.Printf("No input path or namespace specified\n")
		os.Exit(1)
	} else if inputPath == "" && userNamespace != "" {
		// when input path is empty, but a namespace is provided, try to reload an existing site definition
		existingPath := api.GetInternalOutputPath(userNamespace, api.LoadedSiteStatePath)
		if _, err := os.Stat(existingPath); err == nil {
			inputPath = existingPath
		} else {
			fmt.Printf("Namespace %q not found\n", userNamespace)
			os.Exit(1)
		}
	}

	siteState, err := bootstrap(inputPath, namespace)
	if err != nil {
		fmt.Println("Failed to bootstrap:", err)
		os.Exit(1)
	}
	var bundleSuffix string
	if platform.IsBundle() {
		bundleSuffix = " (as a distributable bundle)"
	} else {
		bundleSuffix = fmt.Sprintf(" on namespace %q", namespace)
	}
	fmt.Printf("Site %q has been created%s\n", siteState.Site.Name, bundleSuffix)
	if !platform.IsBundle() {
		tokenPath := api.GetInternalOutputPath(siteState.Site.Namespace, api.RuntimeTokenPath)
		hostTokenPath, err := api.GetHostSiteInternalPath(siteState.Site, api.RuntimeTokenPath)
		if err != nil {
			fmt.Println("Failed to get site's token path:", err)
		}
		tokens, _ := os.ReadDir(tokenPath)
		for _, token := range tokens {
			if !token.IsDir() {
				fmt.Println("Static tokens have been defined at:", hostTokenPath)
				break
			}
		}
	} else {
		siteHome, err := api.GetHostNamespacesPath()
		if err != nil {
			fmt.Println("Failed to get site bundle base directory:", err)
		}
		installationFile := path.Join(siteHome, fmt.Sprintf("skupper-install-%s.sh", siteState.Site.Name))
		if platform.IsTarball() {
			installationFile = path.Join(siteHome, fmt.Sprintf("skupper-install-%s.tar.gz", siteState.Site.Name))
		}
		fmt.Println("Installation bundle available at:", installationFile)
	}
}

func bootstrap(inputPath string, namespace string) (*api.SiteState, error) {
	var siteStateLoader api.SiteStateLoader
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
	err = siteStateRenderer.Render(siteState)
	if err != nil {
		return nil, fmt.Errorf("failed to render site state: %v", err)
	}
	return siteState, nil
}
