package grants

import (
	"github.com/skupperproject/skupper/pkg/kube"
)

func Initialise(controller *kube.Controller, currentNamespace string, watchNamespace string, config *GrantConfig, generator GrantResponse) func() {
	if !config.Enabled {
		disabled(controller, watchNamespace)
		return nil
	}
	ge := enabled(controller, currentNamespace, watchNamespace, config, generator)
	return ge.Start
}
