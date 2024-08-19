package client

import "fmt"

type PathProvider struct {
	Namespace string
}

func (p *PathProvider) GetDefaultNamespace() string {
	return ".local/share/skupper/default"
}

func (p *PathProvider) GetNamespace() string {
	return fmt.Sprintf(".local/share/skupper/%s", p.Namespace)
}
