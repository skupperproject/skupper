package fs

import (
	"errors"
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"io/fs"
)

type AccessTokenHandler struct {
	BaseCustomResourceHandler
	PathProvider PathProvider
}

func NewAccessTokenHandler(namespace string) *AccessTokenHandler {
	return &AccessTokenHandler{
		PathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (a *AccessTokenHandler) Add(resource v2alpha1.AccessToken) error {

	if resource.Name == "" {
		return fmt.Errorf("resource name is required")
	}
	fileName := resource.Name + ".yaml"

	content, err := a.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = a.WriteFile(a.PathProvider.GetNamespace(), fileName, content, common.AccessTokens)
	if err != nil {
		return err
	}

	return nil
}

func (a *AccessTokenHandler) Get(name string, opt GetOptions) (*v2alpha1.AccessToken, error) {
	return nil, nil
}

func (a *AccessTokenHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := a.DeleteFile(a.PathProvider.GetNamespace(), fileName, common.AccessTokens); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

func (a *AccessTokenHandler) List() ([]*v2alpha1.AccessToken, error) { return nil, nil }
