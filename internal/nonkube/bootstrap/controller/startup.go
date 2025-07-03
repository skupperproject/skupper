package controller

import (
	"bytes"
	_ "embed"
	"fmt"
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/nonkube/common"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"log/slog"
	"os"
	"path"
	"text/template"
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
	Name            string
	SkupperPlatform string
	ContainerEngine string
	path            string
}

type StartupScriptsArgs struct {
	Name     string
	Platform types.Platform
	Bundle   bool
}

func GetStartupScripts(args StartupScriptsArgs, controllerPath string) (StartupScript, error) {
	scripts := &startupScripts{
		StartScript:     StartScriptContainerTemplate,
		StopScript:      StopScriptContainerTemplate,
		Name:            args.Name,
		SkupperPlatform: "podman",
	}

	if !args.Platform.IsContainerEngine() && !args.Bundle {
		return nil, fmt.Errorf("startup scripts can only be used with podman or docker platforms")
	}
	scripts.SkupperPlatform = string(args.Platform)
	scripts.ContainerEngine = scripts.SkupperPlatform
	if args.Bundle {
		scripts.ContainerEngine = "{{.ContainerEngine}}"
	}

	scriptsPath := path.Join(controllerPath, string(api.ScriptsPath))

	scripts.path = scriptsPath
	return scripts, nil
}

func (s *startupScripts) Create() error {

	var startBuf bytes.Buffer
	var stopBuf bytes.Buffer
	logger := common.NewLogger()
	logger.Debug("creating startup scripts")

	if err := os.MkdirAll(s.path, 0755); err != nil {
		return fmt.Errorf("error creating skupper system controller directory %q: %v", s.path, err)
	}

	startTemplate := template.Must(template.New("start").Parse(s.StartScript))
	if err := startTemplate.Execute(&startBuf, s); err != nil {
		return fmt.Errorf("failed to create start script: %w", err)
	}
	startFileName := path.Join(s.path, s.GetStartFileName())
	logger.Debug("writing start script", slog.String("path", startFileName))
	err := os.WriteFile(startFileName, startBuf.Bytes(), 0755)
	if err != nil {
		return err
	}
	stopTemplate := template.Must(template.New("stop").Parse(s.StopScript))
	if err := stopTemplate.Execute(&stopBuf, s); err != nil {
		return fmt.Errorf("failed to create stop script: %w", err)
	}
	stopFileName := path.Join(s.path, s.GetStopFileName())
	logger.Debug("writing stop script", slog.String("path", stopFileName))
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
