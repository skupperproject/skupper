package common

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"syscall"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configurationDirectories = []api.InternalPath{
		api.RouterConfigPath,
		api.IssuersPath,
		api.CertificatesPath,
		api.InputIssuersPath,
		api.InputCertificatesPath,
		api.InputSiteStatePath,
		api.LoadedSiteStatePath,
		api.RuntimeSiteStatePath,
		api.RuntimeTokenPath,
		api.ScriptsPath,
	}
	reloadDirectories = []api.InternalPath{
		api.RouterConfigPath,
		api.CertificatesPath,
		api.RuntimeSiteStatePath,
		api.RuntimeTokenPath,
		api.ScriptsPath,
		api.LoadedSiteStatePath,
	}
)

type InputPathType string

const (
	InputPathResources InputPathType = "resources"
	InputPathCerts     InputPathType = "certs"
	InputPathIssuers   InputPathType = "issuers"
)

const (
	DefaultSslProfileBasePath = "${SSL_PROFILE_BASE_PATH}"
)

type FileSystemConfigurationRenderer struct {
	// SslProfileBasePath path where configuration will be read from in runtime
	SslProfileBasePath string
	RouterConfig       qdr.RouterConfig
	Platform           string
	Bundle             bool
	customOutputPath   string
}

func NewFileSystemConfigurationRenderer(outputPath string) *FileSystemConfigurationRenderer {
	return &FileSystemConfigurationRenderer{}
}

// Render simply renders the given site state as configuration files.
func (c *FileSystemConfigurationRenderer) Render(siteState *api.SiteState) error {
	var err error
	logger := NewLogger()
	outputPath := c.GetOutputPath(siteState)
	if c.SslProfileBasePath == "" {
		c.SslProfileBasePath = DefaultSslProfileBasePath
	}
	// Proceed only if output path does not exist
	outputDir, err := os.Open(outputPath)
	if err == nil {
		defer outputDir.Close()
		outputDirStat, err := outputDir.Stat()
		if err != nil {
			return fmt.Errorf("failed to check if output directory exists (%s): %w", outputPath, err)
		}
		if !outputDirStat.IsDir() {
			return fmt.Errorf("output path must be a directory (%s)", outputPath)
		}
	} else {
		var pathErr *os.PathError
		if ok := errors.As(err, &pathErr); ok && !errors.Is(pathErr.Err, syscall.ENOENT) {
			return fmt.Errorf("unable to use output path %s: %v", outputPath, err)
		}
	}
	// Creating internal configuration directories
	for _, dir := range configurationDirectories {
		configDir := path.Join(outputPath, string(dir))
		err := os.MkdirAll(configDir, 0755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("unable to create configuration directory %s: %v", configDir, err)
		}
	}
	// Creating the router config
	err = c.createRouterConfig(siteState)
	if err != nil {
		return fmt.Errorf("unable to create router config: %v", err)
	}

	// Creating the certificates
	err = c.createTlsCertificates(siteState)
	if err != nil {
		return fmt.Errorf("unable to create tls certificates: %v", err)
	}

	// Creating the tokens
	err = c.createTokens(siteState)
	if err != nil {
		return fmt.Errorf("unable to create tokens: %v", err)
	}

	// Saving runtime platform
	if !c.Bundle {
		content := fmt.Sprintf("platform: %s\n", c.Platform)
		platformPath := path.Join(outputPath, string(api.InternalBasePath), "platform.yaml")
		logger.Debug("writing platform", slog.String("platform", c.Platform), slog.String("path", platformPath))
		err = os.WriteFile(platformPath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to write runtime platform: %w", err)
		}
	}
	siteState.Site.SetConfigured(nil)
	siteState.Site.SetRunning(v2alpha1.ReadyCondition())
	endpoints := make([]v2alpha1.Endpoint, 0)
	for raName, ra := range siteState.RouterAccesses {
		for _, role := range ra.Spec.Roles {
			if role.Name != "normal" {
				logger.Debug("site endpoint configured:",
					slog.String("host", ra.Spec.BindHost),
					slog.String("role", role.Name),
					slog.Int("port", role.Port),
				)
				endpoints = append(endpoints, v2alpha1.Endpoint{
					Name:  fmt.Sprintf("%s-%s", raName, role.Name),
					Host:  ra.Spec.BindHost,
					Port:  strconv.Itoa(role.Port),
					Group: "skupper-router",
				})
			}
		}
	}
	siteState.Site.Status.Endpoints = endpoints
	if len(endpoints) > 0 {
		for _, ra := range siteState.RouterAccesses {
			if ra.FindRole("inter-router") != nil || ra.FindRole("edge") != nil {
				ra.Resolve(endpoints, "skupper-router")
			}
		}
		siteState.Site.SetEndpoints(endpoints)
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) GetOutputPath(siteState *api.SiteState) string {
	var customSiteHomeProvider = api.GetCustomSiteHome
	var defaultOutputPathProvider = api.GetDefaultOutputPath
	if siteState.IsBundle() {
		customSiteHomeProvider = api.GetCustomBundleHome
		defaultOutputPathProvider = api.GetDefaultBundleOutputPath
	}
	if c.customOutputPath != "" {
		return customSiteHomeProvider(siteState.Site, c.customOutputPath)
	}
	return defaultOutputPathProvider(siteState.Site.Namespace)
}

func (c *FileSystemConfigurationRenderer) GetInputPath(siteState *api.SiteState, dir InputPathType) string {
	var customSiteHomeProvider = api.GetCustomSiteHome
	var defaultOutputPathProvider = api.GetDefaultOutputPath
	if c.customOutputPath != "" {
		return path.Join(customSiteHomeProvider(siteState.Site, c.customOutputPath), "input", string(dir))
	}
	return path.Join(defaultOutputPathProvider(siteState.Site.Namespace), "input", string(dir))
}

func (c *FileSystemConfigurationRenderer) MarshalSiteStates(loadedSiteState, runtimeSiteState *api.SiteState) error {
	logger := NewLogger()
	if loadedSiteState != nil {
		outputPath := c.GetOutputPath(loadedSiteState)
		sourcesPath := path.Join(outputPath, string(api.LoadedSiteStatePath))
		inputSourcesPath := path.Join(outputPath, string(api.InputSiteStatePath))
		existingLoadedSources, _ := new(utils.DirectoryReader).ReadDir(sourcesPath, nil)
		existingInputSources, _ := new(utils.DirectoryReader).ReadDir(inputSourcesPath, nil)
		// when sources are already defined, we back them up
		if len(existingLoadedSources) > 0 {
			tb := utils.NewTarball()
			err := tb.AddFiles(sourcesPath)
			if err != nil {
				return fmt.Errorf("unable to backup existing sources: %s", err)
			}
			tbFile := path.Join(outputPath, "sources.backup.tar.gz")
			logger.Debug("saving loaded state sources", slog.String("path", tbFile))
			err = tb.Save(tbFile)
			if err != nil {
				return fmt.Errorf("unable to backup sources.backup.tar.gz: %s", err)
			}
			logger.Debug("clean up previous loaded state", slog.String("path", sourcesPath))
			err = os.RemoveAll(sourcesPath)
			if err != nil {
				return fmt.Errorf("unable to remove former loaded state: %s", err)
			}
			if err = os.Mkdir(sourcesPath, 0755); err != nil {
				return fmt.Errorf("unable to recreate sources directory %s: %s", sourcesPath, err)
			}
		}
		if len(existingInputSources) == 0 {
			logger.Debug("creating input sources", slog.String("path", inputSourcesPath))
			if err := api.MarshalSiteState(*loadedSiteState, inputSourcesPath); err != nil {
				return err
			}
		}
		logger.Debug("saving loaded state", slog.String("path", sourcesPath))
		if err := api.MarshalSiteState(*loadedSiteState, sourcesPath); err != nil {
			return err
		}
	}
	outputPath := c.GetOutputPath(runtimeSiteState)
	runtimeStatePath := path.Join(outputPath, string(api.RuntimeSiteStatePath))
	logger.Debug("saving runtime state", slog.String("path", runtimeStatePath))
	if err := api.MarshalSiteState(*runtimeSiteState, runtimeStatePath); err != nil {
		return err
	}
	return nil
}

func CleanupNamespaceForReload(namespace string) error {
	logger := NewLogger()
	logger.Debug("cleaning up namespace for reload")
	// Re-create internal configuration directories
	for _, dir := range reloadDirectories {
		outputPath := api.GetInternalOutputPath(namespace, dir)
		logger.Debug("recreating directory:", slog.String("path", outputPath))
		if err := os.RemoveAll(outputPath); err != nil {
			return fmt.Errorf("failed to remove directory %s: %w", outputPath, err)
		}
		err := os.MkdirAll(outputPath, 0755)
		if err != nil && !os.IsExist(err) {
			return fmt.Errorf("unable to create configuration directory %s: %v", outputPath, err)
		}
	}
	return nil
}

func BackupNamespace(namespace string) ([]byte, error) {
	namespacePath := api.GetDefaultOutputNamespacesPath()
	tb := utils.NewTarball()
	err := tb.AddFiles(namespacePath, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to add files to tarball: %w", err)
	}
	return tb.SaveData()
}

func RestoreNamespaceData(data []byte) error {
	tb := utils.NewTarball()
	err := tb.ExtractData(data, api.GetDefaultOutputNamespacesPath())
	if err != nil {
		return fmt.Errorf("error restoring namespace files: %w", err)
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) createTokens(siteState *api.SiteState) error {
	logger := NewLogger()
	tokens := make([]*api.Token, 0)
	for name, linkAccess := range siteState.RouterAccesses {
		noInterRouterRole := linkAccess.FindRole("inter-router") == nil
		noEdgeRole := linkAccess.FindRole("edge") == nil
		if noInterRouterRole && noEdgeRole {
			continue
		}
		certName := name
		if linkAccess.Spec.TlsCredentials != "" {
			certName = linkAccess.Spec.TlsCredentials
		}
		secretName := fmt.Sprintf("client-%s", certName)
		serverSecret, err := c.loadCertAsSecret(siteState, "server", certName)
		if err != nil {
			return fmt.Errorf("unable to load server secret %s: %w", certName, err)
		}
		secret, err := c.loadClientSecret(siteState, secretName)
		if err != nil {
			return fmt.Errorf("unable to load client secret %s: %v", secretName, err)
		}
		routerTokens := api.CreateTokens(*linkAccess, serverSecret, *secret)
		// routerAccess is valid (inter-router and edge endpoints defined)
		if len(routerTokens) > 0 {
			tokens = append(tokens, routerTokens...)
		}
	}
	outputPath := c.GetOutputPath(siteState)
	for _, token := range tokens {
		tokenFileName := fmt.Sprintf("%s-%s.yaml", token.Links[0].Name, token.Links[0].Spec.Endpoints[0].Host)
		tokenPath := path.Join(outputPath, string(api.RuntimeTokenPath), tokenFileName)
		tokenYaml, err := token.Marshal()
		if err != nil {
			return fmt.Errorf("unable to marshal static link: %v", err)
		}
		logger.Debug("writing static link", slog.String("path", tokenPath))
		if err := os.WriteFile(tokenPath, tokenYaml, 0644); err != nil {
			return fmt.Errorf("unable to create static link file %s: %v", tokenPath, err)
		}
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) createRouterConfig(siteState *api.SiteState) error {
	c.RouterConfig = siteState.ToRouterConfig(c.SslProfileBasePath, c.Platform)

	// Saving router config
	routerConfigJson, err := qdr.MarshalRouterConfig(c.RouterConfig)
	if err != nil {
		return fmt.Errorf("unable to marshal router config: %v", err)
	}
	outputPath := c.GetOutputPath(siteState)
	routerConfigFileName := path.Join(outputPath, string(api.RouterConfigPath), "skrouterd.json")
	NewLogger().Debug("Writing router configuration", slog.String("path", routerConfigFileName))
	err = os.WriteFile(routerConfigFileName, []byte(routerConfigJson), 0644)
	if err != nil {
		return fmt.Errorf("unable to write router config file: %v", err)
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) createTlsCertificates(siteState *api.SiteState) error {
	var err error
	var logger = NewLogger()
	writeSecretFilesIgnore := func(basePath string, secret *corev1.Secret, ignoreExisting bool) error {
		baseDir, err := os.Open(basePath)
		if err != nil {
			if os.IsNotExist(err) {
				err = os.MkdirAll(basePath, 0755)
				if err != nil {
					return fmt.Errorf("unable to create directory %s: %v", basePath, err)
				}
			}
		} else {
			defer baseDir.Close()
			baseDirStat, err := baseDir.Stat()
			if err != nil {
				return fmt.Errorf("unable to verify directory %s: %v", basePath, err)
			}
			if !baseDirStat.IsDir() {
				return fmt.Errorf("%s is not a directory", basePath)
			}
		}
		for fileName, data := range secret.Data {
			certFileName := path.Join(basePath, fileName)
			if certFile, err := os.Open(certFileName); err == nil {
				// ignoring existing certificate
				_ = certFile.Close()
				if ignoreExisting {
					logger.Warn("certificate will not be overwritten", slog.String("path", certFileName))
					continue
				}
			}
			logger.Debug("writing certificate", slog.String("path", certFileName))
			err = os.WriteFile(certFileName, data, 0640)
			if err != nil {
				return fmt.Errorf("error writing %s: %v", certFileName, err)
			}
		}
		return nil
	}
	writeSecretFiles := func(basePath string, secret *corev1.Secret) error {
		return writeSecretFilesIgnore(basePath, secret, false)
	}
	// create certificate authorities first
	outputPath := c.GetOutputPath(siteState)
	for name, certificate := range siteState.Certificates {
		if certificate.Spec.Signing == false {
			continue
		}
		secret, err := certs.GenerateSecret(name, certificate.Spec.Subject, nil, 0, nil)
		if err != nil {
			return err
		}

		ignoreExisting := true
		userCaSecret, err := c.loadUserCertAsSecret(siteState, "ca", name)
		if userCaSecret != nil && err == nil {
			// override with user provided CA
			ignoreExisting = false
			secret = userCaSecret
			fmt.Printf("-> User provided CA found: %s\n", name)
		}
		caPath := path.Join(outputPath, string(api.IssuersPath), name)
		err = writeSecretFilesIgnore(caPath, secret, ignoreExisting)
		if err != nil {
			return err
		}
	}
	// generate all other certificates now
	for name, certificate := range siteState.Certificates {
		var purpose string
		var secret *corev1.Secret
		var caSecret *corev1.Secret
		if certificate.Spec.Ca != "" {
			caSecret, err = c.loadCASecret(siteState, certificate.Spec.Ca)
			if err != nil {
				return fmt.Errorf("unable to load CA secret %s: %v", certificate.Spec.Ca, err)
			}
		}
		if certificate.Spec.Client {
			purpose = "client"
			secret, err = certs.GenerateSecret(name, certificate.Spec.Subject, certificate.Spec.Hosts, 0, caSecret)
			if err != nil {
				return err
			}
			// TODO Not sure if connect.json is needed (probably need to get rid of it)
			if connectJson := c.connectJson(siteState); connectJson != nil {
				secret.Data["connect.json"] = []byte(*connectJson)
			}
		} else if certificate.Spec.Server {
			purpose = "server"
			secret, err = certs.GenerateSecret(name, certificate.Spec.Subject, certificate.Spec.Hosts, 0, caSecret)
			if err != nil {
				return err
			}
		} else {
			continue
		}
		userSecret, err := c.loadUserCertAsSecret(siteState, purpose, name)
		if userSecret != nil && err == nil {
			// override with user provided secret
			secret = userSecret
			fmt.Printf("-> User provided %s certificate found: %s\n", purpose, name)
		}
		certPath := path.Join(outputPath, string(api.CertificatesPath), name)
		err = writeSecretFiles(certPath, secret)
		if err != nil {
			return err
		}
	}
	// saving link related certificates
	for _, link := range siteState.Links {
		secretName := link.Spec.TlsCredentials
		secret, ok := siteState.Secrets[secretName]
		if !ok {
			return fmt.Errorf("secret %s not found", secretName)
		}
		certPath := path.Join(outputPath, string(api.CertificatesPath), secretName+"-profile")
		err = writeSecretFiles(certPath, secret)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) connectJson(siteState *api.SiteState) *string {
	var host string
	port := 0
	for _, la := range siteState.RouterAccesses {
		for _, role := range la.Spec.Roles {
			if role.Name == "normal" {
				port = role.Port
				host = getOption(la.Spec.Settings, la.Spec.BindHost, "127.0.0.1")
			}
		}
		if port > 0 {
			break
		}
	}
	if port == 0 {
		return nil
	}
	content := `
{
    "scheme": "amqps",
    "host": "` + host + `",
    "port": "` + strconv.Itoa(port) + `",
    "tls": {
        "ca": "/etc/messaging/ca.crt",
        "cert": "/etc/messaging/tls.crt",
        "key": "/etc/messaging/tls.key",
        "verify": true
    }
}
`
	return &content
}

func (c *FileSystemConfigurationRenderer) loadCASecret(siteState *api.SiteState, name string) (*corev1.Secret, error) {
	return c.loadCertAsSecret(siteState, "ca", name)
}

func (c *FileSystemConfigurationRenderer) loadClientSecret(siteState *api.SiteState, name string) (*corev1.Secret, error) {
	return c.loadCertAsSecret(siteState, "client", name)
}

func (c *FileSystemConfigurationRenderer) loadCertAsSecret(siteState *api.SiteState, purpose, name string) (*corev1.Secret, error) {
	outputPath := path.Join(c.GetOutputPath(siteState), string(api.CertificatesPath))
	if purpose == "ca" {
		outputPath = path.Join(c.GetOutputPath(siteState), string(api.IssuersPath))
	}
	return c.loadCertAsSecretFrom(outputPath, siteState, purpose, name)
}

func (c *FileSystemConfigurationRenderer) loadUserCertAsSecret(siteState *api.SiteState, purpose, name string) (*corev1.Secret, error) {
	userInputPath := path.Join(c.GetInputPath(siteState, InputPathCerts))
	if purpose == "ca" {
		userInputPath = path.Join(c.GetInputPath(siteState, InputPathIssuers))
	}
	secret, err := c.loadCertAsSecretFrom(userInputPath, siteState, purpose, name)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	keys := map[string]bool{
		"ca.crt":  false,
		"tls.key": false,
		"tls.crt": false,
	}
	for key := range secret.Data {
		keys[key] = true
	}
	for _, hasKey := range keys {
		if !hasKey {
			return nil, fmt.Errorf("secret %q does not contain required keys: %v", name, keys)
		}
	}
	return secret, nil
}

func (c *FileSystemConfigurationRenderer) loadCertAsSecretFrom(basePath string, siteState *api.SiteState, purpose, name string) (*corev1.Secret, error) {
	certPath := path.Join(basePath, name)
	var secret *corev1.Secret
	certDir, err := os.Open(certPath)
	if err != nil {
		return nil, err
	}
	defer certDir.Close()
	certDirStat, err := certDir.Stat()
	if err != nil {
		return nil, fmt.Errorf("error checking %s certificate dir stats %s: %v", purpose, certPath, err)
	}
	if !certDirStat.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", certPath)
	}
	files, err := certDir.ReadDir(0)
	if err != nil {
		return nil, fmt.Errorf("error reading files in %s: %v", certPath, err)
	}
	secret = &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: siteState.GetNamespace(),
		},
		Data: map[string][]byte{},
	}
	for _, file := range files {
		fileName := path.Join(certPath, file.Name())
		fileContent, err := os.ReadFile(fileName)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %v", fileName, err)
		}
		secret.Data[file.Name()] = fileContent
	}
	return secret, nil
}

func getOption(m map[string]string, key, defaultValue string) string {
	if m != nil {
		if value, ok := m[key]; ok {
			return value
		}
	}
	return defaultValue
}
