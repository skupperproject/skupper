package fs

import (
	"errors"
	"io/fs"
	"os"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

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

func (s *SiteHandler) Add(resource v2alpha1.Site) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Sites)
	if err != nil {
		return err
	}

	return nil
}

func (s *SiteHandler) Get(name string, opts GetOptions) (*v2alpha1.Site, error) {
	var context v2alpha1.Site
	fileName := name + ".yaml"

	if opts.RuntimeFirst == true {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime sites try and display configured sites
		err, file := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.Sites)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			err, file = s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Sites)
			if err != nil {
				return nil, err
			}
		}

		if err = s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Sites)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *SiteHandler) Delete(name string) error {
	if name != "" {
		fileName := name + ".yaml"
		if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.Sites); err != nil {
			return err
		}
	} else {
		// remove directory and its contents
		if err := s.DeleteFile(s.pathProvider.GetNamespace(), "", ""); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}
		}
	}

	return nil
}

func (s *SiteHandler) List(opts GetOptions) ([]*v2alpha1.Site, error) {
	var sites []*v2alpha1.Site
	var path string
	var files []fs.DirEntry
	var err error

	inputPath := s.pathProvider.GetNamespace()
	runtimePath := s.pathProvider.GetRuntimeNamespace()

	if opts.InputOnly {
		path = inputPath
		err, files = s.ReadDir(path, common.Sites)
		if err != nil {
			return nil, err
		}
	} else if opts.RuntimeOnly {
		path = runtimePath
		err, files = s.ReadDir(path, common.Sites)
		if err != nil {
			return nil, err
		}
	} else {
		// First read from runtime directory, where output is found after bootstrap
		// has run.  If no runtime sites try and display configured sites
		path = runtimePath
		err, files = s.ReadDir(path, common.Sites)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			path = inputPath
			err, files = s.ReadDir(path, common.Sites)
			if err != nil {
				return nil, err
			}
		}
	}

	for _, file := range files {
		err, site := s.ReadFile(path, file.Name(), common.Sites)
		if err != nil {
			return nil, err
		}
		var context v2alpha1.Site
		if err = s.DecodeYaml(site, &context); err != nil {
			return nil, err
		}
		sites = append(sites, &context)
	}
	return sites, nil
}
