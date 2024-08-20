package compat

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/skupperproject/skupper/pkg/container"
	"gotest.tools/assert"
)

func TestImageMock(t *testing.T) {
	imagesPulled := 0
	cli := NewCompatClientMock([]*container.Container{})
	t.Run("image-pull", func(t *testing.T) {
		assert.Assert(t, cli.ImagePull(context.Background(), "quay.io/skupper/skupper-router:main"))
		imagesPulled += 1
	})
	t.Run("image-list", func(t *testing.T) {
		images, err := cli.ImageList()
		assert.Assert(t, err)
		assert.Assert(t, len(images) == imagesPulled)
		for _, image := range images {
			imageJson, _ := json.MarshalIndent(image, "", "  ")
			t.Log(string(imageJson))
		}
	})
}
