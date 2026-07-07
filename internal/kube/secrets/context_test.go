package secrets

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestIsTlsCredentialSecret(t *testing.T) {
	tlsData := map[string][]byte{
		"ca.crt":  []byte("ca"),
		"tls.crt": []byte("cert"),
		"tls.key": []byte("key"),
	}

	tests := []struct {
		name   string
		secret *corev1.Secret
		want   bool
	}{
		{
			name: "kubernetes.io/tls",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeTLS,
				Data: tlsData,
			},
			want: true,
		},
		{
			name: "opaque with tls keys",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeOpaque,
				Data: tlsData,
			},
			want: true,
		},
		{
			name: "empty type with tls keys",
			secret: &corev1.Secret{
				Data: tlsData,
			},
			want: true,
		},
		{
			name: "opaque missing ca.crt",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"tls.crt": []byte("cert"),
					"tls.key": []byte("key"),
				},
			},
			want: false,
		},
		{
			name: "basic-auth secret",
			secret: &corev1.Secret{
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					"username": []byte("user"),
					"password": []byte("pass"),
				},
			},
			want: false,
		},
		{
			name:   "nil secret",
			secret: nil,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTlsCredentialSecret(tt.secret); got != tt.want {
				t.Fatalf("IsTlsCredentialSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}
