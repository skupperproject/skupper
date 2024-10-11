package fs

import "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"

type RouterAccessHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewRouterAccessHandler(namespace string) *RouterAccessHandler {
	return &RouterAccessHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *RouterAccessHandler) Add(resource v2alpha1.RouterAccess) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, "routerAccesses")
	if err != nil {
		return err
	}

	return nil
}
func (s *RouterAccessHandler) Update(resource v2alpha1.RouterAccess) error { return nil }
func (s *RouterAccessHandler) Get(name string) *v2alpha1.Site              { return nil }
func (s *RouterAccessHandler) Delete(name string) error                    { return nil }
