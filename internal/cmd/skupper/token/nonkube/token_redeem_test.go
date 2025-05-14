package nonkube

import (
	"fmt"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	"github.com/skupperproject/skupper/internal/nonkube/client/fs"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdTokenRedeem_ValidateInput(t *testing.T) {
	type test struct {
		name          string
		args          []string
		flags         common.CommandTokenRedeemFlags
		expectedError string
		siteCreated   bool
	}

	// create temp token file for tests
	_, err := os.Create("/tmp/token-redeem.yaml")
	assert.Check(t, err == nil)

	defer os.Remove("/tmp/token-redeem.yaml")

	tmpDir := filepath.Join(t.TempDir(), "/skupper")
	err = os.Setenv("SKUPPER_OUTPUT_PATH", tmpDir)
	assert.Check(t, err == nil)
	path := filepath.Join(tmpDir, "/namespaces/test", string(api.RuntimeSiteStatePath))

	testTable := []test{
		{
			name:          "redeem a token without a site",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "A site must be active in namespace \"test\" before a token can be redeemed",
			siteCreated:   false,
		},
		{
			name:          "file name is not specified",
			args:          []string{},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "token file name must be configured",
			siteCreated:   true,
		},
		{
			name:          "file name is empty",
			args:          []string{""},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "file name must not be empty",
			siteCreated:   true,
		},
		{
			name:          "more than one argument is specified",
			args:          []string{"my-grant", "/home/user/my-grant.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "only one argument is allowed for this command",
			siteCreated:   true,
		},
		{
			name:          "token file name is not valid.",
			args:          []string{"my new file"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "token file name is not valid: value does not match this regular expression: ^[A-Za-z0-9./~-]+$",
			siteCreated:   true,
		},
		{
			name:          "flags all valid",
			args:          []string{"/tmp/token-redeem.yaml"},
			flags:         common.CommandTokenRedeemFlags{},
			expectedError: "",
			siteCreated:   true,
		},
	}

	for _, test := range testTable {
		t.Run(test.name, func(t *testing.T) {

			command := &CmdTokenRedeem{}
			command.Namespace = "test"
			command.Flags = &test.flags

			if test.siteCreated {
				siteResource := v2alpha1.Site{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "skupper.io/v2alpha1",
						Kind:       "Site",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "the-site",
						Namespace: command.Namespace,
					},
				}

				siteHandler := fs.NewSiteHandler(command.Namespace)

				content, err := siteHandler.EncodeToYaml(siteResource)
				assert.Check(t, err == nil)
				err = siteHandler.WriteFile(path, "my-site.yaml", content, common.Sites)
				assert.Check(t, err == nil)

				defer siteHandler.Delete("the-site")
			}

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
		command.siteState = &api.SiteState{
			Site: &v2alpha1.Site{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "skupper.io/v2alpha1",
					Kind:       "Site",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "the-site",
					Namespace: command.Namespace,
				},
			},
		}

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
		return fmt.Errorf("Could not write to file " + fileName + ": " + err.Error())
	}

	err = s.Encode(&resource, out)
	if err != nil {
		return fmt.Errorf("Could not write out generated token: " + err.Error())
	}

	defer out.Close()
	return nil

}
