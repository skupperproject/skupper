package qdr

import (
	"testing"

	"github.com/skupperproject/skupper/api/types"
)

func TestParseRouterLogConfig(t *testing.T) {
	var tests = []struct {
		input    string
		err      bool
		expected map[string]types.RouterLogConfig
	}{
		{"notice", false, map[string]types.RouterLogConfig{
			"": types.RouterLogConfig{
				Level: "notice",
			},
		}},
		{"notice", false, map[string]types.RouterLogConfig{
			"": types.RouterLogConfig{
				Level: "notice",
			},
		}},
		{"trace", false, map[string]types.RouterLogConfig{
			"": types.RouterLogConfig{
				Level: "trace",
			},
		}},
		{"info,PROTOCOL:trace", false, map[string]types.RouterLogConfig{
			"": types.RouterLogConfig{
				Level: "info",
			},
			"PROTOCOL": types.RouterLogConfig{
				Module: "PROTOCOL",
				Level:  "trace",
			},
		}},
		{"TCP_ADAPTOR:debug,PROTOCOL:trace,POLICY:notice", false, map[string]types.RouterLogConfig{
			"TCP_ADAPTOR": types.RouterLogConfig{
				Module: "TCP_ADAPTOR",
				Level:  "debug",
			},
			"PROTOCOL": types.RouterLogConfig{
				Module: "PROTOCOL",
				Level:  "trace",
			},
			"POLICY": types.RouterLogConfig{
				Module: "POLICY",
				Level:  "notice",
			},
		}},
		{"UNRECOGNISED:debug,PROTOCOL:trace,POLICY:notice", true, map[string]types.RouterLogConfig{}},
		{"PROTOCOL:everything,POLICY:notice", true, map[string]types.RouterLogConfig{}},
	}
	for _, test := range tests {
		actual, err := ParseRouterLogConfig(test.input)
		if test.err && err == nil {
			t.Errorf("Expected error for %s", test.input)
		} else if !test.err && err != nil {
			t.Errorf("Got error for %s:  %s", test.input, err)
		} else {
			for _, i := range actual {
				if test.expected[i.Module] != i {
					t.Errorf("Expected %v got %v", test.expected[i.Module], i)
				}
				delete(test.expected, i.Module)
			}
			for _, v := range test.expected {
				t.Errorf("%v not found", v)
			}
		}
	}
}

func TestRouterLogConfigToString(t *testing.T) {
	var tests = []struct {
		input    []types.RouterLogConfig
		expected string
	}{
		{[]types.RouterLogConfig{
			types.RouterLogConfig{
				Level: "notice",
			},
		}, "notice"},
		{[]types.RouterLogConfig{
			types.RouterLogConfig{
				Module: "PROTOCOL",
				Level:  "debug",
			},
			types.RouterLogConfig{
				Level: "notice",
			},
		}, "PROTOCOL:debug,notice"},
		{[]types.RouterLogConfig{
			types.RouterLogConfig{
				Module: "PROTOCOL",
				Level:  "debug",
			},
			types.RouterLogConfig{
				Module: "HTTP_ADAPTOR",
				Level:  "trace",
			},
			types.RouterLogConfig{
				Module: "POLICY",
				Level:  "error",
			},
		}, "PROTOCOL:debug,HTTP_ADAPTOR:trace,POLICY:error"},
	}
	for _, test := range tests {
		actual := RouterLogConfigToString(test.input)
		if test.expected != actual {
			t.Errorf("Expected %s got %s", test.expected, actual)
		}
	}
}

func TestPreserveLogString(t *testing.T) {
	inputs := []string{
		"notice",
		"notice+",
		"notice,PROTOCOL:debug",
		"POLICY:notice,PROTOCOL:debug,TCP_ADAPTOR:trace",
	}
	for _, input := range inputs {
		config, err := ParseRouterLogConfig(input)
		if err != nil {
			t.Errorf("Got error for %s:  %s", input, err)
		}
		actual := RouterLogConfigToString(config)
		if input != actual {
			t.Errorf("Expected %s got %s", input, actual)
		}
	}
}

func TestConfigureRouterLogging(t *testing.T) {
	var tests = []struct {
		input    string
		expected bool
	}{
		{"notice", true},
		{"notice+", false},
		{"DEFAULT:notice+", false},
		{"DEFAULT:info+", true},
	}
	config := RouterConfig{}
	for _, test := range tests {
		parsed, err := ParseRouterLogConfig(test.input)
		if err != nil {
			t.Errorf("Invalid input: %s", err)
		} else {
			actual := ConfigureRouterLogging(&config, parsed)
			if actual != test.expected {
				t.Errorf("Wrong return value; expected %t got %t", test.expected, actual)

			}
		}
	}
}
