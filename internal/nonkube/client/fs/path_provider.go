package fs

import (
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type PathProvider struct {
	Namespace string
}

func (p *PathProvider) GetNamespace() string {
	return api.GetDefaultOutputPath(p.Namespace) + "/" + string(api.InputSiteStatePath)
}

func (p *PathProvider) GetRuntimeNamespace() string {
	return api.GetDefaultOutputPath(p.Namespace) + "/" + string(api.RuntimeSiteStatePath)
}
