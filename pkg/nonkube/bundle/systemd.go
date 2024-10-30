package bundle

import (
	"bytes"
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
)

func CreateSystemdServices(siteState *api.SiteState) error {
	var err error
	var logger = common.NewLogger()
	serviceTemplates := map[string]string{
		"systemd":   common.SystemdServiceTemplate,
		"container": common.SystemdContainerServiceTemplate,
	}
	scriptsPath := api.GetInternalBundleOutputPath(siteState.Site.Namespace, api.RuntimeScriptsPath)
	for platform, serviceTemplate := range serviceTemplates {
		var buf = new(bytes.Buffer)
		parsedTemplate := template.Must(template.New("service").Parse(serviceTemplate))
		parsedTemplate.Option()
		err = parsedTemplate.Execute(buf, map[string]interface{}{
			"Site":           siteState.Site,
			"SiteId":         "{{.SiteId}}",
			"Namespace":      "{{.Namespace}}",
			"RuntimeDir":     "{{.RuntimeDir}}",
			"SiteScriptPath": "{{.SiteScriptPath}}",
			"SiteConfigPath": "{{.SiteConfigPath}}",
		})
		if err != nil {
			return fmt.Errorf("failed to execute %s service template: %w", platform, err)
		}
		serviceFile := path.Join(scriptsPath, fmt.Sprintf("skupper.service.%s", platform))
		logger.Debug("writing systemd service file", slog.String("path", serviceFile))
		err = os.WriteFile(serviceFile, buf.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("failed to write %s service file: %w", platform, err)
		}
	}
	return nil
}

func CreateStartupScripts(siteState *api.SiteState, platform types.Platform) error {
	// Creating startup scripts first
	startupArgs := common.StartupScriptsArgs{
		Namespace: siteState.GetNamespace(),
		SiteId:    "{{.SiteId}}",
		Platform:  platform,
		Bundle:    true,
	}
	scripts, err := common.GetStartupScripts(startupArgs, api.GetInternalBundleOutputPath)
	if err != nil {
		return fmt.Errorf("error getting startup scripts: %w", err)
	}
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w", err)
	}
	return nil
}
