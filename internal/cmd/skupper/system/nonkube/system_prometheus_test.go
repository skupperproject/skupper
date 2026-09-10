package nonkube

import (
	"fmt"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
)

func TestCmdSystemPrometheus_ValidateInput(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		expectedError string
	}{
		{
			name: "no arguments accepted",
			args: nil,
		},
		{
			name:          "rejects arguments",
			args:          []string{"extra"},
			expectedError: "this command does not accept arguments",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSystemPrometheus{Flags: &common.CommandPrometheusFlags{}}

			err := cmd.ValidateInput(test.args)
			if test.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %q", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), test.expectedError) {
				t.Fatalf("expected error %q, got %q", test.expectedError, err.Error())
			}
		})
	}
}

func TestCmdSystemPrometheus_Run(t *testing.T) {
	tests := []struct {
		name          string
		flags         *common.CommandPrometheusFlags
		install       func() error
		uninstall     func() error
		expectedError string
	}{
		{
			name:    "install succeeds",
			flags:   &common.CommandPrometheusFlags{Uninstall: false},
			install: func() error { return nil },
		},
		{
			name:          "install fails",
			flags:         &common.CommandPrometheusFlags{Uninstall: false},
			install:       func() error { return fmt.Errorf("disk full") },
			expectedError: "installation failed: disk full",
		},
		{
			name:          "install with nil installer (NewClient failed)",
			flags:         &common.CommandPrometheusFlags{Uninstall: false},
			install:       nil,
			expectedError: "failed to create prometheus installer",
		},
		{
			name:      "uninstall succeeds",
			flags:     &common.CommandPrometheusFlags{Uninstall: true},
			uninstall: func() error { return nil },
		},
		{
			name:          "uninstall fails",
			flags:         &common.CommandPrometheusFlags{Uninstall: true},
			uninstall:     func() error { return fmt.Errorf("container not found") },
			expectedError: "uninstallation failed: container not found",
		},
		{
			name:          "uninstall with nil installer (NewClient failed)",
			flags:         &common.CommandPrometheusFlags{Uninstall: true},
			uninstall:     nil,
			expectedError: "failed to create prometheus installer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSystemPrometheus{
				Flags:     test.flags,
				Install:   test.install,
				Uninstall: test.uninstall,
			}

			err := cmd.Run()
			if test.expectedError == "" {
				if err != nil {
					t.Fatalf("expected no error, got %q", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != test.expectedError {
				t.Fatalf("expected error %q, got %q", test.expectedError, err.Error())
			}
		})
	}
}

func TestCmdSystemPrometheus_WaitUntil(t *testing.T) {
	cmd := &CmdSystemPrometheus{}
	if err := cmd.WaitUntil(); err != nil {
		t.Fatalf("expected no error, got %q", err)
	}
}

func TestCmdSystemPrometheus_InputToOptions(t *testing.T) {
	cmd := &CmdSystemPrometheus{Flags: &common.CommandPrometheusFlags{}}
	cmd.InputToOptions()
}
