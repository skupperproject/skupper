package nonkube

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"text/template"
)

type SelfExtractingBundle struct {
	SiteName   string
	OutputPath string
}

func (s *SelfExtractingBundle) InstallFile() string {
	return path.Join(s.OutputPath, fmt.Sprintf("skupper-install-%s.sh", s.SiteName))
}

func (s *SelfExtractingBundle) Generate(tarBall *Tarball) error {
	var data = new(bytes.Buffer)
	var err error

	write := func(buf interface{}) error {
		var size, written int
		switch b := buf.(type) {
		case string:
			size = len(b)
			written, err = data.WriteString(b)
		case []byte:
			size = len(b)
			written, err = data.Write(b)
		}
		if err != nil || written != size {
			return fmt.Errorf("error writing data (size: %d - written: %d): %w", size, written, err)
		}
		return nil
	}

	installScriptTemplate := template.Must(template.New("install").Parse(installScript))
	var parsedInstallScript = new(bytes.Buffer)
	err = installScriptTemplate.Execute(parsedInstallScript, map[string]interface{}{
		"SiteName":        s.SiteName,
		"SelfExtractPart": selfExtractPart,
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
	siteData, err := tarBall.SaveData()
	if err != nil {
		return fmt.Errorf("error saving tarball data: %w", err)
	}
	if err := write(siteData); err != nil {
		return err
	}

	err = os.WriteFile(s.InstallFile(), data.Bytes(), 0755)
	return err
}
