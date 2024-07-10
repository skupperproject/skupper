package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/non_kube/apis"
	"github.com/skupperproject/skupper/pkg/non_kube/bundle"
	"github.com/skupperproject/skupper/pkg/non_kube/common"
	"github.com/skupperproject/skupper/pkg/non_kube/compat"
	"github.com/skupperproject/skupper/pkg/non_kube/systemd"
	"github.com/skupperproject/skupper/pkg/version"
)

var (
	platform = config.GetPlatform()
)

func main() {
	// if -version used, report and exit
	isVersion := flag.Bool("version", false, "Report the version of the Skupper bootstrap command")
	flag.Parse()
	if *isVersion {
		fmt.Println(version.Version)
		os.Exit(0)
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

	var inputPath string
	var outputPath string
	inputPath = flag.Arg(0)

	if platform.IsKubernetes() {
		// Bootstrap uses podman as the default platform
		_ = os.Setenv(types.ENV_PLATFORM, "podman")
	}
	if apis.IsRunningInContainer() {
		inputPath = "/input"
		outputPath = "/output"
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
	// TODO defined standard places for input path?
	if inputPath == "" {
		fmt.Printf("No input path specified\n")
		os.Exit(1)
	}

	siteState, err := bootstrap(inputPath)
	if err != nil {
		fmt.Println("Failed to bootstrap:", err)
		os.Exit(1)
	}
	var bundleSuffix string
	if platform.IsBundle() {
		bundleSuffix = " (as a distributable bundle)"
	}
	fmt.Printf("Site %q has been created%s\n", siteState.Site.Name, bundleSuffix)
	if !platform.IsBundle() {
		siteHome, err := apis.GetHostSiteHome(siteState.Site)
		if err != nil {
			fmt.Println("Failed to get site's home directory:", err)
		} else {
			tokenPath := path.Join(siteHome, common.RuntimeTokenPath)
			hostTokenPath := tokenPath
			if apis.IsRunningInContainer() {
				tokenPath = path.Join("/output", "sites", siteState.Site.Name, common.RuntimeTokenPath)
			}
			tokens, _ := os.ReadDir(tokenPath)
			for _, token := range tokens {
				if !token.IsDir() {
					fmt.Println("Static tokens have been defined at:", hostTokenPath)
					break
				}
			}
		}
	}
}

func bootstrap(inputPath string) (*apis.SiteState, error) {
	var siteStateLoader apis.SiteStateLoader
	siteStateLoader = &common.FileSystemSiteStateLoader{
		Path:   inputPath,
		Bundle: platform.IsBundle(),
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load site state: %v", err)
	}
	var siteStateRenderer apis.StaticSiteStateRenderer
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
