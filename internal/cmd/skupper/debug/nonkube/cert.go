package nonkube

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	certdisplay "github.com/skupperproject/skupper/internal/cmd/skupper/debug/cert"
	"github.com/skupperproject/skupper/internal/utils/validator"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/spf13/cobra"
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
	seen := map[string]bool{}

	certPaths := []struct {
		basePath api.InternalPath
		prefix   string
	}{
		{api.CertificatesPath, ""},
		{api.InputCertificatesPath, "input/"},
	}

	for _, cp := range certPaths {
		dir := api.GetInternalOutputPath(cmd.namespace, cp.basePath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			if seen[name] {
				continue
			}
			certFile := filepath.Join(dir, name, "tls.crt")
			data, err := os.ReadFile(certFile)
			if err != nil {
				continue
			}
			info, err := certdisplay.ParseCertificate(name, data)
			if err != nil {
				return nil, fmt.Errorf("failed to parse certificate %s: %w", name, err)
			}
			if cp.prefix != "" {
				info.Name = cp.prefix + name
			}
			seen[name] = true
			infos = append(infos, *info)
		}
	}

	return infos, nil
}

func (cmd *CmdDebugCert) WaitUntil() error { return nil }
