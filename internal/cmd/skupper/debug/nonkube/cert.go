package nonkube

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	certdisplay "github.com/skupperproject/skupper/internal/cmd/skupper/debug/cert"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

type CmdDebugCert struct {
	CobraCmd  *cobra.Command
	Flags     *common.CommandDebugCertFlags
	namespace string
	certName  string
	output    string
}

func NewCmdDebugCert() *CmdDebugCert {
	return &CmdDebugCert{}
}

func (cmd *CmdDebugCert) NewClient(cobraCommand *cobra.Command, args []string) {
	if cmd.CobraCmd != nil && cmd.CobraCmd.Flag(common.FlagNameNamespace) != nil {
		cmd.namespace = cmd.CobraCmd.Flag(common.FlagNameNamespace).Value.String()
	}
}

func (cmd *CmdDebugCert) ValidateInput(args []string) error {
	var validationErrors []error
	outputTypeValidator := validator.NewOptionValidator(common.OutputTypes)
	filePathValidator := validator.NewFilePathStringValidator()

	if cmd.Flags != nil && cmd.Flags.File != "" {
		ok, err := filePathValidator.Evaluate(cmd.Flags.File)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("file path is not valid: %s", err))
		}
		if len(args) > 0 {
			validationErrors = append(validationErrors, fmt.Errorf("file flag cannot be used with a certificate name argument"))
		}
	} else {
		if len(args) > 1 {
			validationErrors = append(validationErrors, fmt.Errorf("only one certificate name is allowed"))
		}
		if len(args) == 1 {
			cmd.certName = args[0]
		}
	}

	if cmd.Flags != nil && cmd.Flags.Output != "" {
		ok, err := outputTypeValidator.Evaluate(cmd.Flags.Output)
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("output type is not valid: %s", err))
		}
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdDebugCert) InputToOptions() {
	if cmd.Flags != nil {
		cmd.output = cmd.Flags.Output
	}
}

func (cmd *CmdDebugCert) Run() error {
	if cmd.Flags != nil && cmd.Flags.File != "" {
		info, err := certdisplay.ParseCertificateFile(cmd.Flags.File, cmd.Flags.File)
		if err != nil {
			return fmt.Errorf("failed to parse certificate from %s: %w", cmd.Flags.File, err)
		}
		return certdisplay.Display([]certdisplay.Info{*info}, cmd.output, true)
	}

	infos, err := cmd.collectCerts()
	if err != nil {
		return err
	}

	if cmd.certName != "" {
		for _, info := range infos {
			if info.Name == cmd.certName {
				return certdisplay.Display([]certdisplay.Info{info}, cmd.output, true)
			}
		}
		return fmt.Errorf("certificate %s not found", cmd.certName)
	}

	return certdisplay.Display(infos, cmd.output, false)
}

func (cmd *CmdDebugCert) collectCerts() ([]certdisplay.Info, error) {
	var infos []certdisplay.Info
	var err error
	seen := map[string]bool{}

	certPaths := []struct {
		basePath  api.InternalPath
		prefix    string
		isSigning bool
	}{
		{api.CertificatesPath, "", false},
		{api.InputCertificatesPath, "input/", false},
		{api.IssuersPath, "issuers/", true},
		{api.InputIssuersPath, "input/issuers/", true},
	}

	for _, cp := range certPaths {
		infos, err = cmd.collectCertsFromDir(cp.basePath, cp.prefix, cp.isSigning, seen, infos)
		if err != nil {
			return nil, err
		}
	}

	return cmd.collectInputResourceSecrets(seen, infos)
}

func (cmd *CmdDebugCert) collectCertsFromDir(basePath api.InternalPath, prefix string, isSigning bool, seen map[string]bool, infos []certdisplay.Info) ([]certdisplay.Info, error) {
	dir := api.GetInternalOutputPath(cmd.namespace, basePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return infos, nil
		}
		return nil, fmt.Errorf("failed to read certificate directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		displayName := prefix + name
		if seen[displayName] {
			continue
		}
		certFile := filepath.Join(dir, name, "tls.crt")
		data, err := os.ReadFile(certFile)
		if err != nil {
			continue
		}
		info, err := certdisplay.ParseCertificate(displayName, data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate %s: %w", displayName, err)
		}
		signing := isSigning
		info.IsSigningCert = &signing
		seen[displayName] = true
		infos = append(infos, *info)
	}
	return infos, nil
}

func (cmd *CmdDebugCert) collectInputResourceSecrets(seen map[string]bool, infos []certdisplay.Info) ([]certdisplay.Info, error) {
	dir := api.GetInternalOutputPath(cmd.namespace, api.InputSiteStatePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return infos, nil
		}
		return nil, fmt.Errorf("failed to read certificate directory %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filename := entry.Name()
		if !strings.HasPrefix(filename, "Secret-") {
			continue
		}
		if !strings.HasSuffix(filename, ".yaml") && !strings.HasSuffix(filename, ".yml") {
			continue
		}
		secret, err := decodeSecretFile(filepath.Join(dir, filename))
		if err != nil {
			return nil, fmt.Errorf("failed to decode secret from %s: %w", filename, err)
		}
		if secret.Data == nil || len(secret.Data["tls.crt"]) == 0 {
			continue
		}
		displayName := "input/secret/" + secret.Name
		if seen[displayName] {
			continue
		}
		info, err := certdisplay.ParseCertificate(displayName, secret.Data["tls.crt"])
		if err != nil {
			return nil, fmt.Errorf("failed to parse certificate %s: %w", displayName, err)
		}
		seen[displayName] = true
		infos = append(infos, *info)
	}
	return infos, nil
}

func decodeSecretFile(path string) (*corev1.Secret, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	yamlDecoder := yamlutil.NewYAMLOrJSONDecoder(bufio.NewReader(file), 1024)
	var rawObj runtime.RawExtension
	if err := yamlDecoder.Decode(&rawObj); err != nil {
		return nil, err
	}
	obj, gvk, err := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme).Decode(rawObj.Raw, nil, nil)
	if err != nil {
		return nil, err
	}
	if gvk.Kind != "Secret" {
		return nil, fmt.Errorf("expected Secret, got %s", gvk.Kind)
	}
	var secret corev1.Secret
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(runtime.Unstructured).UnstructuredContent(), &secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (cmd *CmdDebugCert) WaitUntil() error { return nil }
