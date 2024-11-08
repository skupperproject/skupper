package fs

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type SiteHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewSiteHandler(namespace string) *SiteHandler {
	return &SiteHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *SiteHandler) Add(resource v2alpha1.Site) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Sites)
	if err != nil {
		return err
	}

	return nil
}

func (s *SiteHandler) Get(name string, opts GetOptions) (*v2alpha1.Site, error) { return nil, nil }
func (s *SiteHandler) Delete(name string) error                                 { return nil }
func (s *SiteHandler) List() ([]*v2alpha1.Site, error)                          { return nil, nil }
