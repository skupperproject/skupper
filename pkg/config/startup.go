package config

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/skupperproject/skupper/api/types"
)

var (
	//go:embed startsh-podman.template
	StartScriptPodmanTemplate string

	//go:embed stopsh-podman.template
	StopScriptPodmanTemplate string
)

type StartupScripts struct {
	StartScript string
	StopScript  string
	Platform    types.Platform
}

func GetStartupScripts(platform types.Platform) *StartupScripts {
	switch platform {
	case types.PlatformPodman:
		return &StartupScripts{
			StartScript: StartScriptPodmanTemplate,
			StopScript:  StopScriptPodmanTemplate,
			Platform:    platform,
		}
	}
	return nil
}

func (s *StartupScripts) Create() error {
	startFileName := path.Join(GetDataHome(), s.GetStartFileName())
	err := ioutil.WriteFile(startFileName, []byte(s.StartScript), 0755)
	if err != nil {
		return err
	}
	stopFileName := path.Join(GetDataHome(), s.GetStopFileName())
	err = ioutil.WriteFile(stopFileName, []byte(s.StopScript), 0755)
	if err != nil {
		return err
	}
	return nil
}

func (s *StartupScripts) Remove() {
	startFileName := path.Join(GetDataHome(), s.GetStartFileName())
	stopFileName := path.Join(GetDataHome(), s.GetStopFileName())
	_ = os.Remove(startFileName)
	_ = os.Remove(stopFileName)
}

func (s *StartupScripts) GetPath() string {
	return GetDataHome()
}

func (s *StartupScripts) GetStartFileName() string {
	return fmt.Sprintf("start-%s.sh", s.Platform)
}

func (s *StartupScripts) GetStopFileName() string {
	return fmt.Sprintf("stop-%s.sh", s.Platform)
}
