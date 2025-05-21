package grants

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	meta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/client/fake"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func Test_postTokenRequest(t *testing.T) {
	var tests = []struct {
		name          string
		token         *v2alpha1.AccessToken
		site          *v2alpha1.Site
		code          int
		body          string
		err           string
		expectedError string
	}{
		{
			name:  "simple",
			token: tf.token("my-token", "x", "http://foo/xyz", "mycode", ""),
			site:  tf.site("my-site", "x"),
			code:  200,
			body:  "OK",
		},
		{
			name:          "not found",
			token:         tf.token("my-token", "x", "http://foo/xyz", "mycode", ""),
			site:          tf.site("my-site", "x"),
			code:          404,
			body:          "no such grant",
			expectedError: "404 (Not Found) no such grant",
		},
		{
			name:          "bad url",
			token:         tf.token("my-token", "x", "foo://a:b", "mycode", ""),
			site:          tf.site("my-site", "x"),
			expectedError: "foo://a:b",
		},
		{
			name:          "bad url",
			token:         tf.token("my-token", "x", "http://foo", "mycode", ""),
			site:          tf.site("my-site", "x"),
			err:           "some kind of failure",
			expectedError: "some kind of failure",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := postTokenRequest(tt.token, tt.site, tripper(tt.code, tt.body, tt.err))
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				body, _ := io.ReadAll(reader)
				assert.Equal(t, string(body), tt.body)
			}
		})
	}
}

type TestTripper struct {
	code int
	body string
	err  string
}

func tripper(code int, body string, err string) http.RoundTripper {
	return &TestTripper{
		code: code,
		body: body,
		err:  err,
	}
}

func (t *TestTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	if t.err != "" {
		return nil, errors.New(t.err)
	}
	return &http.Response{
		StatusCode: t.code,
		Body:       ioutil.NopCloser(bytes.NewBufferString(t.body)),
		Header:     make(http.Header),
	}, nil
}

func Test_handleTokenResponse(t *testing.T) {
	var tests = []struct {
		name                 string
		token                *v2alpha1.AccessToken
		site                 *v2alpha1.Site
		body                 *CertToken
		failReadAt           int
		expectedStatus       string
		expectStatusContains string
		expectedError        string
		expectedLinks        []string
		expectedCosts        []int
		expectedSecret       string
		extraK8sObjects      []runtime.Object
		extraSkupperObjects  []runtime.Object
	}{
		{
			name:  "simple",
			token: tf.token("my-token", "test", "http://foo/xyz", "mycode", ""),
			site:  tf.site("my-site", "test"),
			body: &CertToken{
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
			expectedStatus: "OK",
			expectedLinks:  []string{"my-token"},
			expectedSecret: "my-token",
		},
		{
			name:           "no data",
			token:          tf.token("my-token", "test", "http://foo/xyz", "mycode", ""),
			site:           tf.site("my-site", "test"),
			expectedStatus: "Controller could not decode response",
		},
		{
			name:  "failed read",
			token: tf.token("my-token", "test", "http://foo/xyz", "mycode", ""),
			site:  tf.site("my-site", "test"),
			body: &CertToken{
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
			failReadAt:     1025,
			expectedStatus: "Controller could not decode response",
		},
		{
			name:  "secret collision",
			token: tf.token("my-token", "test", "http://foo/xyz", "mycode", ""),
			site:  tf.site("my-site", "test"),
			body: &CertToken{
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
			expectedStatus:  "Controller could not create received secret: secrets \"my-token\" already exists",
			extraK8sObjects: []runtime.Object{tf.secret("my-token", "test", "Another Subject", nil)},
		},
		{
			name:  "link collision",
			token: tf.token("my-token", "test", "http://foo/xyz", "mycode", ""),
			site:  tf.site("my-site", "test"),
			body: &CertToken{
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
			expectedStatus:      "Controller could not create received link: links.skupper.io \"my-token\" already exists",
			extraSkupperObjects: []runtime.Object{tf.link("my-token", "test", nil, "")},
		},
		{
			name:  "link cost",
			token: tf.addLinkCost(tf.token("my-token", "test", "http://foo/xyz", "mycode", ""), 10),
			site:  tf.site("my-site", "test"),
			body: &CertToken{
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
			expectedStatus: "OK",
			expectedLinks:  []string{"my-token"},
			expectedCosts:  []int{10},
			expectedSecret: "my-token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skupperObjects := []runtime.Object{tt.site, tt.token}
			if tt.extraSkupperObjects != nil {
				skupperObjects = append(skupperObjects, tt.extraSkupperObjects...)
			}
			client, _ := fake.NewFakeClient("test", tt.extraK8sObjects, skupperObjects, "")
			var buffer bytes.Buffer
			if tt.body != nil {
				tt.body.Write(&buffer)
			}
			var reader io.Reader
			reader = &buffer
			if tt.failReadAt != 0 {
				reader = &FailingReader{
					reader: reader,
					fail:   tt.failReadAt,
				}
			}
			err := handleTokenResponse(reader, tt.token, tt.site, client)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				token, err := client.GetSkupperClient().SkupperV2alpha1().AccessTokens("test").Get(context.TODO(), tt.token.Name, metav1.GetOptions{})
				if err != nil {
					t.Error(err)
				} else {
					assert.Equal(t, token.Status.Message, tt.expectedStatus)
					if tt.expectedSecret != "" {
						secret, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.TODO(), tt.expectedSecret, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						assert.Assert(t, secret.Data["tls.crt"] != nil)
					}
					for i, name := range tt.expectedLinks {
						link, err := client.GetSkupperClient().SkupperV2alpha1().Links("test").Get(context.TODO(), name, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						} else {
							assert.Assert(t, len(link.Spec.Endpoints) > 0)
							if len(tt.expectedCosts) > i {
								assert.Equal(t, link.Spec.Cost, tt.expectedCosts[i])
							}
						}
					}
				}
			}
		})
	}
}

func Test_RedeemAccessToken(t *testing.T) {
	var tests = []struct {
		name           string
		scheme         string
		tokenName      string
		grantUID       string
		defaultIssuer  string
		endpoints      []v2alpha1.Endpoint
		expectedError  string
		expectedStatus string
		expectedLinks  []string
		expectedSecret string
		expectRedeemed bool
	}{
		{
			name:      "simple",
			scheme:    "https",
			tokenName: "my-token",
			endpoints: []v2alpha1.Endpoint{
				{
					Name: "inter-router",
					Host: "my-link-host",
					Port: "1111",
				},
				{
					Name: "edge",
					Host: "my-link-host",
					Port: "2222",
				},
			},
			expectedStatus: "OK",
			expectRedeemed: true,
			expectedLinks:  []string{"my-token"},
			expectedSecret: "my-token",
		},
		{
			name:      "ha",
			scheme:    "https",
			tokenName: "my-token",
			endpoints: []v2alpha1.Endpoint{
				{
					Name:  "inter-router",
					Host:  "my-link-host-1",
					Port:  "1111",
					Group: "one",
				},
				{
					Name:  "edge",
					Host:  "my-link-host-1",
					Port:  "2222",
					Group: "one",
				},
				{
					Name:  "inter-router",
					Host:  "my-link-host-2",
					Port:  "1111",
					Group: "two",
				},
				{
					Name:  "edge",
					Host:  "my-link-host-2",
					Port:  "2222",
					Group: "two",
				},
			},
			expectedStatus: "OK",
			expectRedeemed: true,
			expectedLinks:  []string{"my-token-1", "my-token-2"},
			expectedSecret: "my-token",
		},
		{
			name:      "tls disabled",
			scheme:    "http",
			tokenName: "my-token",
			endpoints: []v2alpha1.Endpoint{
				{
					Name: "inter-router",
					Host: "my-link-host",
					Port: "1111",
				},
				{
					Name: "edge",
					Host: "my-link-host",
					Port: "2222",
				},
			},
			expectedStatus: "OK",
			expectRedeemed: true,
			expectedLinks:  []string{"my-token"},
			expectedSecret: "my-token",
		},
		{
			name:           "not known",
			scheme:         "https",
			tokenName:      "my-token",
			grantUID:       "a40fbe84-f276-4755-bf22-5ba980ab1661",
			expectedStatus: "Controller got failed response: 404 (Not Found) No such claim",
			expectRedeemed: false,
		},
		{
			name:           "no resolved endpoints",
			scheme:         "https",
			tokenName:      "my-token",
			expectedStatus: "Controller got failed response: 500 (Internal Server Error) Could not resolve any endpoints for requested link",
		},
		{
			name:           "bad default issuer",
			scheme:         "https",
			tokenName:      "my-token",
			defaultIssuer:  "i-dont-exist",
			expectedStatus: "Controller got failed response: 500 (Internal Server Error) Could not get issuer for requested certificate",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := fake.NewFakeClient("test", []runtime.Object{tf.secret("skupper-site-ca", "test", "Test Site CA", nil)}, []runtime.Object{tf.site("my-site", "test"), tf.grant("my-grant", "test", "")}, "")
			site, err := client.GetSkupperClient().SkupperV2alpha1().Sites("test").Get(context.TODO(), "my-site", metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			if tt.defaultIssuer != "" {
				site.Status.DefaultIssuer = tt.defaultIssuer
			} else {
				site.Status.DefaultIssuer = "skupper-site-ca"
			}
			site.Status.Endpoints = tt.endpoints
			site, err = client.GetSkupperClient().SkupperV2alpha1().Sites("test").UpdateStatus(context.TODO(), site, metav1.UpdateOptions{})
			if err != nil {
				t.Error(err)
			}

			grants := newGrants(client, generator(site, client), tt.scheme, "")
			server := newServer(":0", tt.scheme == "https", grants)
			server.listen()
			grants.setUrl(fmt.Sprintf("localhost:%d", server.port()))
			go server.serve()
			defer server.stop()
			if tt.scheme == "https" {
				secret := tf.secret("my-creds", "test", "grant server", []string{"localhost"})
				err = server.setCertificateFromSecret(secret)
				if err != nil {
					t.Error(err)
				}
				if grants.setCA(string(secret.Data["ca.crt"])) {
					grants.recheckCa()
				}
			}
			grant, err := client.GetSkupperClient().SkupperV2alpha1().AccessGrants("test").Get(context.TODO(), "my-grant", metav1.GetOptions{})
			if err != nil {
				t.Error(err)
			}
			err = grants.checkGrant(grant.Namespace+"/"+grant.Name, grant)
			if err != nil {
				t.Error(err)
			}

			token := tf.token(tt.tokenName, "test", grant.Status.Url, grant.Status.Code, grant.Status.Ca)
			if tt.grantUID != "" {
				token.Spec.Url = fmt.Sprintf("%s://localhost:%d/%s", tt.scheme, server.port(), tt.grantUID)
			}
			token, err = client.GetSkupperClient().SkupperV2alpha1().AccessTokens("test").Create(context.TODO(), token, metav1.CreateOptions{})
			if err != nil {
				t.Error(err)
			}
			err = RedeemAccessToken(token, site, client)
			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else if err != nil {
				t.Error(err)
			} else {
				// verify the access token status is redeemed
				token, err = client.GetSkupperClient().SkupperV2alpha1().AccessTokens("test").Get(context.TODO(), tt.tokenName, metav1.GetOptions{})
				if err != nil {
					t.Error(err)
				} else {
					assert.Equal(t, token.Status.Message, tt.expectedStatus)
					if tt.expectRedeemed {
						assert.Assert(t, meta.IsStatusConditionTrue(token.Status.Conditions, v2alpha1.CONDITION_TYPE_REDEEMED))
						// verify we have link(s) and secret as expected
						for _, name := range tt.expectedLinks {
							link, err := client.GetSkupperClient().SkupperV2alpha1().Links("test").Get(context.TODO(), name, metav1.GetOptions{})
							if err != nil {
								t.Error(err)
							} else {
								assert.Assert(t, len(link.Spec.Endpoints) > 0)
							}
						}
						secret, err := client.GetKubeClient().CoreV1().Secrets("test").Get(context.TODO(), tt.expectedSecret, metav1.GetOptions{})
						if err != nil {
							t.Error(err)
						}
						assert.Assert(t, secret.Data["tls.crt"] != nil)
					}
				}
			}
		})
	}
}

type TestTokenGenerator struct {
	site    *v2alpha1.Site
	clients internalclient.Clients
}

func (g *TestTokenGenerator) generate(namespace string, name string, subject string, writer io.Writer) error {
	generator, err := NewTokenGenerator(g.site, g.clients)
	if err != nil {
		return err
	}
	token := generator.NewCertToken(name, subject)
	return token.Write(writer)
}

func newTestTokenGenerator(site *v2alpha1.Site, clients internalclient.Clients) *TestTokenGenerator {
	return &TestTokenGenerator{
		site:    site,
		clients: clients,
	}
}

func generator(site *v2alpha1.Site, clients internalclient.Clients) GrantResponse {
	return newTestTokenGenerator(site, clients).generate
}

func Test_updateAccessTokenStatus(t *testing.T) {
	var tests = []struct {
		name           string
		errs           []error
		expectedStatus string
	}{
		{
			name:           "simple",
			errs:           []error{nil},
			expectedStatus: "OK",
		},
		{
			name:           "failure",
			errs:           []error{errors.New("something bad happened")},
			expectedStatus: "something bad happened",
		},
		{
			name:           "repeated failure",
			errs:           []error{errors.New("something bad happened"), errors.New("it happened again"), errors.New("it happened again")},
			expectedStatus: "it happened again",
		},
		{
			name:           "recovered sequence",
			errs:           []error{errors.New("something bad happened"), nil, nil},
			expectedStatus: "OK",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := fake.NewFakeClient("test", nil, []runtime.Object{tf.token("my-token", "test", "", "", "")}, "")
			for _, err := range tt.errs {
				token, apiError := client.GetSkupperClient().SkupperV2alpha1().AccessTokens("test").Get(context.TODO(), "my-token", metav1.GetOptions{})
				if apiError != nil {
					t.Error(apiError)
				} else {
					updateAccessTokenStatus(token, err, client)
				}
			}
			token, apiError := client.GetSkupperClient().SkupperV2alpha1().AccessTokens("test").Get(context.TODO(), "my-token", metav1.GetOptions{})
			if apiError != nil {
				t.Error(apiError)
			} else {
				assert.Equal(t, token.Status.Message, tt.expectedStatus)
			}
		})
	}
}
