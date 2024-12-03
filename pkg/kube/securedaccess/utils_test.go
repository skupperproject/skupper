package securedaccess

import (
	"testing"

	"gotest.tools/v3/assert"
)

func Test_possibleKeyPortNamePairs(t *testing.T) {
	testTable := []struct {
		input    string
		expected []pair
	}{
		{
			input: "namespace/abc",
		},
		{
			input:    "namespace/ab-c",
			expected: []pair{{"namespace/ab", "c"}},
		},
		{
			input:    "foo/a-bc",
			expected: []pair{{"foo/a", "bc"}},
		},
		{
			input:    "bar/a-b-c",
			expected: []pair{{"bar/a", "b-c"}, {"bar/a-b", "c"}},
		},
		{
			input: "my-namespace/my-name-my-port",
			expected: []pair{
				{"my-namespace/my", "name-my-port"},
				{"my-namespace/my-name", "my-port"},
				{"my-namespace/my-name-my", "port"},
			},
		},
		{
			input: "a-string-with-lots-of-parts/and-then-another",
			expected: []pair{
				{"a-string-with-lots-of-parts/and", "then-another"},
				{"a-string-with-lots-of-parts/and-then", "another"},
			},
		},
		{
			input: "unqualified-with-port",
			expected: []pair{
				{"unqualified", "with-port"},
				{"unqualified-with", "port"},
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.input, func(t *testing.T) {
			actual := possibleKeyPortNamePairs(tt.input)
			assert.DeepEqual(t, tt.expected, actual)
		})
	}
}

func Test_splits(t *testing.T) {
	testTable := []struct {
		input    string
		expected []pair
	}{
		{
			input:    "abc",
			expected: []pair{{"", "abc"}},
		},
		{
			input:    "ab-c",
			expected: []pair{{"ab", "c"}},
		},
		{
			input:    "a-bc",
			expected: []pair{{"a", "bc"}},
		},
		{
			input:    "a-b-c",
			expected: []pair{{"a", "b-c"}, {"a-b", "c"}},
		},
		{
			input: "my-name-my-port",
			expected: []pair{
				{"my", "name-my-port"},
				{"my-name", "my-port"},
				{"my-name-my", "port"},
			},
		},
		{
			input: "a-string-with-lots-of-parts",
			expected: []pair{
				{"a", "string-with-lots-of-parts"},
				{"a-string", "with-lots-of-parts"},
				{"a-string-with", "lots-of-parts"},
				{"a-string-with-lots", "of-parts"},
				{"a-string-with-lots-of", "parts"},
			},
		},
	}
	for _, tt := range testTable {
		t.Run(tt.input, func(t *testing.T) {
			actual := splits(tt.input, "-")
			assert.DeepEqual(t, tt.expected, actual)
			for _, p := range actual {
				if p.First == "" {
					assert.Equal(t, tt.input, p.Second)
				} else if p.Second == "" {
					assert.Equal(t, tt.input, p.First)
				} else {
					assert.Equal(t, tt.input, p.First+"-"+p.Second)
				}
			}
		})
	}
}
