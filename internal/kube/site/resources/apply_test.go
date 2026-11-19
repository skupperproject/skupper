package resources

import (
	"testing"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
)

func TestConfigDigest_SiteMode(t *testing.T) {
	dedicated := skupperv2alpha1.SiteModeDedicated
	dedicatedEdge := skupperv2alpha1.SiteModeDedicatedEdge

	defaultDigest := configDigest(&skupperv2alpha1.SiteSpec{})
	dedicatedDigest := configDigest(&skupperv2alpha1.SiteSpec{Mode: &dedicated})
	legacyEdgeDigest := configDigest(&skupperv2alpha1.SiteSpec{Edge: true})
	modeEdgeDigest := configDigest(&skupperv2alpha1.SiteSpec{Mode: &dedicatedEdge})

	assert.Equal(t, defaultDigest, dedicatedDigest)
	assert.Equal(t, legacyEdgeDigest, modeEdgeDigest)
	assert.Assert(t, defaultDigest != legacyEdgeDigest)
}
