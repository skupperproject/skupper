package cert

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/skupperproject/skupper/internal/certs"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
)

// Info holds the decoded fields of an X.509 certificate for display.
type Info struct {
	Name               string   `json:"name"`
	Subject            string   `json:"subject"`
	Issuer             string   `json:"issuer"`
	SerialNumber       string   `json:"serialNumber"`
	NotBefore          string   `json:"notBefore"`
	NotAfter           string   `json:"notAfter"`
	DNSNames           []string `json:"dnsNames,omitempty"`
	IPAddresses        []string `json:"ipAddresses,omitempty"`
	EmailAddresses     []string `json:"emailAddresses,omitempty"`
	URIs               []string `json:"uris,omitempty"`
	IsCA               bool     `json:"isCA"`
	PublicKeyAlgorithm string   `json:"publicKeyAlgorithm"`
	PublicKeySize      int      `json:"publicKeySize"`
	SignatureAlgorithm string   `json:"signatureAlgorithm"`
	Status             string   `json:"status,omitempty"`
	CrExpiration       string   `json:"crExpiration,omitempty"`
}

func ParseCertificate(name string, certData []byte) (*Info, error) {
	cert, err := certs.DecodeCertificate(certData)
	if err != nil {
		return nil, err
	}
	return certToInfo(name, cert), nil
}

func ParseCertificateFile(name, path string) (*Info, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseCertificate(name, data)
}

func certToInfo(name string, cert *x509.Certificate) *Info {
	algo, size := publicKeyInfo(cert)
	ips := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		ips[i] = ip.String()
	}
	uris := make([]string, len(cert.URIs))
	for i, uri := range cert.URIs {
		uris[i] = uri.String()
	}
	return &Info{
		Name:               name,
		Subject:            formatName(cert.Subject),
		Issuer:             formatName(cert.Issuer),
		SerialNumber:       cert.SerialNumber.String(),
		NotBefore:          cert.NotBefore.Format(time.RFC3339),
		NotAfter:           cert.NotAfter.Format(time.RFC3339),
		DNSNames:           cert.DNSNames,
		IPAddresses:        ips,
		EmailAddresses:     cert.EmailAddresses,
		URIs:               uris,
		IsCA:               cert.IsCA,
		PublicKeyAlgorithm: algo,
		PublicKeySize:      size,
		SignatureAlgorithm: cert.SignatureAlgorithm.String(),
	}
}

func formatName(name pkix.Name) string {
	if name.CommonName != "" {
		return name.CommonName
	}
	return name.String()
}

func publicKeyInfo(cert *x509.Certificate) (algorithm string, size int) {
	switch pub := cert.PublicKey.(type) {
	case *rsa.PublicKey:
		return "RSA", pub.N.BitLen()
	case *ecdsa.PublicKey:
		return "ECDSA", pub.Curve.Params().BitSize
	default:
		return fmt.Sprintf("%T", cert.PublicKey), 0
	}
}

func formatSANs(info *Info) string {
	var parts []string
	parts = append(parts, info.DNSNames...)
	parts = append(parts, info.IPAddresses...)
	parts = append(parts, info.EmailAddresses...)
	parts = append(parts, info.URIs...)
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

// Display renders one or more certificates to stdout.
func Display(infos []Info, output string, detail bool) error {
	if output != "" {
		for _, info := range infos {
			encoded, err := utils.Encode(output, info)
			if err != nil {
				return err
			}
			fmt.Println(encoded)
		}
		return nil
	}
	if detail && len(infos) == 1 {
		displayDetail(infos[0])
		return nil
	}
	return displayList(infos)
}

func displayDetail(info Info) {
	fmt.Printf("Name\t\t: %s\n", info.Name)
	fmt.Printf("Subject\t\t: %s\n", info.Subject)
	fmt.Printf("Issuer\t\t: %s\n", info.Issuer)
	fmt.Printf("Serial Number\t: %s\n", info.SerialNumber)
	fmt.Printf("Not Before\t: %s\n", info.NotBefore)
	fmt.Printf("Not After\t: %s\n", info.NotAfter)
	fmt.Printf("Is CA\t\t: %t\n", info.IsCA)
	fmt.Printf("SANs\t\t: %s\n", formatSANs(&info))
	if info.PublicKeySize > 0 {
		fmt.Printf("Public Key\t: %s (%d bits)\n", info.PublicKeyAlgorithm, info.PublicKeySize)
	} else {
		fmt.Printf("Public Key\t: %s\n", info.PublicKeyAlgorithm)
	}
	fmt.Printf("Signature\t: %s\n", info.SignatureAlgorithm)
	if info.Status != "" {
		fmt.Printf("Status\t\t: %s\n", info.Status)
	}
	if info.CrExpiration != "" {
		fmt.Printf("CR Expiration\t: %s\n", info.CrExpiration)
	}
}

func displayList(infos []Info) error {
	if len(infos) == 0 {
		fmt.Println("No certificates found")
		return nil
	}
	writer := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	fmt.Fprintln(writer, "NAME\tSUBJECT\tISSUER\tNOT AFTER\tSANs")
	for _, info := range infos {
		sans := formatSANs(&info)
		if len(sans) > 60 {
			sans = sans[:57] + "..."
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", info.Name, info.Subject, info.Issuer, info.NotAfter, sans)
	}
	return writer.Flush()
}
