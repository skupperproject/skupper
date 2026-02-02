package grants

import (
	"log/slog"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func Test_tlsCredentialsUpdated(t *testing.T) {
	secret, err := tf.secret("simple", "test", "My Subject", []string{"foo.com", "bar.org"})
	if err != nil {
		t.Error(err)
	}

	var tests = []struct {
		name   string
		key    string
		secret *corev1.Secret
	}{
		{
			name:   "simple",
			key:    "test/simple",
			secret: secret,
		},
		{
			name:   "non tls secret",
			key:    "test/simple",
			secret: tf.genericSecret("simple", "test"),
		},
		{
			name: "nil secret",
			key:  "test/simple",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := &GrantsEnabled{
				grants: newGrants(nil, nil, "https", ""),
				logger: slog.Default(),
			}
			gc.server = newServer(":0", true, gc.grants)
			err := gc.tlsCredentialsUpdated(tt.key, tt.secret)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
