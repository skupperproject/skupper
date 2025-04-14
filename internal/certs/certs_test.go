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
	"time"

	"gotest.tools/v3/assert"
)

func TestGenerateCASecret(t *testing.T) {
	host1 := "134.565.56.77"
	host2 := "172.345.82.7"
	name := "ca-secret"
	host := host1 + ", " + host2
	cn := "www.example.com"
	ca_secret := GenerateSecret(name, cn, host, 0, nil)
	data, ok := ca_secret.Data["tls.crt"]
	if !ok {
		t.Error("Invalid secret, tls.crt is missing")
	}
	cert, err := DecodeCertificate(data)

	if err != nil {
		t.Error("Error decoding certificate")
	}

	notBefore := time.Now()
	expiration := 5 * 365 * 24 * time.Hour
	notAfter := notBefore.Add(expiration)
	date_match := cert.NotAfter.Year() == notAfter.Year() && cert.NotAfter.YearDay() == notAfter.YearDay()

	assert.Equal(t, host1, cert.DNSNames[0])
	assert.Equal(t, host2, cert.DNSNames[1])
	assert.Equal(t, cn, cert.Issuer.CommonName)
	assert.Equal(t, cert.IsCA, true)
	assert.Equal(t, date_match, true)
	assert.Equal(t, name, ca_secret.Name)
}

func TestGenerateSecret(t *testing.T) {
	ca_cn := "www.example.com"
	ca_secret := GenerateSecret("test-secret", ca_cn, "134.565.56.77", 0, nil)
	my_secret_cn := "www.my.example.com"
	my_secret_host := "172.565.56.77"
	my_secret := GenerateSecret("my_secret", my_secret_cn, my_secret_host, 86400000000000 /*duration of 1 day*/, &ca_secret)
	data, ok := my_secret.Data["tls.crt"]
	if !ok {
		t.Error("Invalid secret, tls.crt is missing")
	}
	my_cert, err := DecodeCertificate(data)
	if err != nil {
		t.Error("Error decoding certificate")
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(86400000000000)
	date_match := my_cert.NotAfter.Year() == notAfter.Year() && my_cert.NotAfter.YearDay() == notAfter.YearDay()

	assert.Equal(t, my_secret_cn, my_cert.Subject.CommonName)
	assert.Equal(t, ca_cn, my_cert.Issuer.CommonName)
	assert.Equal(t, date_match, true)
}
