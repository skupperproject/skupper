package nonkube

import (
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/spf13/cobra"
)

func TestCmdSystemNetworkObserverValidateInput(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		flags         *common.CommandNetworkObserverFlags
		expectedError []string
	}{
		{
			name:          "rejects arguments",
			args:          []string{"extra"},
			expectedError: []string{"this command does not accept arguments"},
		},
		{
			name: "rejects credentials with uninstall",
			flags: &common.CommandNetworkObserverFlags{
				Uninstall: true,
				Password:  "password",
			},
			expectedError: []string{
				"--password cannot be used with --uninstall",
			},
		},
		{
			name: "allows install credentials",
			flags: &common.CommandNetworkObserverFlags{
				Username: "user",
				Password: "password",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSystemNetworkObserver{Flags: test.flags}

			err := cmd.ValidateInput(test.args)
			if len(test.expectedError) == 0 {
				if err != nil {
					t.Fatalf("expected no error, got %q", err.Error())
				}
				return
			}

			if err == nil {
				t.Fatal("expected error")
			}
			for _, expected := range test.expectedError {
				if !strings.Contains(err.Error(), expected) {
					t.Fatalf("expected validation error %q, got %q", expected, err.Error())
				}
			}
		})
	}
}

func TestCmdSystemNetworkObserverNewClient(t *testing.T) {
	tests := []struct {
		name              string
		flagNamespace     string
		initialNamespace  string
		expectedNamespace string
	}{
		{
			name:              "uses namespace flag",
			flagNamespace:     "west",
			expectedNamespace: "west",
		},
		{
			name:              "defaults namespace",
			expectedNamespace: "default",
		},
		{
			name:              "defaults even when namespace was preset without flag",
			initialNamespace:  "east",
			expectedNamespace: "default",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cobraCmd := &cobra.Command{}
			cobraCmd.Flags().String(common.FlagNameNamespace, "", "")
			if test.flagNamespace != "" {
				if err := cobraCmd.Flags().Set(common.FlagNameNamespace, test.flagNamespace); err != nil {
					t.Fatalf("failed to set namespace flag: %v", err)
				}
			}

			cmd := &CmdSystemNetworkObserver{
				CobraCmd:  cobraCmd,
				namespace: test.initialNamespace,
			}

			cmd.NewClient(cobraCmd, nil)

			if cmd.namespace != test.expectedNamespace {
				t.Fatalf("expected namespace %q, got %q", test.expectedNamespace, cmd.namespace)
			}
		})
	}
}

func TestCmdSystemNetworkObserverInputToOptions(t *testing.T) {
	tests := []struct {
		name             string
		flags            *common.CommandNetworkObserverFlags
		initialUser      string
		initialPassword  string
		expectedUser     string
		expectedPassword string
	}{
		{
			name:             "copies credentials from flags",
			flags:            &common.CommandNetworkObserverFlags{Username: "user", Password: "password"},
			expectedUser:     "user",
			expectedPassword: "password",
		},
		{
			name:             "leaves existing values when flags are empty",
			flags:            &common.CommandNetworkObserverFlags{},
			initialUser:      "existing-user",
			initialPassword:  "existing-password",
			expectedUser:     "existing-user",
			expectedPassword: "existing-password",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := &CmdSystemNetworkObserver{
				Flags:    test.flags,
				user:     test.initialUser,
				password: test.initialPassword,
			}

			cmd.InputToOptions()

			if cmd.user != test.expectedUser {
				t.Fatalf("expected user %q, got %q", test.expectedUser, cmd.user)
			}
			if cmd.password != test.expectedPassword {
				t.Fatalf("expected password %q, got %q", test.expectedPassword, cmd.password)
			}
		})
	}
}
