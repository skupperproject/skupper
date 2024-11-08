package fs

import (
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type PathProvider struct {
	Namespace string
}

func (p *PathProvider) GetNamespace() string {
	return api.GetHostNamespaceHome(p.Namespace) + "/" + string(api.InputSiteStatePath)
}

func (p *PathProvider) GetRuntimeNamespace() string {
	return api.GetHostNamespaceHome(p.Namespace) + "/" + string(api.RuntimeSiteStatePath)
}
