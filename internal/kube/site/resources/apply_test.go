package resources

import (
	"testing"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
)

func TestConfigDigest_SiteMode(t *testing.T) {
	dedicated := skupperv2alpha1.SiteModeDedicated
	transit := skupperv2alpha1.SiteModeTransit
	shared := skupperv2alpha1.SiteModeShared

	defaultDigest := configDigest(&skupperv2alpha1.SiteSpec{})
	dedicatedDigest := configDigest(&skupperv2alpha1.SiteSpec{Mode: &dedicated})
	transitDigest := configDigest(&skupperv2alpha1.SiteSpec{Mode: &transit})
	sharedDigest := configDigest(&skupperv2alpha1.SiteSpec{Mode: &shared})
	edgeDigest := configDigest(&skupperv2alpha1.SiteSpec{Edge: true})
	dedicatedEdgeDigest := configDigest(&skupperv2alpha1.SiteSpec{Edge: true, Mode: &dedicated})

	assert.Equal(t, defaultDigest, dedicatedDigest)
	assert.Equal(t, defaultDigest, transitDigest)
	assert.Equal(t, defaultDigest, sharedDigest)
	assert.Equal(t, edgeDigest, dedicatedEdgeDigest)
	assert.Assert(t, defaultDigest != edgeDigest)
}
