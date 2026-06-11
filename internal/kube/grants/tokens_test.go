package grants

import (
	"bytes"
	"io"
	"testing"

	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_CertTokenWrite(t *testing.T) {
	mySecret, err := tf.secret("my-token", "", "My Subject", nil)
	if err != nil {
		t.Error(err)
	}
	var tests = []struct {
		name          string
		cert          *CertToken
		failWrite     int
		expectedError string
	}{
		{
			name: "good",
			cert: &CertToken{
				tlsCredentials: mySecret,
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
				tlsCredentials: mySecret,
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
				tlsCredentials: mySecret,
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

func Test_NewTokenGenerator(t *testing.T) {
	site := tf.site("my-site", "test")
	site.Status.Endpoints = []v2alpha1.Endpoint{
		{
			Name:  "inter-router",
			Host:  "my-host",
			Port:  "55671",
			Group: "default",
		},
	}
	caSecret, err := tf.secret("skupper-site-ca", "test", "site-ca", nil)
	assert.Assert(t, err == nil)
	client, err := fake.NewFakeClient("test", []runtime.Object{caSecret}, nil, "")
	assert.Assert(t, err == nil)

	t.Run("Working Site", func(t *testing.T) {
		site.Spec.Edge = false
		generator, err := NewTokenGenerator(site, client)
		assert.Assert(t, err == nil)
		assert.Assert(t, generator != nil)
	})

	t.Run("Edge Site Fail", func(t *testing.T) {
		site.Spec.Edge = true
		generator, err := NewTokenGenerator(site, client)
		assert.ErrorContains(t, err, "Edge sites cannot accept incoming links from remote sites")
		assert.Assert(t, generator == nil)
	})

	t.Run("No Endpoints Fail", func(t *testing.T) {
		site.Spec.Edge = false
		site.Status.Endpoints = nil
		generator, err := NewTokenGenerator(site, client)
		assert.ErrorContains(t, err, "No valid endpoints found for site")
		assert.Assert(t, generator == nil)
	})

	t.Run("Missing CA", func(t *testing.T) {
		emptyClient, err := fake.NewFakeClient("test", nil, nil, "")
		site.Spec.Edge = false
		site.Status.Endpoints = []v2alpha1.Endpoint{{Host: "foo", Name: "inter-router"}}
		generator, err := NewTokenGenerator(site, emptyClient)
		assert.ErrorContains(t, err, "Could not get issuer for requested certificate")
		assert.Assert(t, generator == nil)
	})
}
