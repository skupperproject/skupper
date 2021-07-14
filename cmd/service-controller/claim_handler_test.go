package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

type MockVerifier struct {
	Current int
	Results []MockVerificationResult
}

type MockVerificationResult struct {
	Password      []byte
	Certificate   *corev1.Secret
	StatusCode    int
	StatusMessage string
}

func (server *MockVerifier) addSuccessfulResult(password []byte, certificate *corev1.Secret) {
	server.Results = append(server.Results, MockVerificationResult{
		Password:    password,
		Certificate: certificate,
		StatusCode:  http.StatusOK,
	})
}

func (server *MockVerifier) addFailedResult(code int, message string) {
	server.Results = append(server.Results, MockVerificationResult{
		Certificate:   nil,
		StatusCode:    code,
		StatusMessage: message,
	})
}

func (server *MockVerifier) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if len(server.Results) > server.Current {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			if !bytes.Equal(body, server.Results[server.Current].Password) {
				http.Error(w, "password does not match", http.StatusForbidden)
			} else if server.Results[server.Current].Certificate == nil {
				http.Error(w, server.Results[server.Current].StatusMessage, server.Results[server.Current].StatusCode)
			} else {
				s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
				err := s.Encode(server.Results[server.Current].Certificate, w)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
				}
			}
		}
	} else {
		http.Error(w, "No result defined for mock verifier", http.StatusInternalServerError)
	}
}

func newTestClaim(name string, url string, password []byte) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{
				types.ClaimUrlAnnotationKey: url + "/" + name,
			},
		},
		Data: map[string][]byte{
			types.ClaimPasswordDataKey: password,
		},
	}
}

func TestClaimHandler(t *testing.T) {

	event.StartDefaultEventStore(nil)
	cli := &client.VanClient{
		Namespace:  "claim-handler-test",
		KubeClient: fake.NewSimpleClientset(),
	}

	handler := &ClaimHandler{
		name:      "ClaimHandler",
		vanClient: cli,
		siteId:    "site-a",
	}

	verifier := &MockVerifier{}
	server := httptest.NewServer(verifier)
	defer server.Close()

	name := "foo"
	password := []byte("abcdefg")
	claim := newTestClaim(name, server.URL, password)
	_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(claim)
	assert.Check(t, err, name)
	cert := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeClaimRecord,
			},
			Annotations: map[string]string{
				"foo": "bar",
				"bar": "baz",
			},
		},
		Data: map[string][]byte{
			"a": []byte("1"),
			"b": []byte("2"),
		},
	}
	verifier.addSuccessfulResult([]byte("abcdefg"), cert)
	err = handler.redeemClaim(claim)
	assert.Check(t, err, name)
	secret, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Get(name, metav1.GetOptions{})
	assert.Check(t, err, name)
	for key, value := range cert.ObjectMeta.Annotations {
		assert.Equal(t, secret.ObjectMeta.Annotations[key], value, name)
	}
	for key, value := range cert.ObjectMeta.Labels {
		assert.Equal(t, secret.ObjectMeta.Labels[key], value, name)
	}
	for key, value := range cert.Data {
		assert.Assert(t, bytes.Equal(secret.Data[key], value), name)
	}
}

func TestInvalidClaims(t *testing.T) {
	event.StartDefaultEventStore(nil)
	cli := &client.VanClient{
		Namespace:  "claim-handler-test",
		KubeClient: fake.NewSimpleClientset(),
	}

	handler := &ClaimHandler{
		name:      "ClaimHandler",
		vanClient: cli,
		siteId:    "site-a",
	}

	var tests = []struct {
		secret *corev1.Secret
		err    string
	}{
		{
			&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
					Labels: map[string]string{
						types.SkupperTypeQualifier: types.TypeClaimRecord,
					},
				},
				Data: map[string][]byte{
					types.ClaimPasswordDataKey: []byte("foo"),
				},
			},
			"no annotations",
		},
		{
			&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
					Labels: map[string]string{
						types.SkupperTypeQualifier: types.TypeClaimRecord,
					},
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
				Data: map[string][]byte{
					types.ClaimPasswordDataKey: []byte("foo"),
				},
			},
			"no url specified",
		},
		{
			&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
					Labels: map[string]string{
						types.SkupperTypeQualifier: types.TypeClaimRecord,
					},
					Annotations: map[string]string{
						types.ClaimUrlAnnotationKey: "http://foo",
					},
				},
			},
			"no data",
		},
		{
			&corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "v1",
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "a",
					Labels: map[string]string{
						types.SkupperTypeQualifier: types.TypeClaimRecord,
					},
					Annotations: map[string]string{
						types.ClaimUrlAnnotationKey: "http://foo",
					},
				},
				Data: map[string][]byte{
					"bar": []byte("foo"),
				},
			},
			"no password specified",
		},
	}
	for _, test := range tests {
		err := handler.redeemClaim(test.secret)
		assert.Check(t, err, test.secret.ObjectMeta.Name)
		assert.Equal(t, test.secret.ObjectMeta.Annotations[types.StatusAnnotationKey], test.err, test.secret.ObjectMeta.Name)
	}
}
