package bundle

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"github.com/skupperproject/skupper/pkg/nonkube/common"
)

func CreateSystemdServices(siteState *api.SiteState) error {
	var err error
	serviceTemplates := map[string]string{
		"systemd":   common.SystemdServiceTemplate,
		"container": common.SystemdContainerServiceTemplate,
	}
	scriptsPath := api.GetInternalOutputPath(siteState.Site.Namespace, api.RuntimeScriptsPath)
	for platform, serviceTemplate := range serviceTemplates {
		var buf = new(bytes.Buffer)
		parsedTemplate := template.Must(template.New("service").Parse(serviceTemplate))
		parsedTemplate.Option()
		err = parsedTemplate.Execute(buf, map[string]interface{}{
			"Site":           siteState.Site,
			"Namespace":      "{{.Namespace}}",
			"RuntimeDir":     "{{.RuntimeDir}}",
			"SiteScriptPath": "{{.SiteScriptPath}}",
			"SiteConfigPath": "{{.SiteConfigPath}}",
		})
		if err != nil {
			return fmt.Errorf("failed to execute %s service template: %w", platform, err)
		}
		serviceFile := path.Join(scriptsPath, fmt.Sprintf("skupper.service.%s", platform))
		err = os.WriteFile(serviceFile, buf.Bytes(), 0644)
		if err != nil {
			return fmt.Errorf("failed to write %s service file: %w", platform, err)
		}
	}
	return nil
}

func CreateStartupScripts(siteState *api.SiteState) error {
	// Creating startup scripts first
	scripts, err := common.GetStartupScripts(siteState.Site, "{{.SiteId}}")
	if err != nil {
		return fmt.Errorf("error getting startup scripts: %w", err)
	}
	err = scripts.Create()
	if err != nil {
		return fmt.Errorf("error creating startup scripts: %w", err)
	}
	return nil
}
