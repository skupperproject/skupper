package grants

import (
	"bytes"
	"io"
	"testing"

	"gotest.tools/v3/assert"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func Test_CertTokenWrite(t *testing.T) {
	var tests = []struct {
		name          string
		cert          *CertToken
		failWrite     int
		expectedError string
	}{
		{
			name: "good",
			cert: &CertToken{
				tlsCredentials: tf.secret("my-token", "", "My Subject", nil),
				links: []*v2alpha1.Link{
					tf.link("my-token", "", []v2alpha1.Endpoint{
						{
							Host: "foo",
							Port: "1234",
						},
					}, "my-token"),
				},
			},
		},
		{
			name: "bad",
			cert: &CertToken{
				tlsCredentials: tf.secret("my-token", "", "My Subject", nil),
				links: []*v2alpha1.Link{
					tf.link("my-token", "", []v2alpha1.Endpoint{
						{
							Host: "foo",
							Port: "1234",
						},
					}, "my-token"),
				},
			},
			failWrite:     1,
			expectedError: "Failed Write",
		},
		{
			name: "later bad",
			cert: &CertToken{
				tlsCredentials: tf.secret("my-token", "", "My Subject", nil),
				links: []*v2alpha1.Link{
					tf.link("my-token", "", []v2alpha1.Endpoint{
						{
							Host: "foo",
							Port: "1234",
						},
					}, "my-token"),
				},
			},
			failWrite:     2,
			expectedError: "Failed Write",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buffer bytes.Buffer
			var writer io.Writer
			writer = &buffer
			if tt.failWrite != 0 {
				writer = &FailingWriter{
					writer: writer,
					fail:   tt.failWrite,
				}
			}

			err := tt.cert.Write(writer)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {

			}
		})
	}
}
