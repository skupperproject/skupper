package fs

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type SecuredAccessHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewSecuredAccessHandler(namespace string) *SecuredAccessHandler {
	return &SecuredAccessHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *SecuredAccessHandler) Add(resource v2alpha1.SecuredAccess) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.SecuredAccesses)
	if err != nil {
		return err
	}

	return nil
}

func (s *SecuredAccessHandler) Get(name string, opt GetOptions) (*v2alpha1.SecuredAccess, error) {
	return nil, nil
}

func (s *SecuredAccessHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.SecuredAccesses); err != nil {
		return err
	}

	return nil
}

func (s *SecuredAccessHandler) List() ([]*v2alpha1.SecuredAccess, error) { return nil, nil }
