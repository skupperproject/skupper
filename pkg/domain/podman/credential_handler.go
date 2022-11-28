package podman

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client/container"
	"github.com/skupperproject/skupper/client/generated/libpod/client/volumes"
	"github.com/skupperproject/skupper/client/podman"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/kube"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodmanCredentialHandler struct {
	cli *podman.PodmanRestClient
}

func (p *PodmanCredentialHandler) ListCertAuthorities() ([]types.CertAuthority, error) {
	list, err := p.cli.VolumeList()
	if err != nil {
		return nil, fmt.Errorf("error retrieving certificate authorities - %w", err)
	}
	cas := []types.CertAuthority{}
	for _, v := range list {
		if v.Labels == nil {
			continue
		}
		if kind, ok := v.Labels[types.SkupperTypeQualifier]; ok && kind == "CertAuthority" {
			cas = append(cas, types.CertAuthority{Name: v.Name})
		}
	}
	return cas, nil
}

func (p *PodmanCredentialHandler) ListCredentials() ([]types.Credential, error) {
	list, err := p.cli.VolumeList()
	if err != nil {
		return nil, fmt.Errorf("error retrieving certificate authorities - %w", err)
	}
	creds := []types.Credential{}
	for _, v := range list {
		if v.Labels == nil {
			continue
		}
		// ignoring volumes that are not credentials
		if kind, ok := v.Labels[types.SkupperTypeQualifier]; !ok || kind != "Credential" {
			continue
		}
		files, err := v.ListFiles()
		if err != nil {
			return nil, fmt.Errorf("error listing credential content - %w", err)
		}
		cred := types.Credential{
			Name: v.Name,
		}

		// Validating if CA is provided
		empty := false
		for _, file := range files {
			// CA defined
			if file.Name() == types.ClaimCaCertDataKey {
				var ca *types.CertAuthority
				content, err := v.ReadFile(file.Name())
				if err != nil {
					return nil, fmt.Errorf("error validating cert authority - %w", err)
				}
				ca = p.getCertAuthorityForCaCrt(content)
				if ca != nil {
					cred.CA = ca.Name
				}
			} else if file.Name() == "connect.json" {
				cred.ConnectJson = true
			} else if file.Name() == "tls.crt" {
				dataStr, err := v.ReadFile(file.Name())
				if dataStr == "" {
					empty = true
					continue
				}
				if err != nil {
					return nil, fmt.Errorf("error reading tls.crt file from volume %s - %w", v.Name, err)
				}
				cn, hostnames, err := getTlsCrtHostnames([]byte(dataStr))
				if err != nil {
					return nil, fmt.Errorf("unable to retrieve subject and hostnames from tls.crt under %s - %w", v.Name, err)
				}
				cred.Subject = cn
				if len(hostnames) > 0 {
					cred.Hosts = hostnames
				} else {
					cred.Simple = true
				}
			}
		}

		if !empty {
			creds = append(creds, cred)
		}
	}
	return creds, nil
}

func NewPodmanCredentialHandler(cli *podman.PodmanRestClient) *PodmanCredentialHandler {
	return &PodmanCredentialHandler{
		cli: cli,
	}
}

func (p *PodmanCredentialHandler) LoadVolumeAsSecret(vol *container.Volume) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vol.Name,
			Namespace: Username,
		},
		Data: map[string][]byte{},
		Type: "kubernetes.io/tls",
	}

	if metadataLabel, ok := vol.Labels[types.InternalMetadataQualifier]; ok {
		err := json.Unmarshal([]byte(metadataLabel), &secret.ObjectMeta)
		if err != nil {
			return nil, fmt.Errorf("error loading secret metadata from volume - %v", err)
		}
	}

	files, err := vol.ListFiles()
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		data, err := ioutil.ReadFile(path.Join(vol.Source, file.Name()))
		if err != nil {
			return nil, fmt.Errorf("error reading file %s for secret %s - %v", file.Name(), vol.Name, err)
		}
		secret.Data[file.Name()] = data
	}

	return secret, nil
}

func (p *PodmanCredentialHandler) SaveSecretAsVolume(secret *corev1.Secret, kind string) (*container.Volume, error) {
	vol, err := p.cli.VolumeInspect(secret.Name)

	if err != nil {
		if _, notFound := err.(*volumes.VolumeInspectLibpodNotFound); !notFound {
			return nil, err
		}
		// creating new volume
		metadataStr, err := json.Marshal(secret.ObjectMeta)
		if err != nil {
			return nil, fmt.Errorf("error marshalling secret info for %s - %v", secret.Name, err)
		}
		vol = &container.Volume{
			Name: secret.Name,
			Labels: map[string]string{
				types.InternalMetadataQualifier: string(metadataStr),
				types.SkupperTypeQualifier:      kind,
			},
		}
		vol, err = p.cli.VolumeCreate(vol)
		if err != nil {
			return nil, fmt.Errorf("error creating volume %s - %v", secret.Name, err)
		}
	}
	_, err = vol.CreateDataFiles(secret.Data, true)
	return nil, err
}

func (p *PodmanCredentialHandler) NewCertAuthority(ca types.CertAuthority) (*corev1.Secret, error) {
	_, err := p.GetSecret(ca.Name)
	if err != nil {
		if _, notFound := err.(*volumes.VolumeInspectLibpodNotFound); !notFound {
			return nil, fmt.Errorf("Failed to check CA %s : %w", ca.Name, err)
		}
	}
	newCA := certs.GenerateCASecret(ca.Name, ca.Name)
	_, err = p.SaveSecretAsVolume(&newCA, "CertAuthority")
	return &newCA, err
}

func (p *PodmanCredentialHandler) DeleteCertAuthority(id string) error {
	return p.removeVolume(id)
}

func (p *PodmanCredentialHandler) removeVolume(id string) error {
	_, err := p.cli.VolumeInspect(id)
	if err != nil {
		if _, notFound := err.(*volumes.VolumeInspectLibpodNotFound); !notFound {
			return fmt.Errorf("Failed to check volume %s : %w", id, err)
		}
	}
	err = p.cli.VolumeRemove(id)
	if err != nil {
		return fmt.Errorf("error deleting volume %s - %v", id, err)
	}
	return nil
}

func (p *PodmanCredentialHandler) NewCredential(cred types.Credential) (*corev1.Secret, error) {
	var caSecret *corev1.Secret
	var err error
	if cred.CA != "" {
		caSecret, err = p.GetSecret(cred.CA)
		if err != nil {
			return nil, fmt.Errorf("error loading CA secret %s - %v", cred.CA, err)
		}
	}
	secret := kube.PrepareNewSecret(cred, caSecret, types.TransportDeploymentName)
	_, err = p.SaveSecretAsVolume(&secret, "Credential")
	return &secret, err
}

func (p *PodmanCredentialHandler) GetSecret(name string) (*corev1.Secret, error) {
	vol, err := p.cli.VolumeInspect(name)
	if err != nil {
		return nil, err
	}
	return p.LoadVolumeAsSecret(vol)
}

func (p *PodmanCredentialHandler) DeleteCredential(id string) error {
	return p.removeVolume(id)
}

func (p *PodmanCredentialHandler) getCertAuthorityForCaCrt(caCrtContent string) *types.CertAuthority {
	cas, err := p.ListCertAuthorities()
	if err != nil {
		return nil
	}
	for _, ca := range cas {
		v, err := p.cli.VolumeInspect(ca.Name)
		if err != nil {
			return nil
		}
		content, _ := v.ReadFile("tls.crt")
		if caCrtContent == content {
			return &ca
		}
	}
	return nil
}

func getTlsCrtHostnames(tlscrtData []byte) (subject string, hostnames []string, err error) {
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

func (p *PodmanCredentialHandler) GetCredential(id string) (*types.Credential, error) {
	credentials, err := p.ListCredentials()
	if err != nil {
		return nil, err
	}
	for _, cred := range credentials {
		if cred.Name == id {
			return &cred, nil
		}
	}
	return nil, fmt.Errorf("credential not found")
}
