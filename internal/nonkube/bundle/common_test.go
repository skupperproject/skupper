package bundle

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestIsValidBundle(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		expect   bool
	}{
		{
			name:   "empty-strategy",
			expect: false,
		},
		{
			name:     "invalid-strategy",
			strategy: "podman",
			expect:   false,
		},
		{
			name:     "bundle-strategy",
			strategy: "bundle",
			expect:   true,
		},
		{
			name:     "tarball-strategy",
			strategy: "tarball",
			expect:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expect, IsValidBundle(test.strategy))
		})
	}
}

func TestGetBundleStrategy(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		flag   string
		expect string
	}{
		{
			name:   "empty-strategy",
			expect: "",
		},
		{
			name:   "invalid-strategy-env",
			env:    "podman",
			expect: "",
		},
		{
			name:   "invalid-strategy-flag",
			flag:   "podman",
			expect: "",
		},
		{
			name:   "invalid-strategy",
			env:    "podman",
			flag:   "podman",
			expect: "",
		},
		{
			name:   "bundle-strategy-env",
			env:    "bundle",
			expect: "bundle",
		},
		{
			name:   "bundle-strategy-flag",
			flag:   "bundle",
			expect: "bundle",
		},
		{
			name:   "bundle-strategy-override",
			flag:   "bundle",
			env:    "tarball",
			expect: "bundle",
		},
		{
			name:   "tarball-strategy-env",
			env:    "tarball",
			expect: "tarball",
		},
		{
			name:   "tarball-strategy-flag",
			flag:   "tarball",
			expect: "tarball",
		},
		{
			name:   "tarball-strategy-override",
			flag:   "tarball",
			env:    "bundle",
			expect: "tarball",
		},
		{
			name:   "tarball-strategy-env-invalid-flag",
			flag:   "invalid",
			env:    "tarball",
			expect: "tarball",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("SKUPPER_BUNDLE", test.env)
			assert.Equal(t, test.expect, GetBundleStrategy(test.flag))
		})
	}
}
