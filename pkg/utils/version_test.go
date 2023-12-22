package utils

import (
	"reflect"
	"testing"
)

func TestParseVersion(t *testing.T) {
	var tests = []struct {
		input    string
		expected Version
	}{
		{"1.2.3", Version{1, 2, 3, ""}},
		{"v1.2.3", Version{1, 2, 3, ""}},
		{"v1.2.3-foo", Version{1, 2, 3, "foo"}},
		{"0.22.9993@bar-xyz", Version{0, 22, 9993, "bar-xyz"}},
		{"0e8beee", Version{}},
		{"1231667", Version{}},
		{"3littlepigs", Version{}},
		{"x0.22.9993@bar-xyz", Version{}},
		{"10.22+whatever", Version{10, 22, 0, "whatever"}},
		{"10+whatever", Version{10, 0, 0, "whatever"}},
		{"10+", Version{10, 0, 0, ""}},
		{"10.", Version{10, 0, 0, ""}},
		{"whatever-10-nonsense", Version{}},
	}
	for _, test := range tests {
		if actual := ParseVersion(test.input); !reflect.DeepEqual(actual, test.expected) {
			t.Errorf("Expected %q for %s, got %q", test.expected, test.input, actual)
		}
	}
}

func TestIsUndefined(t *testing.T) {
	var tests = []struct {
		input    string
		expected bool
	}{
		{"1.2.3", false},
		{"v1.2.3", false},
		{"0.22.9993@bar-xyz", false},
		{"x0.22.9993@bar-xyz", true},
		{"10.22+whatever", false},
		{"10+whatever", false},
		{"whatever-10-nonsense", true},
	}
	for _, test := range tests {
		v := ParseVersion(test.input)
		if actual := v.IsUndefined(); actual != test.expected {
			t.Errorf("Expected IsUndefined() for %s to be %v, got %v", test.input, test.expected, actual)
		}
	}
}

func TestEquivalent(t *testing.T) {
	var tests = []struct {
		a        string
		b        string
		expected bool
	}{
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "v1.2.3", true},
		{"v1.2.3", "v1.2.3", true},
		{"v1.2.3", "1.2.3", true},
		{"v1.2.3", "x1.2.3", false},
		{"1.2.3", "0.2.3", false},
		{"1.2.3", "1.2.4", false},
		{"1.2.3", "1.3.2", false},
		{"0.22.9993@bar-xyz", "0.22.9993", true},
		{"0.22.9993@bar-xyz", "0.22.9993+xyz", true},
		{"0.22.9993@bar-xyz", "0.22.999@bar-xyz", false},
		{"10.22+whatever", "10.22", true},
		{"10.22+whatever", "10.22_ignoreme", true},
		{"10.22+whatever", "10.32_ignoreme", false},
		{"10+whatever", "10+", true},
		{"10+whatever", "10-rrr", true},
		{"cat", "dog", true},
	}
	for _, test := range tests {
		if actual := EquivalentVersion(test.a, test.b); actual != test.expected {
			if test.expected {
				t.Errorf("Expected %s to be equivalent to %s", test.a, test.b)
			} else {
				t.Errorf("Expected %s to not be equivalent to %s", test.a, test.b)
			}
		}
	}
}

func TestMoreRecentThan(t *testing.T) {
	var tests = []struct {
		a        string
		b        string
		expected bool
	}{
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.2", true},
		{"1.2.3", "0.2.3", true},
		{"1.2.3", "0.3.4", true},
		{"1.2.3", "v1.2.3", false},
		{"1.2.3", "1.1.3", true},
		{"1.2.3", "1.1.9", true},
		{"v1.2.3", "1.2.3", false},
		{"v1.2.3", "x1.2.3", true},
		{"v1.2.3", "frog", true},
		{"frog", "v1.2.3", false},
		{"1.2.3", "1.2.4", false},
		{"1.2.3", "1.3.3", false},
		{"1.2.3", "2.1.2", false},
	}
	for _, test := range tests {
		if actual := MoreRecentThanVersion(test.a, test.b); actual != test.expected {
			if test.expected {
				t.Errorf("Expected %s to be MoreRecentThan %s", test.a, test.b)
			} else {
				t.Errorf("Expected %s to not be MoreRecentThan %s", test.a, test.b)
			}
		}
	}
}

func TestLessRecentThan(t *testing.T) {
	var tests = []struct {
		a        string
		b        string
		expected bool
	}{
		{"1.2.3", "1.2.3", false},
		{"1.2.3", "1.2.2", false},
		{"1.2.3", "0.2.3", false},
		{"1.2.3", "0.3.4", false},
		{"1.2.3", "v1.2.3", false},
		{"1.2.3", "1.1.3", false},
		{"1.2.3", "1.1.9", false},
		{"v1.2.3", "1.2.3", false},
		{"v1.2.3", "frog", false},
		{"frog", "v1.2.3", true},
		{"1.2.3", "1.2.4", true},
		{"1.2.3", "1.3.3", true},
		{"1.2.3", "2.1.2", true},
	}
	for _, test := range tests {
		if actual := LessRecentThanVersion(test.a, test.b); actual != test.expected {
			if test.expected {
				t.Errorf("Expected %s to be LessRecentThan %s", test.a, test.b)
			} else {
				t.Errorf("Expected %s to not be LessRecentThan %s", test.a, test.b)
			}
		}
	}
}

func TestIsValidFor(t *testing.T) {
	var tests = []struct {
		actual   string
		minimum  string
		expected bool
	}{
		{"", "0.7.0", false},
		{"0.5.3", "0.7.0", false},
		{"34145a5-modified", "0.7.0", true},
		{"0e8beee", "0.7.0", true},
		{"0.7.0", "0.7.0", true},
		{"0.7.1", "0.7.0", true},
		{"0.8.6", "0.7.5", true},
		{"1.0.0", "0.7.0", true},
	}
	for _, test := range tests {
		if actual := IsValidFor(test.actual, test.minimum); actual != test.expected {
			t.Errorf("Expected IsValidFor(%s, %s) to be %v, got %v", test.actual, test.minimum, test.expected, actual)
		}
	}
}

func TestGetVersionFromImageTag(t *testing.T) {
	var tests = []struct {
		imageTag string
		expected string
	}{
		{"quay.io/skupper/config-sync:1.4.4 (sha256:3b7e81fc45bd)", "1.4.4"},
		{"quay.io/skupper/config-sync:1.4.4", "1.4.4"},
		{"quay.io/skupper/config-sync (sha256:3b7e81fc45bd)", ""},
		{"", ""},
		{"quay.io/skupper/config-sync:1.4.4-prerelease", "1.4.4-prerelease"},
		{"1.5.1", ""},
	}
	for _, test := range tests {
		if actual := GetVersionFromImageTag(test.imageTag); actual != test.expected {
			t.Errorf("Expected GetVersionFromImageTag(%s) to be %s, got %s", test.imageTag, test.expected, actual)
		}
	}
}
