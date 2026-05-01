package nonkube

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	cliutils "github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
	pkgutils "github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

type CmdDebug struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandDebugFlags
	namespace string
	fileName  string
	platform  string
}

func NewCmdDebug() *CmdDebug {

	skupperCmd := CmdDebug{}

	return &skupperCmd
}

func (cmd *CmdDebug) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}
}

func (cmd *CmdDebug) ValidateInput(args []string) error {
	var validationErrors []error
	fileStringValidator := validator.NewFilePathStringValidator()

	// Validate dump file name
	if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if len(args) == 1 {
		if args[0] == "" {
			validationErrors = append(validationErrors, fmt.Errorf("filename must not be empty"))
		} else {
			ok, err := fileStringValidator.Evaluate(args[0])
			if !ok {
				validationErrors = append(validationErrors, fmt.Errorf("filename is not valid: %s", err))
			} else {
				cmd.fileName = args[0]
			}
		}
	}

	// Validate that a site exists in the namespace
	siteHandler := fs.NewSiteHandler(cmd.namespace)
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	sites, err := siteHandler.List(opts)
	if err != nil || sites == nil || len(sites) == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("no skupper site found in namespace"))
	}

	// Detect platform from namespace config
	platformLoader := &nonkubecommon.NamespacePlatformLoader{}
	platform, err := platformLoader.Load(cmd.namespace)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("failed to detect platform: %w", err))
	} else {
		cmd.platform = platform
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdDebug) InputToOptions() {
	if cmd.namespace == "" {
		cmd.namespace = "default"
	}
	if cmd.fileName == "" {
		cmd.fileName = "skupper-dump"
	}
	datetime := time.Now().Format("20060102150405")
	cmd.fileName = fmt.Sprintf("%s-%s-%s", cmd.fileName, cmd.namespace, datetime)
}

func (cmd *CmdDebug) Run() error {
	dumpFile := cmd.fileName

	// Add extension if not present
	if filepath.Ext(dumpFile) == "" {
		dumpFile = dumpFile + ".tar.gz"
	}

	// Create tarball
	tb := pkgutils.NewTarball()

	// Collect all diagnostic information
	cmd.collectVersionInfo(tb)
	cmd.collectSiteResources(tb)
	cmd.collectRouterConfig(tb)
	cmd.collectCertificates(tb)

	// Platform-specific collection
	if cmd.platform == "linux" {
		cmd.collectSystemdInfo(tb)
		cmd.collectSystemdLogs(tb)
		cmd.collectRouterStatsSystemd(tb)
	} else {
		cmd.collectContainerInfo(tb)
		cmd.collectContainerLogs(tb)
		cmd.collectRouterStatsContainer(tb)
	}

	// Save tarball to file
	err := tb.Save(dumpFile)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	}

	fmt.Println("Skupper dump details written to compressed archive:", dumpFile)
	return nil
}

func (cmd *CmdDebug) collectVersionInfo(tb *pkgutils.Tarball) {
	// Skupper version
	manifest, err := cliutils.RunCommand("skupper", "manifest", "-o", "yaml", "--platform", cmd.platform, "-n", cmd.namespace)
	if err == nil {
		cliutils.WriteTar("/versions/skupper.yaml", manifest, time.Now(), tb)
		cliutils.WriteTar("/versions/skupper.yaml.txt", manifest, time.Now(), tb)
	}

	// Platform version
	var platformVersion []byte
	switch cmd.platform {
	case "podman":
		platformVersion, err = cliutils.RunCommand("podman", "version")
		if err == nil {
			cliutils.WriteTar("/versions/podman.txt", platformVersion, time.Now(), tb)
		}
	case "docker":
		platformVersion, err = cliutils.RunCommand("docker", "version")
		if err == nil {
			cliutils.WriteTar("/versions/docker.txt", platformVersion, time.Now(), tb)
		}
	case "linux":
		// Get systemd version
		platformVersion, err = cliutils.RunCommand("systemctl", "--version")
		if err == nil {
			cliutils.WriteTar("/versions/systemd.txt", platformVersion, time.Now(), tb)
		}
	}

	// Router version (only if skrouterd binary exists on host i.e. systemd sites)
	routerVersion, err := cliutils.RunCommand("skrouterd", "--version")
	if err == nil {
		cliutils.WriteTar("/versions/skrouterd.txt", routerVersion, time.Now(), tb)
	}

	// System information (all platforms)
	unameOutput, err := cliutils.RunCommand("uname", "-a")
	if err == nil {
		cliutils.WriteTar("/versions/uname.txt", unameOutput, time.Now(), tb)
	}

	// OS release information (all platforms)
	osRelease, err := os.ReadFile("/etc/os-release")
	if err == nil {
		cliutils.WriteTar("/versions/os-release.txt", osRelease, time.Now(), tb)
	}
}

// writeObjectToTarball serializes a k8s runtime.Object to YAML and adds it to the tarball
// For Secrets, it removes the tls.key field to avoid storing private keys
func writeObjectToTarball(obj k8sruntime.Object, name string, tb *pkgutils.Tarball) error {
	// Sanitize Secrets - remove tls.key before serialization
	if secret, ok := obj.(*corev1.Secret); ok {
		// Create a copy to avoid modifying the original
		secretCopy := secret.DeepCopy()
		if secretCopy.Data != nil {
			delete(secretCopy.Data, "tls.key")
		}
		obj = secretCopy
	}

	// Serialize to YAML
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, cliutils.GetDebugScheme(), cliutils.GetDebugScheme())
	if err := s.Encode(obj, &b); err != nil {
		return err
	}

	// Write both .yaml and .yaml.txt files
	if err := tb.AddFileData(name+".yaml", 0600, time.Now(), b.Bytes()); err != nil {
		return err
	}
	if err := tb.AddFileData(name+".yaml.txt", 0600, time.Now(), b.Bytes()); err != nil {
		return err
	}
	return nil
}

func (cmd *CmdDebug) collectSiteResources(tb *pkgutils.Tarball) {
	basePath := "/site-namespace/resources/"

	// Load resources from both runtime and input directories
	runtimePath := api.GetInternalOutputPath(cmd.namespace, api.RuntimeSiteStatePath)
	inputPath := api.GetInternalOutputPath(cmd.namespace, api.InputSiteStatePath)

	// Load runtime resources
	runtimeLoader := &nonkubecommon.FileSystemSiteStateLoader{Path: runtimePath}
	runtimeState, err := runtimeLoader.Load()
	if err == nil && runtimeState != nil {
		cmd.writeSiteStateResources(runtimeState, "runtime", basePath, tb)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load runtime site state from %s: %v\n", runtimePath, err)
	}

	// Load input resources
	inputLoader := &nonkubecommon.FileSystemSiteStateLoader{Path: inputPath}
	inputState, err := inputLoader.Load()
	if err == nil && inputState != nil {
		cmd.writeSiteStateResources(inputState, "input", basePath, tb)
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load input site state from %s: %v\n", inputPath, err)
	}

	// Collect raw YAML files to capture any malformed or unrecognized resources
	cmd.collectRawYamlFiles(runtimePath, path.Join(basePath, "raw-yaml/runtime"), tb)
	cmd.collectRawYamlFiles(inputPath, path.Join(basePath, "raw-yaml/input"), tb)
}

func (cmd *CmdDebug) writeSiteStateResources(siteState *api.SiteState, source string, basePath string, tb *pkgutils.Tarball) {
	// Site
	if siteState.Site != nil && siteState.Site.Name != "" {
		writeObjectToTarball(siteState.Site, path.Join(basePath, source, "Site-"+siteState.Site.Name), tb)
	}

	// Connectors
	for _, connector := range siteState.Connectors {
		writeObjectToTarball(connector, path.Join(basePath, source, "Connector-"+connector.Name), tb)
	}

	// Listeners
	for _, listener := range siteState.Listeners {
		writeObjectToTarball(listener, path.Join(basePath, source, "Listener-"+listener.Name), tb)
	}

	// Links
	for _, link := range siteState.Links {
		writeObjectToTarball(link, path.Join(basePath, source, "Link-"+link.Name), tb)
	}

	// RouterAccesses
	for _, ra := range siteState.RouterAccesses {
		writeObjectToTarball(ra, path.Join(basePath, source, "RouterAccess-"+ra.Name), tb)
	}

	// Certificates
	for _, cert := range siteState.Certificates {
		writeObjectToTarball(cert, path.Join(basePath, source, "Certificate-"+cert.Name), tb)
	}

	// Secrets (tls.key will be stripped by writeObjectToTarball)
	for _, secret := range siteState.Secrets {
		writeObjectToTarball(secret, path.Join(basePath, source, "Secret-"+secret.Name), tb)
	}

	// ConfigMaps
	for _, cm := range siteState.ConfigMaps {
		writeObjectToTarball(cm, path.Join(basePath, source, "Configmap-"+cm.Name), tb)
	}

	// AccessGrants
	for _, grant := range siteState.Grants {
		writeObjectToTarball(grant, path.Join(basePath, source, "AccessGrant-"+grant.Name), tb)
	}

	// AccessTokens (Claims)
	for _, token := range siteState.Claims {
		writeObjectToTarball(token, path.Join(basePath, source, "AccessToken-"+token.Name), tb)
	}

	// SecuredAccesses
	for _, sa := range siteState.SecuredAccesses {
		writeObjectToTarball(sa, path.Join(basePath, source, "SecuredAccess-"+sa.Name), tb)
	}

	// MultiKeyListeners
	for _, mkl := range siteState.MultiKeyListeners {
		writeObjectToTarball(mkl, path.Join(basePath, source, "MultiKeyListener-"+mkl.Name), tb)
	}
}

func (cmd *CmdDebug) collectRawYamlFiles(sourcePath string, tarPath string, tb *pkgutils.Tarball) {
	// Read all files from the directory
	files, err := os.ReadDir(sourcePath)
	if err != nil {
		// Directory might not exist, which is fine
		return
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Only collect YAML files
		fileName := file.Name()
		if !strings.HasSuffix(fileName, ".yaml") && !strings.HasSuffix(fileName, ".yml") {
			continue
		}

		// Read the file content
		filePath := filepath.Join(sourcePath, fileName)
		content, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read raw YAML file %s: %v\n", filePath, err)
			continue
		}

		// Write to tarball
		cliutils.WriteTar(path.Join(tarPath, fileName), content, time.Now(), tb)
	}
}

func (cmd *CmdDebug) collectRouterConfig(tb *pkgutils.Tarball) {
	basePath := "/site-namespace/resources/"
	skrPath := path.Join(api.GetInternalOutputPath(cmd.namespace, api.RouterConfigPath), "skrouterd.json")
	skrouterd, err := os.ReadFile(skrPath)
	if err == nil && skrouterd != nil {
		cliutils.WriteTar(path.Join(basePath, "router-config.json"), skrouterd, time.Now(), tb)
	}
}

func (cmd *CmdDebug) collectCertificates(tb *pkgutils.Tarball) {
	// Only collect public certificates, not private keys (tls.key)
	certFiles := []string{"ca.crt", "tls.crt"}

	// Input certificates
	certPath := api.GetInternalOutputPath(cmd.namespace, api.InputCertificatesPath)
	certDirs, err := os.ReadDir(certPath)
	if err == nil && certDirs != nil {
		for _, certDir := range certDirs {
			for _, certFile := range certFiles {
				fileName := path.Join(certDir.Name(), certFile)
				file, err := os.ReadFile(filepath.Join(certPath, fileName))
				if err == nil {
					cliutils.WriteTar(path.Join("/site-namespace/resources/certs/input", fileName), file, time.Now(), tb)
				}
			}
		}
	}

	// Runtime certificates
	certPath = api.GetInternalOutputPath(cmd.namespace, api.CertificatesPath)
	certDirs, err = os.ReadDir(certPath)
	if err == nil && certDirs != nil {
		for _, certDir := range certDirs {
			for _, certFile := range certFiles {
				fileName := path.Join(certDir.Name(), certFile)
				file, err := os.ReadFile(filepath.Join(certPath, fileName))
				if err == nil {
					cliutils.WriteTar(path.Join("/site-namespace/resources/certs/runtime", fileName), file, time.Now(), tb)
				}
			}
		}
	}
}

func (cmd *CmdDebug) collectContainerInfo(tb *pkgutils.Tarball) {
	cli, err := compat.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return
	}

	// List all containers
	containers, err := cli.ContainerList()
	if err != nil {
		return
	}

	basePath := "/site-namespace/resources/"
	namespacePrefix := cmd.namespace + "-"
	for _, container := range containers {
		// Only collect skupper-related containers from this namespace
		if container.Labels["application"] == "skupper-v2" && strings.HasPrefix(container.Name, namespacePrefix) {
			// Inspect container details
			details, err := cli.ContainerInspect(container.Name)
			if err == nil {
				encodedOutput, _ := cliutils.Encode("yaml", details)
				cliutils.WriteTar(path.Join(basePath, "Container-"+container.Name+".yaml"), []byte(encodedOutput), time.Now(), tb)
				cliutils.WriteTar(path.Join(basePath, "Container-"+container.Name+".yaml.txt"), []byte(encodedOutput), time.Now(), tb)
			}
		}
	}

	// Controller container
	// Note: Controller is user-scoped, not namespace-scoped, so it may contain information from other namespaces.
	currentUser, err := user.Current()
	if err == nil {
		ctlContainerName := currentUser.Username + "-skupper-controller"
		details, err := cli.ContainerInspect(ctlContainerName)
		if err == nil {
			encodedOutput, _ := cliutils.Encode("yaml", details)
			cliutils.WriteTar(path.Join(basePath, "Container-"+ctlContainerName+".yaml"), []byte(encodedOutput), time.Now(), tb)
			cliutils.WriteTar(path.Join(basePath, "Container-"+ctlContainerName+".yaml.txt"), []byte(encodedOutput), time.Now(), tb)
		}
	}
}

func (cmd *CmdDebug) collectContainerLogs(tb *pkgutils.Tarball) {
	cli, err := compat.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return
	}

	// Router container
	rtrContainerName := cmd.namespace + "-skupper-router"
	logs, err := cli.ContainerLogs(rtrContainerName)
	if err == nil {
		cliutils.WriteTar(path.Join("/site-namespace/logs", rtrContainerName+".txt"), []byte(logs), time.Now(), tb)
	}

	// Controller container
	// Note: Controller is user-scoped, so its logs may contain information from other namespaces.
	currentUser, err := user.Current()
	if err == nil {
		ctlContainerName := currentUser.Username + "-skupper-controller"
		logs, err = cli.ContainerLogs(ctlContainerName)
		if err == nil {
			cliutils.WriteTar(path.Join("/site-namespace/logs", ctlContainerName+".txt"), []byte(logs), time.Now(), tb)
		}
	}
}

func (cmd *CmdDebug) collectRouterStatsContainer(tb *pkgutils.Tarball) {
	cli, err := compat.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return
	}

	localRouterAddress, err := runtime.GetLocalRouterAddress(cmd.namespace)
	if err != nil {
		return
	}

	rtrContainerName := cmd.namespace + "-skupper-router"
	flags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	for _, flag := range flags {
		skStatCommand := []string{
			"/bin/skstat", flag,
			"-b", localRouterAddress,
			"--ssl-certificate", "/etc/skupper-router/runtime/certs/skupper-local-client/tls.crt",
			"--ssl-key", "/etc/skupper-router/runtime/certs/skupper-local-client/tls.key",
			"--ssl-trustfile", "/etc/skupper-router/runtime/certs/skupper-local-client/ca.crt",
		}
		out, err := cli.ContainerExec(rtrContainerName, skStatCommand)
		if err == nil {
			cliutils.WriteTar(path.Join("/site-namespace/resources/skstat", rtrContainerName+"-skstat"+flag+".txt"), []byte(out), time.Now(), tb)
		}
	}
}

func (cmd *CmdDebug) collectSystemdInfo(tb *pkgutils.Tarball) {
	serviceName := "skupper-" + cmd.namespace + ".service"

	// Build systemctl args based on user
	args := []string{}
	if os.Getuid() != 0 {
		args = append(args, "--user")
	}
	args = append(args, "status", serviceName)

	// Get service status
	status, err := cliutils.RunCommand("systemctl", args...)
	if err == nil {
		cliutils.WriteTar(path.Join("/site-namespace/resources", "Systemd-"+serviceName+"-status.txt"), status, time.Now(), tb)
	}

	// Get service file
	scriptsPath := api.GetInternalOutputPath(cmd.namespace, api.ScriptsPath)
	serviceFile, err := os.ReadFile(filepath.Join(scriptsPath, serviceName))
	if err == nil {
		cliutils.WriteTar(path.Join("/site-namespace/resources", "Systemd-"+serviceName+"-file.txt"), serviceFile, time.Now(), tb)
	}
}

func (cmd *CmdDebug) collectSystemdLogs(tb *pkgutils.Tarball) {
	serviceName := "skupper-" + cmd.namespace + ".service"

	args := []string{}
	if os.Getuid() != 0 {
		args = append(args, "--user")
	}
	args = append(args, "-u", serviceName, "--no-pager", "--all")

	logs, err := cliutils.RunCommand("journalctl", args...)
	if err == nil {
		cliutils.WriteTar(path.Join("/site-namespace/logs", "systemd-journal.txt"), logs, time.Now(), tb)
	}
}

func (cmd *CmdDebug) collectRouterStatsSystemd(tb *pkgutils.Tarball) {
	localRouterAddress, err := runtime.GetLocalRouterAddress(cmd.namespace)
	if err != nil {
		return
	}

	certs := runtime.GetRuntimeTlsCert(cmd.namespace, "skupper-local-client")
	flags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	for _, flag := range flags {
		out, err := cliutils.RunCommand("/usr/bin/skstat", flag,
			"-b", localRouterAddress,
			"--ssl-certificate", certs.CertPath,
			"--ssl-key", certs.KeyPath,
			"--ssl-trustfile", certs.CaPath)
		if err == nil {
			cliutils.WriteTar(path.Join("/site-namespace/resources/skstat", "router-skstat"+flag+".txt"), out, time.Now(), tb)
		}
	}
}

func (cmd *CmdDebug) WaitUntil() error { return nil }
