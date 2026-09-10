package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
)

func TestCmdSystemPrometheus_ValidateInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no arguments"},
		{name: "arguments are accepted", args: []string{"something"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSystemPrometheus{Flags: &common.CommandPrometheusFlags{}}
			if err := cmd.ValidateInput(test.args); err != nil {
				t.Fatalf("expected no error, got %q", err)
			}
		})
	}
}

func TestCmdSystemPrometheus_Run(t *testing.T) {
	// The kube Run prints a not-supported message and always returns nil.
	cmd := &CmdSystemPrometheus{Flags: &common.CommandPrometheusFlags{}}
	if err := cmd.Run(); err != nil {
		t.Fatalf("expected no error, got %q", err)
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
