package nonkube

import (
	"log"
	"testing"

	"gotest.tools/assert"
)

func TestNewTarball(t *testing.T) {
	tb := NewTarball()
	assert.Assert(t, tb.AddFiles("/home/fgiorget/Documents/InterConnect/research/20240705-v2-site-bundle", "fedora-west"))
	//assert.Assert(t, tb.AddFiles("/home/fgiorget/Documents/InterConnect/research/20240705-v2-site-bundle/fedora-west"))
	assert.Assert(t, tb.Save("/tmp/fedora-west.tar.gz"))
	tb = NewTarball()
	assert.Assert(t, tb.AddFiles("/home/fgiorget/Documents/InterConnect/research/20240705-v2-site-bundle/fedora-west"))
	data, err := tb.SaveData()
	assert.Assert(t, err)
	log.Println(len(data))
}
