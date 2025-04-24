package fs

import (
	"errors"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"io/fs"
)

type CertificateHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewCertificateHandler(namespace string) *CertificateHandler {
	return &CertificateHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (c *CertificateHandler) Add(resource v2alpha1.Certificate) error {

	fileName := resource.Name + ".yaml"
	content, err := c.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = c.WriteFile(c.pathProvider.GetNamespace(), fileName, content, common.Certificates)
	if err != nil {
		return err
	}

	return nil
}

func (c *CertificateHandler) Get(name string, opt GetOptions) (*v2alpha1.Certificate, error) {
	return nil, nil
}

func (c *CertificateHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := c.DeleteFile(c.pathProvider.GetNamespace(), fileName, common.Certificates); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

func (c *CertificateHandler) List() ([]*v2alpha1.Certificate, error) { return nil, nil }
