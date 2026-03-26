package nonkube

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	nonkubecommon "github.com/skupperproject/skupper/internal/nonkube/common"
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

	tarFile, err := os.Create(dumpFile)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	}
	defer tarFile.Close()

	// Compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	// Collect all diagnostic information
	cmd.collectVersionInfo(tw)
	cmd.collectSiteResources(tw)
	cmd.collectRouterConfig(tw)
	cmd.collectCertificates(tw)

	// Platform-specific collection
	if cmd.platform == "linux" {
		cmd.collectSystemdInfo(tw)
		cmd.collectSystemdLogs(tw)
		cmd.collectRouterStatsSystemd(tw)
	} else {
		cmd.collectContainerInfo(tw)
		cmd.collectContainerLogs(tw)
		cmd.collectRouterStatsContainer(tw)
	}

	fmt.Println("Skupper dump details written to compressed archive:", dumpFile)
	return nil
}

func (cmd *CmdDebug) collectVersionInfo(tw *tar.Writer) {
	// Skupper version
	manifest, err := utils.RunCommand("skupper", "version", "-o", "yaml", "-n", cmd.namespace)
	if err == nil {
		utils.WriteTar("/versions/skupper.yaml", manifest, time.Now(), tw)
		utils.WriteTar("/versions/skupper.yaml.txt", manifest, time.Now(), tw)
	}

	// Platform version
	var platformVersion []byte
	switch cmd.platform {
	case "podman":
		platformVersion, err = utils.RunCommand("podman", "version")
		if err == nil {
			utils.WriteTar("/versions/podman.txt", platformVersion, time.Now(), tw)
		}
	case "docker":
		platformVersion, err = utils.RunCommand("docker", "version")
		if err == nil {
			utils.WriteTar("/versions/docker.txt", platformVersion, time.Now(), tw)
		}
	case "linux":
		// Get systemd version
		platformVersion, err = utils.RunCommand("systemctl", "--version")
		if err == nil {
			utils.WriteTar("/versions/systemd.txt", platformVersion, time.Now(), tw)
		}
	}

	// Router version
	routerVersion, err := utils.RunCommand("skrouterd", "--version")
	if err == nil {
		utils.WriteTar("/versions/skrouterd.txt", routerVersion, time.Now(), tw)
	}
}

func (cmd *CmdDebug) collectSiteResources(tw *tar.Writer) {
	path := "/site-namespace/resources/"
	opts := fs.GetOptions{RuntimeFirst: true, LogWarning: false}

	// Sites
	sites, err := cmd.siteHandler.List(opts)
	if err == nil && sites != nil {
		for _, site := range sites {
			utils.WriteObject(site, path+"Site-"+site.Name, tw)
		}
	}

	// Connectors
	connectors, err := cmd.connectorHandler.List()
	if err == nil && connectors != nil {
		for _, connector := range connectors {
			utils.WriteObject(connector, path+"Connector-"+connector.Name, tw)
		}
	}

	// Listeners
	listeners, err := cmd.listenerHandler.List()
	if err == nil && listeners != nil {
		for _, listener := range listeners {
			utils.WriteObject(listener, path+"Listener-"+listener.Name, tw)
		}
	}

	// Links
	links, err := cmd.linkHandler.List(opts)
	if err == nil && links != nil {
		for _, link := range links {
			utils.WriteObject(link, path+"Link-"+link.Name, tw)
		}
	}

	// Certificates
	certificates, err := cmd.certificateHandler.List()
	if err == nil && certificates != nil {
		for _, cert := range certificates {
			utils.WriteObject(cert, path+"Certificate-"+cert.Name, tw)
		}
	}

	// Secrets
	secrets, err := cmd.secretHandler.List()
	if err == nil && secrets != nil {
		for _, secret := range secrets {
			utils.WriteObject(secret, path+"Secret-"+secret.Name, tw)
		}
	}

	// ConfigMaps
	configMaps, err := cmd.configMapHandler.List()
	if err == nil && configMaps != nil {
		for _, cm := range configMaps {
			utils.WriteObject(cm, path+"Configmap-"+cm.Name, tw)
		}
	}

	// Note: RouterAccessHandler doesn't have a List method
	// Individual router accesses can be collected through other means if needed
}

func (cmd *CmdDebug) collectRouterConfig(tw *tar.Writer) {
	path := "/site-namespace/resources/"
	skrPath := api.GetInternalOutputPath(cmd.namespace, api.RouterConfigPath+"/skrouterd.json")
	skrouterd, err := os.ReadFile(skrPath)
	if err == nil && skrouterd != nil {
		utils.WriteTar(path+"router-config.json", skrouterd, time.Now(), tw)
	}
}

func (cmd *CmdDebug) collectCertificates(tw *tar.Writer) {
	certFiles := []string{"ca.crt", "tls.crt", "tls.key"}

	// Input certificates
	certPath := api.GetInternalOutputPath(cmd.namespace, api.InputCertificatesPath)
	certDirs, err := os.ReadDir(certPath)
	if err == nil && certDirs != nil {
		for _, certDir := range certDirs {
			for _, certFile := range certFiles {
				fileName := certDir.Name() + "/" + certFile
				file, err := os.ReadFile(filepath.Join(certPath, fileName))
				if err == nil {
					utils.WriteTar("/site-namespace/resources/certs/input/"+fileName, file, time.Now(), tw)
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
					utils.WriteTar("/site-namespace/resources/certs/runtime/"+fileName, file, time.Now(), tw)
				}
			}
		}
	}
}

func (cmd *CmdDebug) collectContainerInfo(tw *tar.Writer) {
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
		if container.Labels["application"] == "skupper" {
			// Inspect container details
			details, err := cli.ContainerInspect(container.Name)
			if err == nil {
				encodedOutput, _ := utils.Encode("yaml", details)
				utils.WriteTar(path+"Container-"+container.Name+".yaml", []byte(encodedOutput), time.Now(), tw)
				utils.WriteTar(path+"Container-"+container.Name+".yaml.txt", []byte(encodedOutput), time.Now(), tw)
			}
		}
	}
}

func (cmd *CmdDebug) collectContainerLogs(tw *tar.Writer) {
	cli, err := compat.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
	if err != nil {
		return
	}

	// Router container
	rtrContainerName := cmd.namespace + "-skupper-router"
	logs, err := cli.ContainerLogs(rtrContainerName)
	if err == nil {
		utils.WriteTar("/site-namespace/logs/"+rtrContainerName+".txt", []byte(logs), time.Now(), tw)
	}

	// Controller container
	ctlContainerName := "system-controller"
	logs, err = cli.ContainerLogs(ctlContainerName)
	if err == nil {
		utils.WriteTar("/site-namespace/logs/"+ctlContainerName+".txt", []byte(logs), time.Now(), tw)
	}
}

func (cmd *CmdDebug) collectRouterStatsContainer(tw *tar.Writer) {
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
			utils.WriteTar("/site-namespace/resources/skstat/"+rtrContainerName+"-skstat"+flag+".txt", []byte(out), time.Now(), tw)
		}
	}
}

func (cmd *CmdDebug) collectSystemdInfo(tw *tar.Writer) {
	serviceName := "skupper-" + cmd.namespace + ".service"

	user := ""
	if os.Getuid() != 0 {
		user = "--user"
	}

	// Get service status
	status, err := utils.RunCommand("systemctl", user, "status", serviceName)
	if err == nil {
		utils.WriteTar("/site-namespace/resources/Systemd-"+serviceName+"-status.txt", status, time.Now(), tw)
	}

	// Get service file
	scriptsPath := api.GetInternalOutputPath(cmd.namespace, api.ScriptsPath)
	serviceFile, err := os.ReadFile(filepath.Join(scriptsPath, serviceName))
	if err == nil {
		utils.WriteTar("/site-namespace/resources/Systemd-"+serviceName+"-file.txt", serviceFile, time.Now(), tw)
	}
}

func (cmd *CmdDebug) collectSystemdLogs(tw *tar.Writer) {
	serviceName := "skupper-" + cmd.namespace + ".service"

	user := ""
	if os.Getuid() != 0 {
		user = "--user"
	}

	logs, err := utils.RunCommand("journalctl", user, "-u", serviceName, "--no-pager", "--all")
	if err == nil {
		utils.WriteTar("/site-namespace/logs/systemd-journal.txt", logs, time.Now(), tw)
	}
}

func (cmd *CmdDebug) collectRouterStatsSystemd(tw *tar.Writer) {
	localRouterAddress, err := runtime.GetLocalRouterAddress(cmd.namespace)
	if err != nil {
		return
	}

	certs := runtime.GetRuntimeTlsCert(cmd.namespace, "skupper-local-client")
	flags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	for _, flag := range flags {
		out, err := utils.RunCommand("/usr/bin/skstat", flag,
			"-b", localRouterAddress,
			"--ssl-certificate", certs.CertPath,
			"--ssl-key", certs.KeyPath,
			"--ssl-trustfile", certs.CaPath)
		if err == nil {
			utils.WriteTar("/site-namespace/resources/skstat/router-skstat"+flag+".txt", out, time.Now(), tw)
		}
	}
}

func (cmd *CmdDebug) WaitUntil() error { return nil }
