package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCmdTokenRedeem_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		flags         common.CommandTokenRedeemFlags
		expectedError string
	}

	// create temp token file for tests
	_, err := os.Create("/tmp/token-redeem.yaml")
	assert.Check(t, err == nil)

	defer os.Remove("/tmp/token-redeem.yaml")

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err = os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)

	testTable := []test{
		{
			name:          "file name is not specified",
			args:          []string{},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "token file name must be configured",
		},
		{
			name:          "file name is empty",
			args:          []string{""},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "file name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my-grant", "/home/user/my-grant.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "token file name is not valid.",
			args:          []string{"my new file"},
			flags:         common.CommandTokenRedeemFlags{Timeout: 60 * time.Second},
			expectedError: "token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name:          "flags all valid",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdTokenRedeem{}
			command.Namespace = "test"
			command.tokenHandler = fs.NewTokenHandler(command.Namespace)
			command.accessTokenHandler = fs.NewAccessTokenHandler(command.Namespace)

			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdTokenRedeem_Run(t *testing.T) {
	type test struct {
		name         string
		errorMessage string
	}

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err := os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)

	tmpDirToken := t.TempDir()
	tokenFile := filepath.Join(tmpDirToken, "token.yaml")

	testTable := []test{
		{
			name:         "token redeemed",
			errorMessage: "",
		},
		{
			name:         "token is not redeemed",
			errorMessage: "resource name is required",
		},
	}

	for _, test := range testTable {

		command := &CmdTokenRedeem{}
		command.Namespace = "test"
		command.tokenHandler = fs.NewTokenHandler(command.Namespace)
		command.accessTokenHandler = fs.NewAccessTokenHandler(command.Namespace)
		command.fileName = tokenFile

		if test.errorMessage == "" {
			err = newAccessTokenFile(tokenFile, false)
		} else {
			err = newAccessTokenFile(tokenFile, true)
		}

		assert.Check(t, err == nil)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, test.errorMessage == err.Error())
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods

func newAccessTokenFile(fileName string, withErrors bool) error {

	var resource v2alpha1.AccessToken
	if withErrors {
		resource = v2alpha1.AccessToken{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "AccessToken",
			},
			ObjectMeta: metav1.ObjectMeta{},
		}

	} else {
		resource = v2alpha1.AccessToken{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "skupper.io/v2alpha1",
				Kind:       "AccessToken",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "token",
			},
			Spec: v2alpha1.AccessTokenSpec{
				Url:  "AAA",
				Ca:   "BBB",
				Code: "CCC",
			},
		}
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("Could not write to file " + fileName + ": " + err.Error())
	}

	err = s.Encode(&resource, out)
	if err != nil {
		return fmt.Errorf("Could not write out generated token: " + err.Error())
	}

	defer out.Close()
	return nil

}
