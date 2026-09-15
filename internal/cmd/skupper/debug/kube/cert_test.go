package kube

import (
	"testing"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/testutils"
	fakeclient "github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCmdDebugCert_ValidateInput(t *testing.T) {
	type test struct {
		name           string
		args           []string
		flags          common.CommandDebugCertFlags
		k8sObjects     []runtime.Object
		skupperObjects []runtime.Object
		expectedError  string
	}

	testTable := []test{
		{
			name:          "more than one argument",
			args:          []string{"cert-a", "cert-b"},
			expectedError: "only one certificate name is allowed",
		},
		{
			name:          "file and name argument",
			flags:         common.CommandDebugCertFlags{File: "tls.crt"},
			args:          []string{"my-cert"},
			expectedError: "file flag cannot be used with a certificate name argument",
		},
		{
			name:          "invalid output format",
			flags:         common.CommandDebugCertFlags{Output: "xml"},
			expectedError: "output type is not valid: value xml not allowed. It should be one of this options: [json yaml]",
		},
	}

	for _, tt := range testTable {
		t.Run(tt.name, func(t *testing.T) {
			cmd, err := newCmdDebugCertWithMocks("test", tt.k8sObjects, tt.skupperObjects, "")
			assert.Assert(t, err)
			cmd.Flags = &tt.flags
			testutils.CheckValidateInput(t, cmd, tt.expectedError, tt.args)
		})
	}
}

func TestCmdDebugCert_certInfoFromSecret(t *testing.T) {
	caSecret, err := certs.GenerateSecret("test-ca", "ca.example.com", nil, 0, nil)
	assert.NilError(t, err)

	leafSecret, err := certs.GenerateSecret("my-cert", "my.example.com", []string{"my.example.com"}, 86400000000000, caSecret)
	assert.NilError(t, err)

	cmd := NewCmdDebugCert()
	info, err := cmd.certInfoFromSecret("my-cert", leafSecret, "Ready", "2027-01-01T00:00:00Z")
	assert.NilError(t, err)
	assert.Equal(t, "my-cert", info.Name)
	assert.Equal(t, "my.example.com", info.Subject)
	assert.Equal(t, "Ready", info.Status)
	assert.Equal(t, "2027-01-01T00:00:00Z", info.CrExpiration)
}

func TestCmdDebugCert_RunWithCertificateCR(t *testing.T) {
	leafSecret, err := certs.GenerateSecret("skupper-local-server", "local.example.com", []string{"local.example.com"}, 0, nil)
	assert.NilError(t, err)
	leafSecret.Namespace = "test"

	certCR := &v2alpha1.Certificate{
		ObjectMeta: v1.ObjectMeta{Name: "skupper-local-server", Namespace: "test"},
		Status: v2alpha1.CertificateStatus{
			Status: v2alpha1.Status{
				StatusType: v2alpha1.StatusReady,
			},
			Expiration: "2027-01-01T00:00:00Z",
		},
	}

	cmd, err := newCmdDebugCertWithMocks("test", []runtime.Object{leafSecret}, []runtime.Object{certCR}, "")
	assert.Assert(t, err)
	cmd.certName = "skupper-local-server"

	err = cmd.Run()
	assert.NilError(t, err)
}

func newCmdDebugCertWithMocks(namespace string, k8sObjects []runtime.Object, skupperObjects []runtime.Object, fakeSkupperError string) (*CmdDebugCert, error) {
	client, err := fakeclient.NewFakeClient(namespace, k8sObjects, skupperObjects, fakeSkupperError)
	if err != nil {
		return nil, err
	}
	return &CmdDebugCert{
		Client:     client.GetSkupperClient().SkupperV2alpha1(),
		KubeClient: client.GetKubeClient(),
		Namespace:  namespace,
	}, nil
}
