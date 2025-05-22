package fs

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

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

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.RouterAccesses)
	if err != nil {
		return err
	}

	return nil
}

func (s *RouterAccessHandler) Get(name string) (*v2alpha1.RouterAccess, error) {
	var context v2alpha1.RouterAccess
	fileName := name + ".yaml"

	// First read from runtime directory, where output is found after bootstrap
	// has run.  If no runtime sites try and display configured sites
	err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.RouterAccesses)
	if err != nil {
		fmt.Println("Site not initialized yet")
		err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.RouterAccesses)
		if err != nil {
			return nil, err
		}
	}

	if err = s.DecodeYaml(file, &context); err != nil {
		return nil, err
	}

	return &context, nil
}

func (s *RouterAccessHandler) Delete(name string) error {
	fileName := name + ".yaml"

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.RouterAccesses); err != nil {
		fmt.Println(err)
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	return nil
}

func (s *RouterAccessHandler) Update(name string) (*v2alpha1.RouterAccess, error) {
	var context v2alpha1.RouterAccess
	fileName := name + ".yaml"

	// read from input directory to get latest config
	err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.RouterAccesses)
	if err != nil {
		return nil, err
	}

	if err = s.DecodeYaml(file, &context); err != nil {
		return nil, err
	}

	return &context, nil
}
