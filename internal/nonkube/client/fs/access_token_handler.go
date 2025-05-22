package fs

import (
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type AccessTokenHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewAccessTokenHandler(namespace string) *AccessTokenHandler {
	return &AccessTokenHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (a *AccessTokenHandler) Add(resource v2alpha1.AccessToken) error {

	fileName := resource.Name + ".yaml"
	content, err := a.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = a.WriteFile(a.pathProvider.GetNamespace(), fileName, content, common.AccessTokens)
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

	if err := a.DeleteFile(a.pathProvider.GetNamespace(), fileName, common.AccessTokens); err != nil {
		return err
	}

	return nil
}

func (a *AccessTokenHandler) List() ([]*v2alpha1.AccessToken, error) { return nil, nil }
