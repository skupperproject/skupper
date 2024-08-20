package bundle

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/utils"
	"gotest.tools/assert"
)

func TestTarballBundle_InstallFile(t *testing.T) {
	b := &TarballBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	assert.Equal(t, b.InstallFile(), "/tmp/skupper-install-my-site.tar.gz")
}

// TestTarballBundle_Generate unit test the tar ball generation
// only, not producing a valid tarball bundle. But it validates
// if the produced tarball is valid and that it contains an
// installation script.
func TestTarballBundle_Generate(t *testing.T) {
	var cleanupPaths []string
	b := &TarballBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	// cleanup function
	defer func() {
		var errors []error
		appendError := func(e error) {
			if e == nil {
				return
			}
			errors = append(errors, e)
		}
		for _, cleanupPath := range cleanupPaths {
			appendError(os.RemoveAll(cleanupPath))
		}
		assert.Equal(t, len(errors), 0, "No errors expected during cleanup, but found: %v", errors)
	}()
	var sitePath string
	var extractPath string
	var err error
	tb := utils.NewTarball()

	t.Run("generate-fake-crs", func(t *testing.T) {
		sitePath, err = fakeSiteCrs(true)
		assert.Assert(t, err)
		cleanupPaths = append(cleanupPaths, sitePath)
	})

	t.Run("generate-tarball-bundle", func(t *testing.T) {
		assert.Assert(t, tb.AddFiles(sitePath))
		assert.Assert(t, b.Generate(tb))
		cleanupPaths = append(cleanupPaths, b.InstallFile())
	})

	t.Run("validate-tarball-bundle", func(t *testing.T) {
		installFileStat, err := os.Stat(b.InstallFile())
		assert.Assert(t, err)
		assert.Assert(t, installFileStat.Mode().IsRegular())
		assert.Assert(t, installFileStat.Mode().Perm() == os.FileMode(0644))
		extractPath, err = os.MkdirTemp("", "tarballbundle.*")
		assert.Assert(t, err)
		cleanupPaths = append(cleanupPaths, extractPath)
	})

	t.Run("validate-tarball-content", func(t *testing.T) {
		assert.Assert(t, tb.Extract(b.InstallFile(), extractPath))
		installFile := path.Join(extractPath, "install.sh")
		helpCommand := exec.Command(installFile, "-h")
		output, err := helpCommand.CombinedOutput()
		assert.Assert(t, err != nil)
		assert.Assert(t, strings.Contains(string(output), fmt.Sprintf("Usage: %s", installFile)))
		mySiteDir, err := os.Stat(path.Join(extractPath, "my-site"))
		assert.Assert(t, err)
		assert.Assert(t, mySiteDir.Mode().IsDir())
	})
}
