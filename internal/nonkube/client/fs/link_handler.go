package fs

import (
	"io/fs"
	"os"
	"strings"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type LinkHandler struct {
	BaseCustomResourceHandler
	pathProvider PathProvider
}

func NewLinkHandler(namespace string) *LinkHandler {
	return &LinkHandler{
		pathProvider: PathProvider{
			Namespace: namespace,
		},
	}
}

func (s *LinkHandler) Add(resource v2alpha1.Link) error {

	fileName := resource.Name + ".yaml"
	content, err := s.EncodeToYaml(resource)
	if err != nil {
		return err
	}

	err = s.WriteFile(s.pathProvider.GetNamespace(), fileName, content, common.Links)
	if err != nil {
		return err
	}

	return nil
}

func (s *LinkHandler) Get(name string, opts GetOptions) (*v2alpha1.Link, error) {
	var context v2alpha1.Link
	fileName := name
	if !strings.HasSuffix(name, "yaml") {
		fileName = name + ".yaml"
	}

	if opts.RuntimeFirst == true {
		// Read from runtime directory
		err, link := s.ReadFile(s.pathProvider.GetRuntimeNamespace(), fileName, common.Links)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			return nil, err
		}

		// remove the secret and parse link
		lastIndex := strings.LastIndex(string(link), "---")
		index := 0
		if lastIndex != -1 {
			index = lastIndex + 3
		}

		if err = s.DecodeYaml(link[index:], &context); err != nil {
			return nil, err
		}
	} else {
		// read from input directory to get latest config
		err, file := s.ReadFile(s.pathProvider.GetNamespace(), fileName, common.Links)
		if err != nil {
			return nil, err
		}
		if err := s.DecodeYaml(file, &context); err != nil {
			return nil, err
		}
	}

	return &context, nil
}

func (s *LinkHandler) Delete(name string) error {
	fileName := name
	if !strings.HasSuffix(name, "yaml") {
		fileName = name + ".yaml"
	}

	if err := s.DeleteFile(s.pathProvider.GetNamespace(), fileName, common.Links); err != nil {
		return err
	}

	return nil
}

func (s *LinkHandler) List(opts GetOptions) ([]*v2alpha1.Link, error) {
	var links []*v2alpha1.Link
	var files []fs.DirEntry
	var err error
	path := s.pathProvider.GetRuntimeNamespace()
	prefix := common.Links

	// Based on the opts select the proper directory to get links from
	if opts.ResourcesPath != "" {
		prefix = "link"
		path = opts.ResourcesPath
	}

	// Read from runtime directory
	if opts.RuntimeFirst {
		err, files = s.ReadDir(path, prefix)
		if err != nil {
			if opts.LogWarning {
				os.Stderr.WriteString("Site not initialized yet\n")
			}
			return nil, err
		}
	} else {
		// just get configured values
		path = s.pathProvider.GetNamespace()
		err, files = s.ReadDir(path, prefix)
		if err != nil {
			return nil, err
		}
	}

	for _, file := range files {
		err, link := s.ReadFile(path, file.Name(), prefix)
		if err != nil {
			return nil, err
		}
		// remove the secret and parse link
		lastIndex := strings.LastIndex(string(link), "---")
		index := 0
		if lastIndex != -1 {
			index = lastIndex + 3
		}

		var context v2alpha1.Link
		if err = s.DecodeYaml(link[index:], &context); err != nil {
			return nil, err
		}
		links = append(links, &context)
	}
	return links, nil
}
