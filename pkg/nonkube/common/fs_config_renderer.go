package common

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/qdr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configurationDirectories = []api.InternalPath{
		api.ConfigRouterPath,
		api.CertificatesCaPath,
		api.CertificatesClientPath,
		api.CertificatesServerPath,
		api.CertificatesLinkPath,
		api.LoadedSiteStatePath,
		api.RuntimeSiteStatePath,
		api.RuntimeTokenPath,
		api.RuntimeScriptsPath,
	}
	reloadDirectories = []api.InternalPath{
		api.ConfigRouterPath,
		api.CertificatesClientPath,
		api.CertificatesServerPath,
		api.CertificatesLinkPath,
		api.RuntimeSiteStatePath,
		api.RuntimeTokenPath,
		api.RuntimeScriptsPath,
	}
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
		err = os.WriteFile(path.Join(outputPath, string(api.RuntimeSiteStatePath), "platform.yaml"), []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to write runtime platform: %w", err)
		}
	}
	siteState.Site.SetConfigured(nil)
	endpoints := make([]v1alpha1.Endpoint, 0)
	for raName, ra := range siteState.RouterAccesses {
		for _, role := range ra.Spec.Roles {
			if role.Name != "normal" {
				endpoints = append(endpoints, v1alpha1.Endpoint{
					Name: fmt.Sprintf("%s-%s", raName, role.Name),
					Host: ra.Spec.BindHost,
					Port: strconv.Itoa(role.Port),
				})
			}
		}
	}
	siteState.Site.Status.Endpoints = endpoints
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

func (c *FileSystemConfigurationRenderer) MarshalSiteStates(loadedSiteState, runtimeSiteState *api.SiteState) error {
	if loadedSiteState != nil {
		outputPath := c.GetOutputPath(loadedSiteState)
		sourcesPath := path.Join(outputPath, string(api.LoadedSiteStatePath))
		existingSources, _ := new(utils.DirectoryReader).ReadDir(sourcesPath, nil)
		// when sources are already defined, we back them up
		if len(existingSources) > 0 {
			tb := utils.NewTarball()
			err := tb.AddFiles(sourcesPath)
			if err != nil {
				return fmt.Errorf("unable to backup existing sources: %s", err)
			}
			tbFile := path.Join(outputPath, "sources.backup.tar.gz")
			err = tb.Save(tbFile)
			if err != nil {
				return fmt.Errorf("unable to backup sources.backup.tar.gz: %s", err)
			}
			err = os.RemoveAll(sourcesPath)
			if err != nil {
				return fmt.Errorf("unable to remove old sources: %s", err)
			}
			if err = os.Mkdir(sourcesPath, 0755); err != nil {
				defer func() {
					if err != nil {
						if restoreErr := tb.Extract(tbFile, outputPath); restoreErr != nil {
							fmt.Printf("unable to restore sources.backup.tar.gz: %s", restoreErr)
							return
						}
					}
				}()
				return fmt.Errorf("unable to recreate sources directory %s: %s", sourcesPath, err)
			}
		}
		if err := api.MarshalSiteState(*loadedSiteState, sourcesPath); err != nil {
			return err
		}
	}
	outputPath := c.GetOutputPath(runtimeSiteState)
	runtimeStatePath := path.Join(outputPath, string(api.RuntimeSiteStatePath))
	if err := api.MarshalSiteState(*runtimeSiteState, runtimeStatePath); err != nil {
		return err
	}
	return nil
}

func CleanupNamespaceForReload(namespace string) error {
	// Re-create internal configuration directories
	for _, dir := range reloadDirectories {
		outputPath := api.GetInternalOutputPath(namespace, dir)
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
		secret, err := c.loadClientSecret(siteState, secretName)
		if err != nil {
			return fmt.Errorf("unable to load client secret %s: %v", secretName, err)
		}
		routerTokens := api.CreateTokens(*linkAccess, *secret)
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
			return fmt.Errorf("unable to marshal token: %v", err)
		}
		if err := os.WriteFile(tokenPath, tokenYaml, 0644); err != nil {
			return fmt.Errorf("unable to create token file %s: %v", tokenPath, err)
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
	routerConfigFileName := path.Join(outputPath, string(api.ConfigRouterPath), "skrouterd.json")
	err = os.WriteFile(routerConfigFileName, []byte(routerConfigJson), 0644)
	if err != nil {
		return fmt.Errorf("unable to write router config file: %v", err)
	}
	return nil
}

func (c *FileSystemConfigurationRenderer) createTlsCertificates(siteState *api.SiteState) error {
	var err error
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
					log.Printf("warning: %s will not be overwritten", certFileName)
					continue
				}
			}
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
		secret := certs.GenerateCASecret(name, certificate.Spec.Subject)
		caPath := path.Join(outputPath, string(api.CertificatesCaPath), name)
		err = writeSecretFilesIgnore(caPath, &secret, true)
		if err != nil {
			return err
		}
	}
	// generate all other certificates now
	for name, certificate := range siteState.Certificates {
		var purpose string
		var secret corev1.Secret
		var caSecret *corev1.Secret
		if certificate.Spec.Ca != "" {
			caSecret, err = c.loadCASecret(siteState, certificate.Spec.Ca)
			if err != nil {
				return fmt.Errorf("unable to load CA secret %s: %v", certificate.Spec.Ca, err)
			}
		}
		if certificate.Spec.Client {
			purpose = "client"
			secret = certs.GenerateSecret(name, certificate.Spec.Subject, strings.Join(certificate.Spec.Hosts, ","), caSecret)
			// TODO Not sure if connect.json is needed (probably need to get rid of it)
			if connectJson := c.connectJson(siteState); connectJson != nil {
				secret.Data["connect.json"] = []byte(*connectJson)
			}
		} else if certificate.Spec.Server {
			purpose = "server"
			secret = certs.GenerateSecret(name, certificate.Spec.Subject, strings.Join(certificate.Spec.Hosts, ","), caSecret)
		} else {
			continue
		}
		certPath := path.Join(outputPath, "certificates", purpose, name)
		err = writeSecretFiles(certPath, &secret)
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
		certPath := path.Join(outputPath, "certificates", "link", secretName+"-profile")
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
				host = getOption(la.Spec.Options, la.Spec.BindHost, "127.0.0.1")
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
	outputPath := c.GetOutputPath(siteState)
	certPath := path.Join(outputPath, fmt.Sprintf("certificates/%s", purpose), name)
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
