package grants

import (
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
)

func Initialise(controller *internalclient.Controller, currentNamespace string, watchNamespace string, config *GrantConfig, generator GrantResponse) func() {
	if !config.Enabled {
		disabled(controller, watchNamespace)
		return nil
	}
	ge := enabled(controller, currentNamespace, watchNamespace, config, generator)
	return ge.Start
}
