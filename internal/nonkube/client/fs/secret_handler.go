package fs

import (
	"fmt"
	"io/fs"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
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
		return err
	}

	return nil
}

func (c *SecretHandler) List(opts GetOptions) ([]*v1.Secret, error) {
	var secrets []*v1.Secret
	var path string
	var files []fs.DirEntry
	var err error

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime secrets try and display configured secrets
	if opts.RuntimeFirst {
		path = c.pathProvider.GetRuntimeNamespace()
		err, files = c.ReadDir(path, common.Secrets)
		if err != nil {
			return nil, err
		}
	} else {
		// just get configured values
		path = c.pathProvider.GetNamespace()
		err, files = c.ReadDir(path, common.Secrets)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, secret := c.ReadFile(path, file.Name(), common.Secrets)
		if err != nil {
			fmt.Println("err reading file", file.Name())
			return nil, err
		}
		var context v1.Secret
		if err = c.DecodeYaml(secret, &context); err != nil {
			return nil, err
		}
		if opts.RemoveKey && context.Data["tls.key"] != nil {
			context.Data["tls.key"] = []byte("")
		}

		secrets = append(secrets, &context)
	}

	return secrets, nil
}
