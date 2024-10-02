package fs

import "fmt"

type PathProvider struct {
	Namespace string
}

func (p *PathProvider) getDefaultNamespace() string {
	return ".local/share/skupper/namespaces/default/input/sources"
}

func (p *PathProvider) GetNamespace() string {

	if p.Namespace == "" {
		return p.getDefaultNamespace()
	}
	return fmt.Sprintf(".local/share/skupper/namespaces/%s/input/sources", p.Namespace)
}
