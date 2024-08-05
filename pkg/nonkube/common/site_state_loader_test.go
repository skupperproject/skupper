package common

import (
	"os"
	"path"
	"testing"

	"github.com/skupperproject/skupper/pkg/nonkube/apis"
	"gotest.tools/assert"
)

func TestFileSystemSiteStateLoder(t *testing.T) {
	outputPath, err := os.MkdirTemp("", "sitestate-loader-*")
	assert.Assert(t, err)
	defer func() {
		err = os.RemoveAll(outputPath)
		assert.Assert(t, err)
	}()
	ss := fakeSiteState()
	ss.CreateBridgeCertificates()
	ss.CreateLinkAccessesCertificates()
	assert.Assert(t, apis.MarshalSiteState(*ss, outputPath))

	fsStateLoader := &FileSystemSiteStateLoader{
		Path: path.Join(outputPath),
	}
	loadedSiteState, err := fsStateLoader.Load()
	assert.Assert(t, err)
	assert.Assert(t, loadedSiteState != nil)
}
