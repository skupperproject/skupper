package common

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

var (
	//go:embed startsh-container.template
	StartScriptContainerTemplate string

	//go:embed stopsh-container.template
	StopScriptContainerTemplate string
)

type StartupScript interface {
	Create() error
	Remove()
	GetPath() string
	GetStartFileName() string
	GetStopFileName() string
}

type startupScripts struct {
	StartScript     string
	StopScript      string
	Site            *v1alpha1.Site
	SiteId          string
	SkupperPlatform string
	ContainerEngine string
	path            string
}

func GetStartupScripts(site *v1alpha1.Site, siteId string) (StartupScript, error) {
	scripts := &startupScripts{
		StartScript:     StartScriptContainerTemplate,
		StopScript:      StopScriptContainerTemplate,
		Site:            site,
		SiteId:          siteId,
		SkupperPlatform: "podman",
	}

	platform := config.GetPlatform()
	if !platform.IsContainerEngine() && !platform.IsBundle() {
		return nil, fmt.Errorf("startup scripts can only be used with podman or docker platforms")
	}
	scripts.SkupperPlatform = string(platform)
	scripts.ContainerEngine = scripts.SkupperPlatform
	if platform.IsBundle() {
		scripts.ContainerEngine = "{{.ContainerEngine}}"
	}
	scripts.path = api.GetInternalOutputPath(site.Namespace, api.RuntimeScriptsPath)
	return scripts, nil
}

func (s *startupScripts) Create() error {
	var startBuf bytes.Buffer
	var stopBuf bytes.Buffer

	startTemplate := template.Must(template.New("start").Parse(s.StartScript))
	if err := startTemplate.Execute(&startBuf, s); err != nil {
		return fmt.Errorf("failed to create start script: %w", err)
	}
	startFileName := path.Join(s.path, s.GetStartFileName())
	err := os.WriteFile(startFileName, startBuf.Bytes(), 0755)
	if err != nil {
		return err
	}
	stopTemplate := template.Must(template.New("stop").Parse(s.StopScript))
	if err := stopTemplate.Execute(&stopBuf, s); err != nil {
		return fmt.Errorf("failed to create stop script: %w", err)
	}
	stopFileName := path.Join(s.path, s.GetStopFileName())
	err = os.WriteFile(stopFileName, stopBuf.Bytes(), 0755)
	if err != nil {
		return err
	}
	return nil
}

func (s *startupScripts) Remove() {
	startFileName := path.Join(s.path, s.GetStartFileName())
	stopFileName := path.Join(s.path, s.GetStopFileName())
	_ = os.Remove(startFileName)
	_ = os.Remove(stopFileName)
}

func (s *startupScripts) GetPath() string {
	return s.path
}

func (s *startupScripts) GetStartFileName() string {
	return "start.sh"
}

func (s *startupScripts) GetStopFileName() string {
	return "stop.sh"
}
