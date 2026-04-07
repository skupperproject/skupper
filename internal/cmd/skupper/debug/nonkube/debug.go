package nonkube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
)

type CmdDebug struct {
	CobraCmd            *cobra.Command
	Flags               *common.CommandDebugFlags
	namespace           string
	fileName            string
	platform            string
	siteHandler         *fs.SiteHandler
	connectorHandler    *fs.ConnectorHandler
	listenerHandler     *fs.ListenerHandler
	linkHandler         *fs.LinkHandler
	routerAccessHandler *fs.RouterAccessHandler
	certificateHandler  *fs.CertificateHandler
	secretHandler       *fs.SecretHandler
	configMapHandler    *fs.ConfigMapHandler
}

func NewCmdDebug() *CmdDebug {

	skupperCmd := CmdDebug{}

	return &skupperCmd
}

func (cmd *CmdDebug) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String() != "" {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}

	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
	cmd.linkHandler = fs.NewLinkHandler(cmd.namespace)
	cmd.routerAccessHandler = fs.NewRouterAccessHandler(cmd.namespace)
	cmd.certificateHandler = fs.NewCertificateHandler(cmd.namespace)
	cmd.secretHandler = fs.NewSecretHandler(cmd.namespace)
	cmd.configMapHandler = fs.NewConfigMapHandler(cmd.namespace)
}

func (cmd *CmdDebug) ValidateInput(args []string) error {
	var validationErrors []error
	fileStringValidator := validator.NewFilePathStringValidator()

	// Validate dump file name
	if len(args) < 1 {
		cmd.fileName = "skupper-dump"
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("filename must not be empty"))
	} else {
		ok, err := fileStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("filename is not valid: %s", err))
		} else {
			cmd.fileName = args[0]
		}
	}

	// Validate that a site exists in the namespace
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	sites, err := cmd.siteHandler.List(opts)
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
	manifest, err := cliutils.RunCommand("skupper", "manifest", "-o", "yaml")
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
}

func (cmd *CmdDebug) collectSiteResources(tb *pkgutils.Tarball) {
	path := "/site-namespace/resources/"

	// Collect both runtime and input resources
	optsRuntime := fs.GetOptions{RuntimeFirst: true, LogWarning: false}
	optsInput := fs.GetOptions{RuntimeFirst: false, LogWarning: false}

	// Sites - collect both runtime and input
	sites, err := cmd.siteHandler.List(optsRuntime)
	if err == nil && sites != nil {
		for _, site := range sites {
			cliutils.WriteObject(site, path+"runtime/Site-"+site.Name, tb)
		}
	}
	sites, err = cmd.siteHandler.List(optsInput)
	if err == nil && sites != nil {
		for _, site := range sites {
			cliutils.WriteObject(site, path+"input/Site-"+site.Name, tb)
		}
	}

	// Connectors
	connectors, err := cmd.connectorHandler.List()
	if err == nil && connectors != nil {
		for _, connector := range connectors {
			cliutils.WriteObject(connector, path+"Connector-"+connector.Name, tb)
		}
	}

	// Listeners
	listeners, err := cmd.listenerHandler.List()
	if err == nil && listeners != nil {
		for _, listener := range listeners {
			cliutils.WriteObject(listener, path+"Listener-"+listener.Name, tb)
		}
	}

	// Links - collect both runtime and input
	links, err := cmd.linkHandler.List(optsRuntime)
	if err == nil && links != nil {
		for _, link := range links {
			cliutils.WriteObject(link, path+"runtime/Link-"+link.Name, tb)
		}
	}
	links, err = cmd.linkHandler.List(optsInput)
	if err == nil && links != nil {
		for _, link := range links {
			cliutils.WriteObject(link, path+"input/Link-"+link.Name, tb)
		}
	}

	// Certificates
	certificates, err := cmd.certificateHandler.List()
	if err == nil && certificates != nil {
		for _, cert := range certificates {
			cliutils.WriteObject(cert, path+"Certificate-"+cert.Name, tb)
		}
	}

	// Secrets
	secrets, err := cmd.secretHandler.List()
	if err == nil && secrets != nil {
		for _, secret := range secrets {
			cliutils.WriteObject(secret, path+"Secret-"+secret.Name, tb)
		}
	}

	// ConfigMaps
	configMaps, err := cmd.configMapHandler.List()
	if err == nil && configMaps != nil {
		for _, cm := range configMaps {
			cliutils.WriteObject(cm, path+"Configmap-"+cm.Name, tb)
		}
	}

	// Note: RouterAccessHandler doesn't have a List method
	// Individual router accesses can be collected through other means if needed
}

func (cmd *CmdDebug) collectRouterConfig(tb *pkgutils.Tarball) {
	path := "/site-namespace/resources/"
	skrPath := api.GetInternalOutputPath(cmd.namespace, api.RouterConfigPath+"/skrouterd.json")
	skrouterd, err := os.ReadFile(skrPath)
	if err == nil && skrouterd != nil {
		cliutils.WriteTar(path+"router-config.json", skrouterd, time.Now(), tb)
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
				fileName := certDir.Name() + "/" + certFile
				file, err := os.ReadFile(filepath.Join(certPath, fileName))
				if err == nil {
					cliutils.WriteTar("/site-namespace/resources/certs/input/"+fileName, file, time.Now(), tb)
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
				fileName := certDir.Name() + "/" + certFile
				file, err := os.ReadFile(filepath.Join(certPath, fileName))
				if err == nil {
					cliutils.WriteTar("/site-namespace/resources/certs/runtime/"+fileName, file, time.Now(), tb)
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

	path := "/site-namespace/resources/"
	for _, container := range containers {
		// Only collect skupper-related containers
		if container.Labels["application"] == "skupper-v2" {
			// Inspect container details
			details, err := cli.ContainerInspect(container.Name)
			if err == nil {
				encodedOutput, _ := cliutils.Encode("yaml", details)
				cliutils.WriteTar(path+"Container-"+container.Name+".yaml", []byte(encodedOutput), time.Now(), tb)
				cliutils.WriteTar(path+"Container-"+container.Name+".yaml.txt", []byte(encodedOutput), time.Now(), tb)
			}
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
		cliutils.WriteTar("/site-namespace/logs/"+rtrContainerName+".txt", []byte(logs), time.Now(), tb)
	}

	// Controller container
	ctlContainerName := "system-controller"
	logs, err = cli.ContainerLogs(ctlContainerName)
	if err == nil {
		cliutils.WriteTar("/site-namespace/logs/"+ctlContainerName+".txt", []byte(logs), time.Now(), tb)
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
			cliutils.WriteTar("/site-namespace/resources/skstat/"+rtrContainerName+"-skstat"+flag+".txt", []byte(out), time.Now(), tb)
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
		cliutils.WriteTar("/site-namespace/resources/Systemd-"+serviceName+"-status.txt", status, time.Now(), tb)
	}

	// Get service file
	scriptsPath := api.GetInternalOutputPath(cmd.namespace, api.ScriptsPath)
	serviceFile, err := os.ReadFile(filepath.Join(scriptsPath, serviceName))
	if err == nil {
		cliutils.WriteTar("/site-namespace/resources/Systemd-"+serviceName+"-file.txt", serviceFile, time.Now(), tb)
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
		cliutils.WriteTar("/site-namespace/logs/systemd-journal.txt", logs, time.Now(), tb)
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
			cliutils.WriteTar("/site-namespace/resources/skstat/router-skstat"+flag+".txt", out, time.Now(), tb)
		}
	}
}

func (cmd *CmdDebug) WaitUntil() error { return nil }
