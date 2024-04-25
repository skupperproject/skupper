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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"
)

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}

func pemBlockForKey(priv interface{}) *pem.Block {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(k)}
	default:
		return nil
	}
}

type CertificateAuthority struct {
	Certificate *x509.Certificate
	Key         interface{}
	CrtData     []byte
}

type CertificateData map[string][]byte

func decodeDataElement(in []byte, name string) []byte {
	block, _ := pem.Decode(in)
	if block == nil {
		log.Fatal("failed to decode PEM block of type " + name)
	}
	return block.Bytes
}

func getCAFromSecret(secret *corev1.Secret) CertificateAuthority {
	cert, err := x509.ParseCertificate(decodeDataElement(secret.Data["tls.crt"], "certificate"))
	if err != nil {
		log.Fatal("failed to get CA certificate from secret")
	}
	key, err := x509.ParsePKCS1PrivateKey(decodeDataElement(secret.Data["tls.key"], "private key"))
	if err != nil {
		log.Fatal("failed to get CA private key from secret", err)
	}
	return CertificateAuthority{
		Certificate: cert,
		Key:         key,
		CrtData:     secret.Data["tls.crt"],
	}
}

func generateSecret(name string, subject string, hosts string, ca *CertificateAuthority, expiration time.Duration) corev1.Secret {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate private key: %s", err)
	}

	notBefore := time.Now()
	if expiration == 0 {
		expiration = 5 * 365 * 24 * time.Hour
	}
	notAfter := notBefore.Add(expiration)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		log.Fatalf("failed to generate serial number: %s", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: subject,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
	}

	hosts_list := strings.Split(hosts, ",")
	for _, h := range hosts_list {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		}
		template.DNSNames = append(template.DNSNames, h)
	}

	var parent *x509.Certificate
	var cakey interface{}
	if ca == nil {
		// self signed
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
		parent = &template
		cakey = priv
	} else {
		parent = ca.Certificate
		cakey = ca.Key
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, parent, publicKey(priv), cakey)
	if err != nil {
		log.Fatalf("Failed to create certificate: %s", err)
	}

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Type: "kubernetes.io/tls",
		Data: map[string][]byte{},
	}

	certString := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyString := pem.EncodeToMemory(pemBlockForKey(priv))

	secret.Data["tls.crt"] = []byte(certString)
	secret.Data["tls.key"] = []byte(keyString)
	if ca != nil {
		secret.Data["ca.crt"] = ca.CrtData
	}

	return secret
}

func generateSimpleSecretWithCA(name string, ca *CertificateAuthority) corev1.Secret {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: map[string][]byte{},
	}

	secret.Data["ca.crt"] = ca.CrtData

	return secret
}

func SecretToCertData(secret corev1.Secret) CertificateData {
	certData := CertificateData{}
	for k, v := range secret.Data {
		certData[k] = v
	}
	return certData
}

func CertDataToSecret(name string, certData CertificateData, annotations map[string]string) corev1.Secret {
	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: annotations,
		},
		Type: "kubernetes.io/tls",
		Data: map[string][]byte{},
	}
	for k, v := range certData {
		secret.Data[k] = v
	}
	return secret
}

func GenerateSecret(name string, subject string, hosts string, ca *corev1.Secret) corev1.Secret {
	return GenerateSecretWithExpiration(name, subject, hosts, 0, ca)
}

func GenerateSecretWithExpiration(name string, subject string, hosts string, expiration time.Duration, ca *corev1.Secret) corev1.Secret {
	var caCert *CertificateAuthority
	if ca != nil {
		caFromSecret := getCAFromSecret(ca)
		caCert = &caFromSecret
	}
	return generateSecret(name, subject, hosts, caCert, expiration)
}

func GenerateCASecret(name string, subject string) corev1.Secret {
	return generateSecret(name, subject, "", nil, 0)
}

func GenerateSimpleSecret(name string, ca *corev1.Secret) corev1.Secret {
	caCert := getCAFromSecret(ca)
	return generateSimpleSecretWithCA(name, &caCert)
}

func AnnotateConnectionToken(secret *corev1.Secret, role string, host string, port string) {
	if secret.ObjectMeta.Annotations == nil {
		secret.ObjectMeta.Annotations = map[string]string{}
	}
	secret.ObjectMeta.Annotations[role+"-host"] = host
	secret.ObjectMeta.Annotations[role+"-port"] = port
}

func GenerateSecretFile(secretFile string, secret *corev1.Secret, localOnly bool) error {
	// generate yaml and save it to the specified path
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	out, err := os.Create(secretFile)
	if err != nil {
		return fmt.Errorf("Could not write to file " + secretFile + ": " + err.Error())
	}
	err = s.Encode(secret, out)
	if err != nil {
		return fmt.Errorf("Could not write out generated secret: " + err.Error())
	}
	var extra string
	if localOnly {
		extra = "(Note: token will only be valid for local cluster)"
	}
	fmt.Printf("Connection token written to %s %s", secretFile, extra)
	fmt.Println()
	return nil
}

func DecodeCertificate(data []byte) (*x509.Certificate, error) {
	b, _ := pem.Decode(data)
	if b == nil {
		return nil, fmt.Errorf("Could not decode PEM block from data")
	}
	return x509.ParseCertificate(b.Bytes)
}

func GetTlsCrtHostnames(tlscrtData []byte) (subject string, hostnames []string, err error) {
	b, _ := pem.Decode(tlscrtData)
	if b == nil {
		return "", nil, fmt.Errorf("error decoding certificate data")
	}
	cert, err := x509.ParseCertificate(b.Bytes)
	if err != nil {
		return "", nil, err
	}
	subject = cert.Subject.CommonName
	for _, name := range cert.DNSNames {
		hostnames = append(hostnames, name)
	}
	return subject, hostnames, nil
}

type TlsConfigRetriever struct {
	Cert   string
	Key    string
	Ca     string
	Verify bool
}

func (paths *TlsConfigRetriever) GetTlsConfig() (*tls.Config, error) {
	tlsConfig, err := getTlsConfig(paths.Verify, paths.Cert, paths.Key, paths.Ca)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

func GetTlsConfigRetriever(verify bool, cert, key, ca string) *TlsConfigRetriever {
	return &TlsConfigRetriever{
		Cert:   cert,
		Key:    key,
		Ca:     ca,
		Verify: verify,
	}
}

func getTlsConfig(verify bool, cert, key, ca string) (*tls.Config, error) {
	var config tls.Config
	config.InsecureSkipVerify = true
	if verify {
		certPool := x509.NewCertPool()
		file, err := os.ReadFile(ca)
		if err != nil {
			return nil, err
		}
		certPool.AppendCertsFromPEM(file)
		config.RootCAs = certPool
		config.InsecureSkipVerify = false
	}

	_, errCert := os.Stat(cert)
	_, errKey := os.Stat(key)
	if errCert == nil || errKey == nil {
		tlsCert, err := tls.LoadX509KeyPair(cert, key)
		if err != nil {
			log.Fatal("Could not load x509 key pair", err.Error())
		}
		config.Certificates = []tls.Certificate{tlsCert}
	}
	config.MinVersion = tls.VersionTLS10

	return &config, nil
}
