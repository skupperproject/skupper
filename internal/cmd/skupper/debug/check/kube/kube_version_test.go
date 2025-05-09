package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/version"
)

func TestCheckVersion(t *testing.T) {
	tests := []struct {
		major         string
		minor         string
		errorExpected bool
	}{
		{"0", "25", true},
		{"1", "23", true},
		{"1", "24", false},
		{"1", "25", false},
		{"2", "22", false},
	}

	for _, test := range tests {
		err := checkVersion(&version.Info{Major: test.major, Minor: test.minor})
		if (test.errorExpected && err == nil) || (!test.errorExpected && err != nil) {
			t.Fail()
		}
	}
}
