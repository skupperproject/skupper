package base

import (
	"gotest.tools/assert"
	"testing"
)

func TestKubeConfigFiles(t *testing.T) {
	tcs := []struct {
		name    string
		public  int
		private int
	}{
		{name: "multiple-public-multiple-private", public: 3, private: 2},
		{name: "single-public-single-private", public: 1, private: 1},
		{name: "single-public", public: 1, private: 0},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// generates dummy flags
			setUnitTestFlags(tc.public, tc.private)
			// collect returned configs
			publicConfigs := KubeConfigFiles(t, false, true)
			privateConfigs := KubeConfigFiles(t, true, false)
			allConfigs := KubeConfigFiles(t, true, true)
			// validating counts
			assert.Equal(t, tc.public, len(publicConfigs))
			assert.Equal(t, tc.private, len(privateConfigs))
			assert.Equal(t, tc.public+tc.private, len(allConfigs))
		})
	}
}

func TestMultipleClusters(t *testing.T) {
	tcs := []struct {
		name     string
		public   int
		private  int
		expected bool
	}{
		{name: "multiple-public-multiple-private", public: 3, private: 2, expected: true},
		{name: "single-public-single-private", public: 1, private: 1, expected: true},
		{name: "single-public", public: 1, private: 0, expected: false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// generating dummy flags
			setUnitTestFlags(tc.public, tc.private)
			// should match
			assert.Assert(t, MultipleClusters(t) == tc.expected)
		})
	}
}
