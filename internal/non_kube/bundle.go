package non_kube

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"text/template"
)

const (
	scriptExit = "\nexit 0\n"
	shellDelim = "\n__TARBALL_CONTENT__\n"
)

var (
	//go:embed self_extract.sh
	selfExtractPart string

	//go:embed install.sh.template
	installScript string
)

type SelfExtractingBundle struct {
	SiteName   string
	OutputPath string
}

func (s *SelfExtractingBundle) InstallFile() string {
	return path.Join(s.OutputPath, fmt.Sprintf("skupper-install-%s.sh", s.SiteName))
}

func (s *SelfExtractingBundle) Generate(siteData []byte) error {
	var data = new(bytes.Buffer)
	var err error

	write := func(buf interface{}) error {
		var size, written int
		switch buf.(type) {
		case string:
			size = len(buf.(string))
			written, err = data.WriteString(buf.(string))
		case []byte:
			size = len(buf.([]byte))
			written, err = data.Write(buf.([]byte))
		}
		if err != nil || written != size {
			return fmt.Errorf("error writing data (size: %d - written: %d): %w", size, written, err)
		}
		return nil
	}

	if err := write(selfExtractPart); err != nil {
		return err
	}
	installScriptTemplate := template.Must(template.New("install").Parse(installScript))
	var parsedInstallScript = new(bytes.Buffer)
	err = installScriptTemplate.Execute(parsedInstallScript, map[string]interface{}{
		"SiteName": s.SiteName,
	})
	if err != nil {
		return err
	}
	if err := write(parsedInstallScript.String()); err != nil {
		return err
	}
	if err := write(scriptExit); err != nil {
		return err
	}
	if err := write(shellDelim); err != nil {
		return err
	}
	if err := write(siteData); err != nil {
		return err
	}

	err = os.WriteFile(s.InstallFile(), data.Bytes(), 0755)
	return err
}
