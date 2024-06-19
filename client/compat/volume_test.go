package compat

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/container"
	"gotest.tools/assert"
)

func TestVolume(t *testing.T) {
	var err error
	ctx, cancel := context.WithCancel(context.Background())
	cli, wg := NewClientOrSkip(t, "", ctx)
	defer wg.Wait()
	defer cancel()

	name := RandomName("skupper-volume-test")
	labels := map[string]string{
		"label1": "value1",
		"label2": "value2",
	}

	// Creating volume
	t.Run("volume-create", func(t *testing.T) {
		_, err = cli.VolumeCreate(&container.Volume{
			Name:   name,
			Labels: labels,
		})
		assert.Assert(t, err, "error creating volume")
	})

	// Inspecting volume
	t.Run("volume-inspect", func(t *testing.T) {
		vol, err := cli.VolumeInspect(name)
		assert.Assert(t, err)

		assert.Equal(t, name, vol.Name)
		ValidateMaps(t, labels, vol.Labels)
		assert.Assert(t, vol.Labels["application"] == types.AppName)

		// create file
		fileName := "test.txt"
		fileContent := "test content"
		_, err = vol.CreateFile(fileName, []byte(fileContent), false)
		assert.Assert(t, err, "error creating file test.txt under volume")

		_, err = vol.CreateFile(fileName, []byte(fileContent), false)
		assert.Error(t, err, "file already exists - <nil>")

		_, err = vol.CreateFile(fileName, []byte(fileContent), true)
		assert.Assert(t, err, "error overwriting file test.txt under volume")

		// read file
		content, err := vol.ReadFile(fileName)
		assert.Assert(t, err)
		assert.Assert(t, content == fileContent)

		// list files
		entries, err := vol.ListFiles()
		assert.Assert(t, err)
		assert.Assert(t, 1 == len(entries))
		assert.Assert(t, fileName == entries[0].Name())

		// delete file
		assert.Assert(t, vol.DeleteFile(fileName, false))
	})

	// Listing volume
	t.Run("volume-list", func(t *testing.T) {
		vols, err := cli.VolumeList()
		assert.Assert(t, err)
		found := false
		for _, vol := range vols {
			if vol.Name == name {
				found = true
				break
			}
		}
		assert.Assert(t, found, "volume not listed")
	})

	// Removing volume
	t.Run("volume-remove", func(t *testing.T) {
		assert.Assert(t, cli.VolumeRemove(name))
	})
}
