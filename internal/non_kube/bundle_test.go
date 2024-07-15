package non_kube

import (
	"testing"

	"gotest.tools/assert"
)

func TestSelfExtractingBundle_InstallFile(t *testing.T) {
	b := &SelfExtractingBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	assert.Equal(t, b.InstallFile(), "/tmp/skupper-install-my-site.sh")
}

func TestSelfExtractingBundle_Generate(t *testing.T) {
	b := &SelfExtractingBundle{
		SiteName:   "my-site",
		OutputPath: "/tmp",
	}
	tb := NewTarball()
	tb.AddFiles("/home/fgiorget/Documents/InterConnect/research/20240705-v2-site-bundle/fedora-west")
	data, err := tb.SaveData()
	assert.Assert(t, err)

	assert.Assert(t, b.Generate(data))
}
