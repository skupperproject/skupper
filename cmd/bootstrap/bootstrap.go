package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"github.com/skupperproject/skupper/api/types"
	internalbundle "github.com/skupperproject/skupper/internal/nonkube/bundle"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/bundle"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/compat"
	"github.com/skupperproject/skupper/pkg/nonkube/systemd"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/pkg/version"
)

var (
	platform       = config.GetPlatform()
	inputPath      string
	userNamespace  string
	bundleStrategy string
)

const (
	description = `
Bootstraps a nonkube Skupper site base on the provided flags.

When the path (-p) flag is provided, it will be used as the source
directory containing the Skupper custom resources to be processed,
generating a local Skupper site using the "default" namespace, unless
a namespace is set in the custom resources, or if the namespace (-n)
flag is provided.

A namespace is just a directory in the file system where all site specific
files are stored, like certificates, configurations, the original sources
(original custom resources used to bootstrap the nonkube site) and
the runtime files generated during initialization.

Namespaces are stored under ${XDG_DATA_HOME}/.local/share/skupper/namespaces
for regular users when XDG_DATA_HOME environment variable is set, or under
${HOME}/.local/share/skupper/namespaces when it is not set.
As the root user, namespaces are stored under: /usr/local/share/skupper/namespaces.

In case the path (-p) flag is omitted, Skupper will try to process
custom resources stored at the sources directory of the default namespace,
or from the namespace provided through the namespace (-n) flag.

If the respective namespace already exists and you want to bootstrap it
over, you must provide the force (-f) flag. When you do that, the existing
Certificate Authorities (CAs) are preserved, so eventual existing incoming
links should be able to reconnect.

To produce a bundle, instead of rendering a site, the bundle strategy (-b)
flag must be set to "bundle" or "tarball".
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
	flag.StringVar(&bundleStrategy, "b", "", "The bundle strategy to be produced: bundle or tarball")
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
			fmt.Printf("Unable to determine absolute path of %s: %v\n", inputPath, err)
			os.Exit(1)
		}
	}
	if bundleStrategy != "" {
		if !internalbundle.IsValidBundle(bundleStrategy) {
			fmt.Printf("Invalid bundle strategy: %s\n", bundleStrategy)
			os.Exit(1)
		}
	}
	//
	// NOTE FOR CONTAINERS
	// When running bootstrap process through a container
	// the /input path must be mapped to a directory containing a site
	// definition based on CR files, if an input path has been provided.
	// The /output path must be mapped to the Host's XDG_DATA_HOME/skupper
	// or $HOME/.local/share/skupper (non-root)
	// and /usr/local/share/skupper (root).
	//
	fmt.Printf("Skupper nonkube bootstrap (version: %s)\n", version.Version)

	if platform.IsKubernetes() {
		platform = types.PlatformPodman
		// Bootstrap uses podman as the default platform
		//_ = os.Setenv(types.ENV_PLATFORM, "podman")
	}
	isBundle := internalbundle.GetBundleStrategy(bundleStrategy) != ""
	if api.IsRunningInContainer() {
		requiredPaths := []string{"/output"}
		if inputPath != "" {
			if inputPath != "/input" {
				fmt.Println("The input path must be set to /input when using a container to bootstrap")
				os.Exit(1)
			}
			requiredPaths = append(requiredPaths, "/input")
		}
		for _, directory := range requiredPaths {
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
	} else if !isBundle {
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
		existingPath := api.GetInternalOutputPath(namespace, api.InputSiteStatePath)
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
	if !isBundle && err == nil && !*force {
		fmt.Printf("Namespace already exists: %s\n", namespace)
		os.Exit(1)
	}

	siteState, err := bootstrap(inputPath, userNamespace, internalbundle.GetBundleStrategy(bundleStrategy))
	if err != nil {
		fmt.Println("Failed to bootstrap:", err)
		os.Exit(1)
	}
	var bundleSuffix string
	if isBundle {
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
	if !isBundle {
		fmt.Printf("Platform: %s\n", platform)
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
		sourcesPath, _ := api.GetHostSiteInternalPath(siteState.Site, api.InputSiteStatePath)
		fmt.Printf("Definition is available at: %s\n", sourcesPath)
	} else {
		siteHome, err := api.GetHostBundlesPath()
		if err != nil {
			fmt.Println("Failed to get site bundle base directory:", err)
		}
		installationFile := path.Join(siteHome, fmt.Sprintf("skupper-install-%s.sh", siteState.Site.Name))
		if internalbundle.GetBundleStrategy(bundleStrategy) == string(internalbundle.BundleStrategyTarball) {
			installationFile = path.Join(siteHome, fmt.Sprintf("skupper-install-%s.tar.gz", siteState.Site.Name))
		}
		fmt.Println("Installation bundle available at:", installationFile)
		fmt.Println("Default namespace:", siteState.GetNamespace())
		fmt.Println("Default platform:", utils.DefaultStr(string(platform), "podman"))
	}
}

func bootstrap(inputPath string, namespace string, bundleStrategy string) (*api.SiteState, error) {
	var siteStateLoader api.SiteStateLoader
	var reloadExisting bool
	isBundle := bundleStrategy != ""
	sourcesPath := api.GetInternalOutputPath(namespace, api.InputSiteStatePath)
	_, err := os.Stat(sourcesPath)
	if !isBundle && err == nil {
		reloadExisting = true
		nsPlatformLoader := &common.NamespacePlatformLoader{}
		nsPlatform, err := nsPlatformLoader.Load(namespace)
		if err != nil {
			_, runtimeStateErr := os.Stat(api.GetInternalOutputPath(namespace, api.RuntimeSiteStatePath))
			if runtimeStateErr == nil {
				return nil, fmt.Errorf("unable to determine current platform used in namespace %q", namespace)
			}
			// platform.yaml not present, which is ok if a site is not yet rendered
			nsPlatform = string(platform)
			reloadExisting = false
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
		Path:   inputPath,
		Bundle: isBundle,
	}
	siteState, err := siteStateLoader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load site state: %v", err)
	}
	// if sources are being consume from namespace sources, they must be properly set
	crNamespace := siteState.GetNamespace()
	targetNamespace := utils.DefaultStr(namespace, "default")
	if inputPath == sourcesPath {
		if crNamespace != targetNamespace {
			return nil, fmt.Errorf("namespace must be %q, but sources are defined using %q", targetNamespace, crNamespace)
		}
	} else if namespace != "" {
		siteState.SetNamespace(namespace)
	}

	var siteStateRenderer api.StaticSiteStateRenderer
	if isBundle {
		siteStateRenderer = &bundle.SiteStateRenderer{
			Strategy: internalbundle.BundleStrategy(bundleStrategy),
			Platform: platform,
		}
	} else if platform == types.PlatformSystemd {
		siteStateRenderer = &systemd.SiteStateRenderer{}
	} else {
		siteStateRenderer = &compat.SiteStateRenderer{
			Platform: platform,
		}
	}
	err = siteStateRenderer.Render(siteState, reloadExisting)
	if err != nil {
		return nil, fmt.Errorf("failed to render site state: %v", err)
	}
	return siteState, nil
}
