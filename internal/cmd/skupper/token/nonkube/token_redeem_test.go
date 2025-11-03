package nonkube

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestCmdTokenRedeem_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		namespace     string
		args          []string
		flags         common.CommandTokenRedeemFlags
		expectedError string
	}

	// create temp token file for tests
	_, err := os.Create("/tmp/token-redeem.yaml")
	assert.Check(t, err == nil)

	defer os.Remove("/tmp/token-redeem.yaml")

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}

	testTable := []test{
		{
			name:          "file name is not specified",
			namespace:     "test",
			args:          []string{},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "token file name must be configured",
		},
		{
			name:          "file name is empty",
			namespace:     "test",
			args:          []string{""},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "file name must not be empty",
		},
		{
			name:          "more than one argument is specified",
			namespace:     "test",
			args:          []string{"my-grant", "/home/user/my-grant.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "only one argument is allowed for this command",
		},
		{
			name:          "token file name is not valid.",
			namespace:     "test",
			args:          []string{"my new file"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
		},
		{
			name:          "invalid namespace",
			namespace:     "Test5",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "namespace is not valid: value does not match this regular expression: ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
		},
		{
			name:          "flags all valid",
			namespace:     "test",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "",
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdTokenRedeem{}
			command.Namespace = test.namespace
			command.Flags = &test.flags

			testutils.CheckValidateInput(t, command, test.expectedError, test.args)
		})
	}
}

func TestCmdTokenRedeem_Run(t *testing.T) {
	type test struct {
		name         string
		errorMessage string
		errorType    string
	}

	if os.Getuid() == 0 {
		api.DefaultRootDataHome = t.TempDir()
	} else {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
	}

	tmpDirToken := t.TempDir()
	tokenFile := filepath.Join(tmpDirToken, "token.yaml")

	testTable := []test{
		{
			name:         "token redeemed",
			errorMessage: "",
		},
		{
			name:         "token is not redeemed",
			errorMessage: "token name is required",
			errorType:    "noResourceName",
		},
		{
			name:         "malformed file is trying to be redeemed ",
			errorMessage: "couldn't get version/kind; json parse error:",
			errorType:    "malformedFile",
		},
	}

	for _, test := range testTable {

		command := &CmdTokenRedeem{}
		command.Namespace = "test"
		command.linkHandler = fs.NewLinkHandler(command.Namespace)
		command.secretHandler = fs.NewSecretHandler(command.Namespace)
		command.fileName = tokenFile

		var err error
		switch test.errorType {
		case "malformedFile":
			err = newMalformedTokenFile(tokenFile)
		case "noResourceName":
			err = newAccessTokenFile(tokenFile, true)
		default:
			err = newAccessTokenFile(tokenFile, false)
		}
		assert.Check(t, err == nil)

		t.Run(test.name, func(t *testing.T) {

			err := command.Run()
			if err != nil {
				assert.Check(t, strings.HasPrefix(err.Error(), test.errorMessage), fmt.Sprintf("Expected: %s, Found: %s", test.errorMessage, err.Error()))
			} else {
				assert.Check(t, err == nil)
			}
		})
	}
}

// --- helper methods
func newMalformedTokenFile(fileName string) error {
	content := []byte("Not an AccessToken")
	err := os.WriteFile(fileName, content, 0644)
	return err
}

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
		return fmt.Errorf("could not write to file %s: %v", fileName, err)
	}

	err = s.Encode(&resource, out)
	if err != nil {
		return fmt.Errorf("could not write out generated token: %v", err)
	}

	defer out.Close()
	return nil

}
