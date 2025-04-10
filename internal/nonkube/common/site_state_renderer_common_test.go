package common

import (
	"reflect"
	"testing"

	"gotest.tools/v3/assert"
)

func TestCopySiteState(t *testing.T) {
	ss := fakeSiteState()
	newSs := CopySiteState(ss)
	assert.DeepEqual(t, ss.Site, newSs.Site)
	assert.Assert(t, ss.Site != newSs.Site)
	assert.DeepEqual(t, ss.Listeners, newSs.Listeners)
	assert.DeepEqual(t, ss.Connectors, newSs.Connectors)
	assert.DeepEqual(t, ss.RouterAccesses, newSs.RouterAccesses)
	assert.DeepEqual(t, ss.Grants, newSs.Grants)
	assert.DeepEqual(t, ss.Links, newSs.Links)
	assert.DeepEqual(t, ss.Secrets, newSs.Secrets)
	assert.DeepEqual(t, ss.Claims, newSs.Claims)
	assert.DeepEqual(t, ss.Certificates, newSs.Certificates)
	assert.DeepEqual(t, ss.SecuredAccesses, newSs.SecuredAccesses)
	assert.DeepEqual(t, ss.ConfigMaps, newSs.ConfigMaps)
	assert.Assert(t, equalsButNotShallowCopy(ss.Listeners, newSs.Listeners))
	assert.Assert(t, equalsButNotShallowCopy(ss.Connectors, newSs.Connectors))
	assert.Assert(t, equalsButNotShallowCopy(ss.RouterAccesses, newSs.RouterAccesses))
	assert.Assert(t, equalsButNotShallowCopy(ss.Grants, newSs.Grants))
	assert.Assert(t, equalsButNotShallowCopy(ss.Links, newSs.Links))
	assert.Assert(t, equalsButNotShallowCopy(ss.Secrets, newSs.Secrets))
	assert.Assert(t, equalsButNotShallowCopy(ss.Claims, newSs.Claims))
	assert.Assert(t, equalsButNotShallowCopy(ss.Certificates, newSs.Certificates))
	assert.Assert(t, equalsButNotShallowCopy(ss.SecuredAccesses, newSs.SecuredAccesses))
	assert.Assert(t, equalsButNotShallowCopy(ss.ConfigMaps, newSs.ConfigMaps))
}

func equalsButNotShallowCopy[T comparable](oldMap, newMap map[string]T) bool {
	for k, v := range oldMap {
		newV := newMap[k]
		if v == newV {
			return false
		}
		if !reflect.DeepEqual(v, newV) {
			return false
		}
	}
	return true
}
