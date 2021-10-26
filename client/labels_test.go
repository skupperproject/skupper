package client

import (
	"regexp"
	"testing"

	"gotest.tools/assert"
)

func TestValidRfc1123Label(t *testing.T) {
	r := regexp.MustCompile(ValidRfc1123Label)
	assert.Assert(t, r.MatchString("a=a"))
	assert.Assert(t, r.MatchString("label1=value1"))
	assert.Assert(t, r.MatchString("label1=Value1"))
	assert.Assert(t, r.MatchString("my.app_label=my.App-Label_01"))
	assert.Assert(t, r.MatchString("my.app_label-01=my.App-Label_01,my.app_label-02=my.App-Label_02,my.app_label-03=My.App-Label_03,my.app_label-04=1.0"))
}

func TestInvalidRfc1123Label(t *testing.T) {
	r := regexp.MustCompile(ValidRfc1123Label)

	assert.Assert(t, !r.MatchString("a"), "no value")
	assert.Assert(t, !r.MatchString("label1-=value1"), "invalid key")
	assert.Assert(t, !r.MatchString("-label1=value1"), "invalid key")
	assert.Assert(t, !r.MatchString("label1=value1-"), "invalid value")
	assert.Assert(t, !r.MatchString("label1=-value1"), "invalid value")
	assert.Assert(t, !r.MatchString("Label1=value1"), "invalid key")
	assert.Assert(t, !r.MatchString("label1=value1,label2"), "invalid second label")
	assert.Assert(t, !r.MatchString("-label1=value1,label2=value2"), "invalid first key")
	assert.Assert(t, !r.MatchString("label1-=value1,label2=value2"), "invalid first key")
	assert.Assert(t, !r.MatchString("label1=-value1,label2=value2"), "invalid first value")
	assert.Assert(t, !r.MatchString("label1=value1-,label2=value2"), "invalid first value")
	assert.Assert(t, !r.MatchString("label1=value1,-label2=value2"), "invalid second key")
	assert.Assert(t, !r.MatchString("label1=value1,label2-=value2"), "invalid second key")
	assert.Assert(t, !r.MatchString("label1=value1,Label2=value2,label3=value3"), "invalid second key")
	assert.Assert(t, !r.MatchString("label1=value1,label2=-value2"), "invalid second value")
	assert.Assert(t, !r.MatchString("label1=value1,label2=value2-"), "invalid second value")
	assert.Assert(t, !r.MatchString("label1=value1,label2=value2-"), "invalid second value")
	assert.Assert(t, !r.MatchString("label1=value1,label2=value2,label3=value3 "), "invalid third value")
	assert.Assert(t, !r.MatchString("my.app_label-01=my.App-Label_01,my.app_label-02=my.App-Label_02,my.app_label-03=My.App-Label_03,my.app_label-04=1.0."), "invalid fourth value")
}
