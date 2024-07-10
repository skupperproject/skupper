package kube

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"gotest.tools/assert"
)

func TestGenerateConsoleSessionCredentials(t *testing.T) {
	zeros := bytes.NewReader(make([]byte, 128))
	const zerosEncoded = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

	exampleText := "GenerateConsoleSessionCredentials"
	exampleTextEncoded := "R2VuZXJhdGVDb25zb2xlU2Vzc2lvbkNyZWRlbnRpYWw="

	testcases := []struct {
		Input     io.Reader
		CheckData func(map[string][]byte) error
	}{
		{
			Input: zeros,
			CheckData: func(data map[string][]byte) error {
				if string(data["session_secret"]) != zerosEncoded {
					return fmt.Errorf("session secret should have been %q but got %v", zerosEncoded, data)
				}
				return nil
			},
		},
		{
			Input: nil,
			CheckData: func(data map[string][]byte) error {
				if len(data["session_secret"]) != 44 {
					return fmt.Errorf("session secret should have been 44 bytes long but got %v", data)
				}
				return nil
			},
		},
		{
			Input: rand.Reader,
			CheckData: func(data map[string][]byte) error {
				if len(data["session_secret"]) != 44 {
					return fmt.Errorf("session secret should have been 44 bytes long but got %v", data)
				}
				return nil
			},
		},
		{
			Input: bytes.NewReader([]byte(exampleText)),
			CheckData: func(data map[string][]byte) error {
				if string(data["session_secret"]) != exampleTextEncoded {
					return fmt.Errorf("session secret should have been %q but got %v", exampleTextEncoded, data)
				}
				return nil
			},
		},
	}

	for _, tc := range testcases {
		t.Run("", func(t *testing.T) {
			creds, err := GenerateConsoleSessionCredentials(tc.Input)
			assert.Assert(t, err)
			assert.Equal(t, creds.Name, types.ConsoleSessionSecret)
			assert.Assert(t, tc.CheckData(creds.Data))
		})
	}
}
