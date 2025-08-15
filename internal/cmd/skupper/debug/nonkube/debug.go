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
	internalclient "github.com/skupperproject/skupper/internal/nonkube/client/compat"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/internal/nonkube/client/runtime"
	nk_common "github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
)

type CmdDebug struct {
	CobraCmd            *cobra.Command
	Flags               *common.CommandDebugFlags
	namespace           string
	fileName            string
	connectorHandler    *fs.ConnectorHandler
	listenerHandler     *fs.ListenerHandler
	siteHandler         *fs.SiteHandler
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

	cmd.connectorHandler = fs.NewConnectorHandler(cmd.namespace)
	cmd.listenerHandler = fs.NewListenerHandler(cmd.namespace)
	cmd.siteHandler = fs.NewSiteHandler(cmd.namespace)
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
	rpath := "/runtime/"
	inpath := "/input/"
	intpath := "/internal/"
	certFiles := []string{"ca.crt", "tls.crt"}
	flags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	//check if namespace exists
	path := api.GetInternalOutputPath(cmd.namespace, api.InputSiteStatePath)
	if _, err := os.ReadDir(path); err != nil {
		return fmt.Errorf("Namespace %s has not been configured, cannot run debug dump command", cmd.namespace)
	}

	// Add extension if not present
	if filepath.Ext(dumpFile) == "" {
		dumpFile = dumpFile + ".tar.gz"
	}

	tarFile, err := os.Create(dumpFile)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	nsl := &nk_common.NamespacePlatformLoader{}
	platform, err := nsl.Load(cmd.namespace)

	pv, err := utils.RunCommand("podman", "version")
	if err == nil {
		utils.WriteTar("/versions/podman.txt", pv, time.Now(), tw)
	}
	pv, err = utils.RunCommand("docker", "version")
	if err == nil {
		utils.WriteTar("/versions/docker.txt", pv, time.Now(), tw)
	}
	pv, err = utils.RunCommand("skrouterd", "--version")
	if err == nil {
		utils.WriteTar("/versions/skrouterd.txt", pv, time.Now(), tw)
	}

	manifest, err := utils.RunCommand("skupper", "version", "-o", "yaml", "-n", cmd.namespace)
	if err == nil {
		utils.WriteTar("/versions/skupper.yaml", manifest, time.Now(), tw)
		utils.WriteTar("/versions/skupper.yaml.txt", manifest, time.Now(), tw)
	}

	//input/certs
	path = inpath + "certs/"
	certPath := api.GetInternalOutputPath(cmd.namespace, api.InputCertificatesPath)
	certDirs, err := os.ReadDir(certPath)
	if err == nil && certDirs != nil && len(certDirs) != 0 {
		for _, certDir := range certDirs {
			for x := range certFiles {
				fileName := certDir.Name() + "/" + certFiles[x]
				file, err := os.ReadFile(certPath + "/" + fileName)
				if file != nil && err == nil {
					utils.WriteTar(path+fileName, file, time.Now(), tw)
				}
			}
		}
	}

	//input/resources
	opts := fs.GetOptions{RemoveKey: true, LogWarning: false}
	path = inpath + "resources/"
	sites, err := cmd.siteHandler.List(opts)
	if err == nil && sites != nil && len(sites) != 0 {
		for _, site := range sites {
			err := utils.WriteObject(site, path+"Site-"+site.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	routerAccesses, err := cmd.routerAccessHandler.List(opts)
	if err == nil && routerAccesses != nil && len(routerAccesses) != 0 {
		for _, routerAccess := range routerAccesses {
			err = utils.WriteObject(routerAccess, path+"RouterAccess-"+routerAccess.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	listeners, err := cmd.listenerHandler.List(opts)
	if err == nil && listeners != nil && len(listeners) != 0 {
		for _, listener := range listeners {
			err := utils.WriteObject(listener, path+"Listener-"+listener.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	connectors, err := cmd.connectorHandler.List(opts)
	if err == nil && connectors != nil && len(connectors) != 0 {
		for _, connector := range connectors {
			err := utils.WriteObject(connector, path+"Connector-"+connector.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	secrets, err := cmd.secretHandler.List(opts)
	if err == nil && secrets != nil && len(secrets) != 0 {
		for _, secret := range secrets {
			err := utils.WriteObject(secret, path+"Secret-"+secret.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	links, err := cmd.linkHandler.List(opts)
	if err == nil && links != nil && len(links) != 0 {
		for _, link := range links {
			err := utils.WriteObject(link, path+"Link-"+link.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	//internal/scripts
	path = intpath + "scripts/"
	scriptsPath := api.GetInternalOutputPath(cmd.namespace, api.ScriptsPath)
	scripts, err := os.ReadDir(scriptsPath)
	if err == nil && scripts != nil && len(scripts) != 0 {
		for _, script := range scripts {
			fileName := script.Name()
			file, err := os.ReadFile(scriptsPath + "/" + fileName)
			if file != nil && err == nil {
				utils.WriteTar(path+fileName, file, time.Now(), tw)
			}
		}
	}

	// runtime/certs
	path = rpath + "certs/"
	certPath = api.GetInternalOutputPath(cmd.namespace, api.CertificatesPath)
	certDirs, err = os.ReadDir(certPath)
	if err == nil && certDirs != nil && len(certDirs) != 0 {
		for _, certDir := range certDirs {
			for x := range certFiles {
				fileName := certDir.Name() + "/" + certFiles[x]
				file, err := os.ReadFile(certPath + "/" + fileName)
				if file != nil && err == nil {
					utils.WriteTar(path+fileName, file, time.Now(), tw)
				}
			}
		}
	}

	// runtime/issuers
	path = rpath + "issuers/"
	iPath := api.GetInternalOutputPath(cmd.namespace, api.IssuersPath)
	iDirs, err := os.ReadDir(iPath)
	if err == nil && iDirs != nil && len(iDirs) != 0 {
		for _, iDir := range iDirs {
			for x := range certFiles {
				fileName := iDir.Name() + "/" + certFiles[x]
				file, err := os.ReadFile(iPath + "/" + fileName)
				if file != nil && err == nil {
					utils.WriteTar(path+fileName, file, time.Now(), tw)
				}
			}
		}
	}

	// runtime/links
	path = rpath + "links/"
	lpath := api.GetInternalOutputPath(cmd.namespace, api.RuntimeTokenPath)
	lopts := fs.GetOptions{ResourcesPath: lpath, RuntimeFirst: true, LogWarning: false}
	links, err = cmd.linkHandler.List(lopts)
	if err == nil && links != nil && len(links) != 0 {
		for _, link := range links {
			err := utils.WriteObject(link, path+link.Name+"-"+link.Spec.Endpoints[0].Host, tw)
			if err != nil {
				return err
			}
		}
	}

	//runtime/resources
	path = rpath + "resources/"
	opts = fs.GetOptions{RuntimeFirst: true, LogWarning: false, RemoveKey: true}
	sites, err = cmd.siteHandler.List(opts)
	if err == nil && sites != nil && len(sites) != 0 {
		for _, site := range sites {
			err := utils.WriteObject(site, path+"Site-"+site.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	routerAccesses, err = cmd.routerAccessHandler.List(opts)
	if err == nil && routerAccesses != nil && len(routerAccesses) != 0 {
		for _, routerAccess := range routerAccesses {
			err = utils.WriteObject(routerAccess, path+"RouterAccess-"+routerAccess.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	listeners, err = cmd.listenerHandler.List(opts)
	if err == nil && listeners != nil && len(listeners) != 0 {
		for _, listener := range listeners {
			err := utils.WriteObject(listener, path+"Listener-"+listener.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	connectors, err = cmd.connectorHandler.List(opts)
	if err == nil && connectors != nil && len(connectors) != 0 {
		for _, connector := range connectors {
			err := utils.WriteObject(connector, path+"Connector-"+connector.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	certificates, err := cmd.certificateHandler.List()
	if err == nil && certificates != nil && len(certificates) != 0 {
		for _, certificate := range certificates {
			err := utils.WriteObject(certificate, path+"Certificate-"+certificate.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	secrets, err = cmd.secretHandler.List(opts)
	if err == nil && secrets != nil && len(secrets) != 0 {
		for _, secret := range secrets {
			err := utils.WriteObject(secret, path+"Secret-"+secret.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	configMaps, err := cmd.configMapHandler.List()
	if err == nil && configMaps != nil && len(configMaps) != 0 {
		for _, configMap := range configMaps {
			err := utils.WriteObject(configMap, path+"ConfigMap-"+configMap.Name, tw)
			if err != nil {
				return err
			}
		}
	}

	//runtime/router
	path = rpath + "router/"
	skrPath := api.GetInternalOutputPath(cmd.namespace, api.RouterConfigPath+"/skrouterd.json")
	//skrPath := pathProvider.GetRuntimeNamespace()+ api.
	skrouterd, err := os.ReadFile(skrPath)
	if err == nil && skrouterd != nil {
		utils.WriteTar(path+"skrouterd.json", skrouterd, time.Now(), tw)
	}

	//logs and skupper-router statistics
	if platform == "linux" {
		user := "--user"
		if os.Getuid() == 0 {
			user = ""
		}
		rtrName := "skupper-" + cmd.namespace + ".service"
		pv, err = utils.RunCommand("journalctl", user, "-u", rtrName, "--no-pager", "--all")
		if err == nil {
			utils.WriteTar(path+"logs/"+rtrName+".txt", pv, time.Now(), tw)
		}
		localRouterAddress, err := runtime.GetLocalRouterAddress(cmd.namespace)
		if err == nil {
			certs := runtime.GetRuntimeTlsCert(cmd.namespace, "skupper-local-client")
			if err == nil {
				for x := range flags {
					pv, err = utils.RunCommand("/usr/bin/skstat", flags[x],
						"-b", localRouterAddress,
						"--ssl-certificate", certs.CertPath,
						"--ssl-key", certs.KeyPath,
						"--ssl-trustfile", certs.CaPath)
					if err == nil {
						utils.WriteTar(path+"skstat/"+"skrouterd"+"-skstat"+flags[x]+".txt", pv, time.Now(), tw)
					}
				}
			}
		}
	} else {
		if err := os.Setenv("SKUPPER_PLATFORM", platform); err == nil {
			cli, err := internalclient.NewCompatClient(os.Getenv("CONTAINER_ENDPOINT"), "")
			if err == nil {
				rtrContainerName := cmd.namespace + "-skupper-router"
				if container, err := cli.ContainerInspect(rtrContainerName); err == nil {
					encodedOutput, _ := utils.Encode("yaml", container)
					utils.WriteTar(rpath+"Container-"+container.Name+".yaml", []byte(encodedOutput), time.Now(), tw)
				}

				localRouterAddress, err := runtime.GetLocalRouterAddress(cmd.namespace)
				if err == nil {
					for x := range flags {
						skStatCommand := []string{
							"/bin/skstat", flags[x],
							"-b", localRouterAddress,
							"--ssl-certificate", "/etc/skupper-router/runtime/certs/skupper-local-client/tls.crt",
							"--ssl-key", "/etc/skupper-router/runtime/certs/skupper-local-client/tls.key",
							"--ssl-trustfile", "/etc/skupper-router/runtime/certs/skupper-local-client/ca.crt",
						}
						out, err := cli.ContainerExec(rtrContainerName, skStatCommand) //strings.Split(skStatCommand, " "))
						if err == nil {
							utils.WriteTar(path+"skstat/"+rtrContainerName+"-skstat"+flags[x]+".txt", []byte(out), time.Now(), tw)
						}
					}
				}

				logs, err := cli.ContainerLogs(rtrContainerName)
				if err == nil {
					utils.WriteTar(rpath+"logs/"+rtrContainerName+".txt", []byte(logs), time.Now(), tw)
				}

				ctlContainerName := "system-controller"
				if container, err := cli.ContainerInspect(ctlContainerName); err == nil {
					encodedOutput, _ := utils.Encode("yaml", container)
					utils.WriteTar(rpath+"Container-"+container.Name+".yaml", []byte(encodedOutput), time.Now(), tw)
				}

				logs, err = cli.ContainerLogs(ctlContainerName)
				if err == nil {
					utils.WriteTar(rpath+"logs/"+ctlContainerName+".txt", []byte(logs), time.Now(), tw)
				}
			}
		}
	}

	fmt.Println("Skupper dump details written to compressed archive: ", dumpFile)
	return nil
}

func (cmd *CmdDebug) WaitUntil() error { return nil }
