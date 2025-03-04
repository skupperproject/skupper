package grants

import (
	"github.com/skupperproject/skupper/internal/kube/watchers"
)

func Initialise(events *watchers.EventProcessor, currentNamespace string, watchNamespace string, config *GrantConfig, generator GrantResponse, filter NamespaceFilter) func() {
	if !config.Enabled {
		disabled(events, watchNamespace)
		return nil
	}
	ge := enabled(events, currentNamespace, watchNamespace, config, generator, filter)
	return ge.Start
}
