package fs

import "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"

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

func (s *SiteHandler) Add(resource v1alpha1.Site) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, "sites")
	if err != nil {
		return err
	}

	return nil
}
func (s *SiteHandler) Update(resource v1alpha1.Site) error { return nil }
func (s *SiteHandler) Get(name string) *v1alpha1.Site      { return nil }
func (s *SiteHandler) Delete(name string) error            { return nil }
