package fs

import (
	"errors"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"io/fs"
	v1 "k8s.io/api/core/v1"
)

type SecretHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewSecretHandler(namespace string) *SecretHandler {
	return &SecretHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *SecretHandler) Add(resource v1.Secret) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Secrets)
	if err != nil {
		return err
	}

	return nil
}

func (s *SecretHandler) Get(name string, opt GetOptions) (*v1.Secret, error) { return nil, nil }

func (s *SecretHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.Secrets); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

func (s *SecretHandler) List() ([]*v1.Secret, error) { return nil, nil }
