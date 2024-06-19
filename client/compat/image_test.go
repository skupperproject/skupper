package compat

import (
	"encoding/json"
	"testing"

	"gotest.tools/assert"
)

func TestImageList(t *testing.T) {
	cli, err := NewCompatClient("", "")
	assert.Assert(t, err)
	images, err := cli.ImageList()
	assert.Assert(t, err)
	for _, image := range images {
		imageJson, _ := json.MarshalIndent(image, "", "  ")
		t.Log(string(imageJson))
	}
}
