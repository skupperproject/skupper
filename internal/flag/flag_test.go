package flag

import (
	"flag"
	"testing"

	"gotest.tools/assert"
)

func Test_StringVar(t *testing.T) {
	tests := []struct {
		name          string
		defaultValue  string
		args          []string
		env           map[string]string
		expectedValue string
		expectedError string
	}{
		{
			name:          "default value returned",
			defaultValue:  "foo",
			expectedValue: "foo",
		},
		{
			name:          "flag specified as two args",
			defaultValue:  "foo",
			args:          []string{"-dummy", "bar"},
			expectedValue: "bar",
		},
		{
			name:          "flag specified as one arg",
			defaultValue:  "foo",
			args:          []string{"-dummy=bar"},
			expectedValue: "bar",
		},
		{
			name:         "env var returned",
			defaultValue: "foo",
			env: map[string]string{
				"SKUPPER_DUMMY": "bar",
			},
			expectedValue: "bar",
		},
		{
			name:          "invalid arg",
			defaultValue:  "foo",
			args:          []string{"-xyz=bar"},
			expectedError: "flag provided but not defined: -xyz",
			expectedValue: "foo",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &flag.FlagSet{}
			var value string
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			StringVar(flags, &value, "dummy", "SKUPPER_DUMMY", tt.defaultValue, "Test of dummy config option")
			err := flags.Parse(tt.args)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			}
			assert.Equal(t, value, tt.expectedValue)
		})
	}
}

func Test_BoolVar(t *testing.T) {
	tests := []struct {
		name          string
		defaultValue  bool
		args          []string
		env           map[string]string
		expectedValue bool
		expectedError string
	}{
		{
			name:          "default value returned",
			defaultValue:  true,
			expectedValue: true,
		},
		{
			name:          "flag specified as two args",
			args:          []string{"-dummy", "true"},
			expectedValue: true,
		},
		{
			name:          "flag specified as one arg",
			args:          []string{"-dummy=true"},
			expectedValue: true,
		},
		{
			name:          "flag overrides default",
			defaultValue:  true,
			args:          []string{"-dummy=false"},
			expectedValue: false,
		},
		{
			name: "env var returned",
			env: map[string]string{
				"SKUPPER_DUMMY": "true",
			},
			expectedValue: true,
		},
		{
			name:         "env var overrides default",
			defaultValue: true,
			env: map[string]string{
				"SKUPPER_DUMMY": "false",
			},
			expectedValue: false,
		},
		{
			name:         "invalid env var",
			defaultValue: true,
			env: map[string]string{
				"SKUPPER_DUMMY": "i am a bad value!",
			},
			expectedError: "i am a bad value",
			expectedValue: true,
		},
		{
			name: "error references env var name",
			env: map[string]string{
				"SKUPPER_DUMMY": "i am a bad value!",
			},
			expectedError: "SKUPPER_DUMMY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &flag.FlagSet{}
			var value bool
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			err := BoolVar(flags, &value, "dummy", "SKUPPER_DUMMY", tt.defaultValue, "Test of dummy config option")
			flags.Parse(tt.args)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			}
			assert.Equal(t, value, tt.expectedValue)
		})
	}
}

func Test_IntVar(t *testing.T) {
	tests := []struct {
		name          string
		defaultValue  int
		args          []string
		env           map[string]string
		expectedValue int
		expectedError string
	}{
		{
			name:          "default value returned",
			defaultValue:  123,
			expectedValue: 123,
		},
		{
			name:          "flag specified as two args",
			args:          []string{"-dummy", "123"},
			expectedValue: 123,
		},
		{
			name:          "flag specified as one arg",
			args:          []string{"-dummy=456"},
			expectedValue: 456,
		},
		{
			name:          "flag overrides default",
			defaultValue:  123,
			args:          []string{"-dummy=321"},
			expectedValue: 321,
		},
		{
			name: "env var returned",
			env: map[string]string{
				"SKUPPER_DUMMY": "999",
			},
			expectedValue: 999,
		},
		{
			name:         "env var overrides default",
			defaultValue: 123,
			env: map[string]string{
				"SKUPPER_DUMMY": "789",
			},
			expectedValue: 789,
		},
		{
			name:         "invalid env var",
			defaultValue: 555,
			env: map[string]string{
				"SKUPPER_DUMMY": "i am a bad value!",
			},
			expectedError: "i am a bad value",
			expectedValue: 555,
		},
		{
			name: "error references env var name",
			env: map[string]string{
				"SKUPPER_DUMMY": "i am a bad value!",
			},
			expectedError: "SKUPPER_DUMMY",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags := &flag.FlagSet{}
			var value int
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			err := IntVar(flags, &value, "dummy", "SKUPPER_DUMMY", tt.defaultValue, "Test of dummy config option")
			flags.Parse(tt.args)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			}
			assert.Equal(t, value, tt.expectedValue)
		})
	}
}
