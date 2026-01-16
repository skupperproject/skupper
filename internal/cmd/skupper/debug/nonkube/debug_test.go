package nonkube

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCmdDebug_ValidateInput(t *testing.T) {
	type test struct {
		name              string
		args              []string
		flags             common.CommandDebugFlags
		cobraGenericFlags map[string]string
		expectedError     string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}

	testTable := []test{
		{
			name:          "too many args",
			flags:         common.CommandDebugFlags{},
			args:          []string{"test", "not-valid"},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "empty name",
			flags:         common.CommandDebugFlags{},
			args:          []string{""},
			expectedError: "filename must not be empty",
		},
		{
			name:          "invalid name",
			flags:         common.CommandDebugFlags{},
			args:          []string{"!Bad"},
			expectedError: "filename is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name:  "ok",
			flags: common.CommandDebugFlags{},
			args:  []string{"test"},
		},
		{
			name:  "ok default name",
			flags: common.CommandDebugFlags{},
			args:  []string{},
		},
	}

	command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
	command.CobraCmd = &cobra.Command{Use: "test"}
	command.namespace = "test"

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command.Flags = &test.flags

			if test.cobraGenericFlags != nil && len(test.cobraGenericFlags) > 0 {
				for name, value := range test.cobraGenericFlags {
					command.CobraCmd.Flags().String(name, value, "")
				}
			}

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)

		})
	}
}

func TestCmdDebug_InputToOptions(t *testing.T) {
	type test struct {
		name      string
		namespace string
		filename  string
		args      []string
		flags     common.CommandDebugFlags
	}

	testTable := []test{
		{
			name:      "default name",
			namespace: "default",
			filename:  "skupper-dump",
			args:      []string{},
			flags:     common.CommandDebugFlags{},
		},
		{
			name:      "name",
			namespace: "test",
			filename:  "dump",
			args:      []string{},
			flags:     common.CommandDebugFlags{},
		},
		{
			name:     "name",
			filename: "skupper-dump",
			args:     []string{},
			flags:    common.CommandDebugFlags{},
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {
			command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
			command.CobraCmd = &cobra.Command{Use: "test"}
			namespace := test.namespace
			if namespace == "" {
				namespace = "default"
			}
			command.namespace = test.namespace
			command.fileName = test.filename
			name := fmt.Sprintf("%s-%s-%s", test.filename, namespace, time.Now().Format("20060102150405"))
			command.InputToOptions()

			assert.Check(t, command.fileName == name)
		})
	}
}

func TestCmdDebug_Run(t *testing.T) {
	type test struct {
		name         string
		namespace    string
		DebugName    string
		errorMessage string
	}

	testTable := []test{
		{
			name:      "run namespace exists",
			namespace: "test2",
		},
		{
			name:         "no namespace",
			namespace:    "default",
			errorMessage: "Namespace default has not been configured, cannot run debug dump command",
		},
	}
	// Add a temp file so listener/connector/site exists for delete tests
	listenerResource := v2alpha1.Listener{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Listener",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-listener",
			Namespace: "test2",
		},
	}
	connectorResource := v2alpha1.Connector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Connector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-connector",
			Namespace: "test2",
		},
	}
	siteResource := v2alpha1.Site{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Site",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-site",
			Namespace: "test2",
		},
	}
	routerAccessResource := v2alpha1.RouterAccess{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "RouterAccess",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-routerAccess",
			Namespace: "test2",
		},
	}
	certificateResource := v2alpha1.Certificate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Certificate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-certificate",
			Namespace: "test2",
		},
	}
	secretResource := v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "test2",
		},
	}
	linkResource := v2alpha1.Link{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "Link",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-link",
			Namespace: "test2",
		},
		Spec: v2alpha1.LinkSpec{
			Endpoints: []v2alpha1.Endpoint{
				{
					Name: "inter-router",
					Host: "127.0.0.1",
					Port: "55671",
				},
			},
		},
	}
	configMapResource := v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "skupper.io/v2alpha1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-map",
			Namespace: "test2",
		},
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}
	tmpDir := api.GetDataHome()
	ipath := filepath.Join(tmpDir, "/namespaces/test2/", string(api.InputSiteStatePath))
	rpath := filepath.Join(tmpDir, "/namespaces/test2/", string(api.RuntimeSiteStatePath))
	lpath := filepath.Join(tmpDir, "/namespaces/test2/", string(api.RuntimeTokenPath))

	for _, test := range testTable {
		command := &CmdDebug{Flags: &common.CommandDebugFlags{}}
		command.CobraCmd = &cobra.Command{Use: "test"}
		command.namespace = test.namespace
		command.fileName = "/tmp/test"
		command.siteHandler = fs.NewSiteHandler(command.namespace)
		command.routerAccessHandler = fs.NewRouterAccessHandler(command.namespace)
		command.listenerHandler = fs.NewListenerHandler(command.namespace)
		command.connectorHandler = fs.NewConnectorHandler(command.namespace)
		command.linkHandler = fs.NewLinkHandler(command.namespace)
		command.certificateHandler = fs.NewCertificateHandler(command.namespace)
		command.secretHandler = fs.NewSecretHandler(command.namespace)
		command.configMapHandler = fs.NewConfigMapHandler(command.namespace)

		content, err := command.siteHandler.EncodeToYaml(siteResource)
		assert.Check(t, err == nil)
		err = command.siteHandler.WriteFile(ipath, "my-site.yaml", content, common.Sites)
		assert.Check(t, err == nil)
		err = command.siteHandler.WriteFile(rpath, "my-site.yaml", content, common.Sites)
		assert.Check(t, err == nil)
		//defer command.siteHandler.Delete("my-site")

		content, err = command.routerAccessHandler.EncodeToYaml(routerAccessResource)
		assert.Check(t, err == nil)
		err = command.routerAccessHandler.WriteFile(ipath, "my-routerAccess.yaml", content, common.RouterAccesses)
		assert.Check(t, err == nil)
		err = command.routerAccessHandler.WriteFile(rpath, "my-routerAccess.yaml", content, common.RouterAccesses)
		assert.Check(t, err == nil)
		//defer command.routerAccessHandler.Delete("my-routerAccess")

		content, err = command.listenerHandler.EncodeToYaml(listenerResource)
		assert.Check(t, err == nil)
		err = command.listenerHandler.WriteFile(ipath, "my-listener.yaml", content, common.Listeners)
		assert.Check(t, err == nil)
		err = command.listenerHandler.WriteFile(rpath, "my-listener.yaml", content, common.Listeners)
		assert.Check(t, err == nil)
		//defer command.listenerHandler.Delete("my-listener")

		content, err = command.connectorHandler.EncodeToYaml(connectorResource)
		assert.Check(t, err == nil)
		err = command.connectorHandler.WriteFile(ipath, "my-connector.yaml", content, common.Connectors)
		assert.Check(t, err == nil)
		err = command.connectorHandler.WriteFile(rpath, "my-connector.yaml", content, common.Connectors)
		assert.Check(t, err == nil)
		//defer command.connectorHandler.Delete("my-connector")

		content, err = command.certificateHandler.EncodeToYaml(certificateResource)
		assert.Check(t, err == nil)
		err = command.certificateHandler.WriteFile(ipath, "", content, common.Certificates)
		assert.Check(t, err == nil)
		err = command.certificateHandler.WriteFile(rpath, "my-certificate.yaml", content, common.Certificates)
		assert.Check(t, err == nil)
		//defer command.certificateHandler.Delete("my-certificate")

		content, err = command.secretHandler.EncodeToYaml(secretResource)
		assert.Check(t, err == nil)
		err = command.secretHandler.WriteFile(ipath, "my-secret.yaml", content, common.Secrets)
		assert.Check(t, err == nil)
		err = command.secretHandler.WriteFile(rpath, "my-secret.yaml", content, common.Secrets)
		assert.Check(t, err == nil)
		//defer command.secretHandler.Delete("my-secret")

		content, err = command.linkHandler.EncodeToYaml(linkResource)
		assert.Check(t, err == nil)
		err = command.linkHandler.WriteFile(ipath, "my-link.yaml", content, common.Links)
		assert.Check(t, err == nil)
		err = command.linkHandler.WriteFile(lpath, "my-link.yaml", content, "link")
		assert.Check(t, err == nil)
		//defer command.siteHandler.Delete("my-link")

		content, err = command.configMapHandler.EncodeToYaml(configMapResource)
		assert.Check(t, err == nil)
		err = command.configMapHandler.WriteFile(rpath, "my-configmap.yaml", content, common.ConfigMaps)
		assert.Check(t, err == nil)
		//defer command.secretHandler.Delete("my-secret")

		certPath := api.GetDefaultOutputPath(command.namespace) + "/" + string(api.InputCertificatesPath)
		err = os.MkdirAll(certPath, 0775)
		if err == nil {
			_, _ = os.Create(certPath + "/ca.crt")
		}

		certPath = api.GetDefaultOutputPath(command.namespace) + "/" + string(api.CertificatesPath) + "/link-router-access-test2/"
		err = os.MkdirAll(certPath, 0775)
		if err == nil {
			_, _ = os.Create(certPath + "ca.crt")
		}

		issuerPath := api.GetDefaultOutputPath(command.namespace) + "/" + string(api.IssuersPath) + "/skupper-local-ca/"
		err = os.MkdirAll(issuerPath, 0775)
		if err == nil {
			_, _ = os.Create(issuerPath + "ca.crt")
		}

		scriptPath := api.GetDefaultOutputPath(command.namespace) + "/" + string(api.ScriptsPath)
		err = os.MkdirAll(scriptPath, 0775)
		if err == nil {
			_, _ = os.Create(scriptPath + "/start.sh")
		}

		defer os.Remove("/tmp/test.tar.gz") //clean up

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}
