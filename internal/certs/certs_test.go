/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certs

import (
	"testing"

	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateCASecret(t *testing.T) {
	name := "ca-secret"
	cn := "www.example.com"
	ca_secret, err := GenerateSecret(name, cn, nil, 0, nil)
	if err != nil {
		t.Error(err)
	}
	data, ok := ca_secret.Data["tls.crt"]
	if !ok {
		t.Error("Invalid secret, tls.crt is missing")
	}
	cert, err := DecodeCertificate(data)

	if err != nil {
		t.Error("Error decoding certificate")
	}

	assert.Equal(t, 0, len(cert.DNSNames))
	assert.Equal(t, cn, cert.Issuer.CommonName)
	assert.Equal(t, cert.IsCA, true)
	assert.Equal(t, name, ca_secret.Name)
}

func TestGenerateSecret(t *testing.T) {
	ca_cn := "www.example.com"
	ca_secret, err := GenerateSecret("test-secret", ca_cn, []string{"134.565.56.77"}, 0, nil)
	if err != nil {
		t.Error(err)
	}
	my_secret_cn := "www.my.example.com"
	my_secret_host := "172.565.56.77"
	my_secret, err := GenerateSecret("my_secret", my_secret_cn, []string{my_secret_host}, 86400000000000 /*duration of 1 day*/, ca_secret)
	if err != nil {
		t.Error(err)
	}
	data, ok := my_secret.Data["tls.crt"]
	if !ok {
		t.Error("Invalid secret, tls.crt is missing")
	}
	my_cert, err := DecodeCertificate(data)
	if err != nil {
		t.Error("Error decoding certificate")
	}

	assert.Equal(t, my_secret_cn, my_cert.Subject.CommonName)
	assert.Equal(t, ca_cn, my_cert.Issuer.CommonName)

	// Instantiate the Secret object.
	caSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "emptyCASecret",
			Namespace: "testNamespace",
		},
		//  To create a 0-byte data field, initialize the map as empty.
		//  The type is map[string][]byte.
		Data: map[string][]byte{},
		Type: corev1.SecretTypeOpaque,
	}

	_, err = GenerateSecret("test-secret", ca_cn, []string{"134.565.56.77"}, 0, caSecret)
	errorText := err.Error()
	assert.Equal(t, errorText, "error reading CA Certificate from Secret \"emptyCASecret\": failed to read PEM encoded data from \"tls.crt\"")
}

// TestGH2284 exercises the ability to generate and parse x509 certificates
// with invalid empty DNS names.
//
// Prior to Skupper version 2.1.2 bug GH2277 resulted in the creation of many
// such certificates for CA and client use. A Go1.24.8 (and Go1.25.2) security
// fix briefly enabled strict validation that prevented the parsing of these
// certificates. These strict validations were relaxed in a subsequent point
// release.
func TestGH2284(t *testing.T) {
	invalidDNSHosts := []string{""}
	secret, err := GenerateSecret("issue-2284", "local.test", invalidDNSHosts, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	certBytes, ok := secret.Data["tls.crt"]
	if !ok {
		t.Fatal("Invalid secret, tls.crt is missing")
	}
	cert, err := DecodeCertificate(certBytes)

	const errMsgSANdNS = `x509: SAN dNSName is malformed`
	if err != nil {
		if err.Error() == errMsgSANdNS {
			t.Fatalf("Unexpected error %q: see https://github.com/skupperproject/skupper/issues/2284", err)
		}
		t.Fatalf("Unexpected error decoding certificate: %q", err)
	}

	assert.DeepEqual(t, cert.DNSNames, []string{""})
}
