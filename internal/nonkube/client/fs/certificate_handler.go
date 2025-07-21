package fs

import (
	"fmt"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
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
		return err
	}

	return nil
}

func (c *CertificateHandler) List() ([]*v2alpha1.Certificate, error) {
	var certificates []*v2alpha1.Certificate
	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime secrets try and display configured secrets
	path := c.pathProvider.GetRuntimeNamespace()
	err, files := c.ReadDir(path, common.Certificates)
	if err != nil {
		fmt.Println("err: reading dir", path)
		return nil, err
	}

	for _, file := range files {
		err, site := c.ReadFile(path, file.Name(), common.Certificates)
		if err != nil {
			fmt.Println("err reading file", file.Name())
			return nil, err
		}
		var context v2alpha1.Certificate
		if err = c.DecodeYaml(site, &context); err != nil {
			return nil, err
		}
		certificates = append(certificates, &context)
	}

	return certificates, nil
}
