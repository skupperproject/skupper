package bundle

import (
	"bytes"
	_ "embed"
	"fmt"
	"path"
	"text/template"
	"time"

	"github.com/skupperproject/skupper/internal/utils"
	pkgutils "github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/internal/version"
)

type TarballBundle struct {
	SiteName   string
	OutputPath string
	Namespace  string
}

func (s *TarballBundle) InstallFile() string {
	return path.Join(s.OutputPath, fmt.Sprintf("skupper-install-%s.tar.gz", s.SiteName))
}

func (s *TarballBundle) Generate(tarBall *utils.Tarball, defaultPlatform string) error {
	var err error

	installScriptTemplate := template.Must(template.New("install").Parse(installScript))
	var parsedInstallScript = new(bytes.Buffer)
	err = installScriptTemplate.Execute(parsedInstallScript, map[string]interface{}{
		"SiteName":        s.SiteName,
		"Namespace":       s.Namespace,
		"Platform":        pkgutils.DefaultStr(defaultPlatform, "podman"),
		"Version":         version.Version,
		"SelfExtractPart": "",
	})
	if err != nil {
		return err
	}
	if err = tarBall.AddFileData("install.sh", 0755, time.Now(), parsedInstallScript.Bytes()); err != nil {
		return fmt.Errorf("error writing install.sh: %w", err)
	}
	err = tarBall.Save(s.InstallFile())
	return err
}
