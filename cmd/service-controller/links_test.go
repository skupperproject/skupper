package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type MockConnectorManager struct {
	connectors map[string]qdr.ConnectorStatus
	err        error
}

func (m *MockConnectorManager) getConnectorStatus() (map[string]qdr.ConnectorStatus, error) {
	return m.connectors, m.err
}

func getTestToken(name string, tokentype string, annotations map[string]string) *corev1.Secret {
	token := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: tokentype,
			},
			Annotations: map[string]string{},
		},
		Data: map[string][]byte{},
	}
	if annotations != nil {
		token.ObjectMeta.Annotations = annotations
	} else if tokentype == types.TypeClaimRequest {
		token.ObjectMeta.Annotations[types.ClaimUrlAnnotationKey] = "http://myserver:1234/foo"
		token.Data[types.ClaimPasswordDataKey] = []byte("abcdefgh")
	} else if tokentype == types.TypeToken {

	}
	return &token
}

func getEncodedTestToken(name string, tokentype string, annotations map[string]string) *bytes.Buffer {
	token := getTestToken(name, tokentype, annotations)
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	var encoded bytes.Buffer
	s.Encode(token, &encoded)
	return &encoded
}

func decodeToken(data []byte) (*corev1.Secret, error) {
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme, json.SerializerOptions{Yaml: true})
	var token corev1.Secret
	_, _, err := s.Decode(data, nil, &token)
	return &token, err
}

func createTestToken(cli *client.VanClient, name string, tokentype string, annotations map[string]string) error {
	token := getTestToken(name, tokentype, annotations)
	_, err := cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(token)
	return err
}

func TestGetLinks(t *testing.T) {
	event.StartDefaultEventStore(nil)
	testname := "get-links-test"
	cli := &client.VanClient{
		Namespace:  testname,
		KubeClient: fake.NewSimpleClientset(),
	}
	connectors := &MockConnectorManager{}
	manager := &LinkManager{
		cli:        cli,
		connectors: connectors,
	}

	err := createTestToken(cli, "a", types.TypeClaimRequest, nil)
	assert.Check(t, err, testname)
	err = createTestToken(cli, "b", types.TypeToken, nil)
	assert.Check(t, err, testname)

	links, err := manager.getLinks()
	assert.Check(t, err, testname)
	expectedLinks := map[string]types.LinkStatus{
		"a": types.LinkStatus{
			Name:        "a",
			Url:         "",
			Cost:        0,
			Connected:   false,
			Configured:  false,
			Description: "",
		},
		"b": types.LinkStatus{
			Name:        "b",
			Url:         "",
			Cost:        1,
			Connected:   false,
			Configured:  true,
			Description: "",
		},
	}
	assert.Equal(t, len(links), len(expectedLinks), testname)
	for _, link := range links {
		assert.Equal(t, link.Name, expectedLinks[link.Name].Name, testname)
		assert.Equal(t, link.Connected, expectedLinks[link.Name].Connected, testname)
		assert.Equal(t, link.Configured, expectedLinks[link.Name].Configured, testname)
		specific, err := manager.getLink(link.Name)
		assert.Check(t, err, testname)
		assert.Equal(t, specific.Name, expectedLinks[link.Name].Name, testname)
		assert.Equal(t, specific.Connected, expectedLinks[link.Name].Connected, testname)
		assert.Equal(t, specific.Configured, expectedLinks[link.Name].Configured, testname)
	}
	link, err := manager.getLink("idonotexist")
	assert.Check(t, err, testname)
	assert.Assert(t, link == nil, testname)
}

func TestCreateDeleteLinks(t *testing.T) {
	event.StartDefaultEventStore(nil)
	testname := "create-links-test"
	cli := &client.VanClient{
		Namespace:  testname,
		KubeClient: fake.NewSimpleClientset(),
	}
	connectors := &MockConnectorManager{}
	manager := &LinkManager{
		cli:        cli,
		connectors: connectors,
	}
	err := skupperInit(cli, testname)
	assert.Check(t, err, testname)
	err = manager.createLink(5, getEncodedTestToken("mytoken", types.TypeClaimRequest, nil).Bytes())
	assert.Check(t, err, testname)
	links, err := manager.getLinks()
	assert.Check(t, err, testname)
	assert.Equal(t, len(links), 1, testname)
	assert.Equal(t, links[0].Connected, false, testname)
	assert.Equal(t, links[0].Configured, false, testname)
	assert.Equal(t, links[0].Cost, 5, testname)
	ok, err := manager.deleteLink(links[0].Name)
	assert.Check(t, err, testname)
	assert.Assert(t, ok, testname)
	links, err = manager.getLinks()
	assert.Check(t, err, testname)
	assert.Equal(t, len(links), 0, testname)
}

type MockLinkManager struct {
	links      []types.LinkStatus
	connectors map[string]qdr.ConnectorStatus
	err        error
}

func (m *MockLinkManager) getLinks() ([]types.LinkStatus, error) {
	return m.links, m.err
}

func (m *MockLinkManager) getLink(name string) (*types.LinkStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, link := range m.links {
		if link.Name == name {
			return &link, nil
		}
	}
	return nil, nil
}

func (m *MockLinkManager) deleteLink(name string) (bool, error) {
	if m.err != nil {
		return false, m.err
	}
	found := false
	updated := []types.LinkStatus{}
	for _, link := range m.links {
		if link.Name != name {
			updated = append(updated, link)
		} else {
			found = true
		}
	}
	m.links = updated
	return found, nil
}

func (m *MockLinkManager) createLink(cost int, token []byte) error {
	if m.err != nil {
		return m.err
	}
	if len(token) == 0 {
		return fmt.Errorf("Need to specify token!")
	}
	secret, err := decodeToken(token)
	if err != nil {
		return err
	}
	link := getLinkStatus(secret, m.connectors)
	if link != nil {
		m.links = append(m.links, *link)
	}
	return nil
}

func (m *MockLinkManager) addLink(link types.LinkStatus) {
	m.links = append(m.links, link)
}

func TestServeLinks(t *testing.T) {
	event.StartDefaultEventStore(nil)
	var tests = []struct {
		name         string
		method       string
		path         string
		body         io.Reader
		expectedCode int
	}{
		{
			method:       http.MethodGet,
			path:         "/",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodGet,
			path:         "/links",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodGet,
			path:         "/links/foo",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodGet,
			path:         "/links/bar",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/links/deleteme",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodDelete,
			path:         "/links/idontexist",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/links",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPut,
			path:         "/links",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPost,
			path:         "/links",
			body:         getEncodedTestToken("letmebe", types.TypeToken, nil),
			expectedCode: http.StatusOK,
		},
		{
			name:         "no token supplied",
			method:       http.MethodPost,
			path:         "/links",
			expectedCode: http.StatusInternalServerError,
		},
	}
	mockManager := &MockLinkManager{}
	mockManager.addLink(types.LinkStatus{
		Name:       "foo",
		Connected:  true,
		Configured: true,
		Url:        "myhost:1234/foo",
		Cost:       7,
	})
	mockManager.addLink(types.LinkStatus{
		Name: "deleteme",
	})
	router := mux.NewRouter()
	handler := serveLinks(mockManager)
	router.Handle("/links", handler)
	router.Handle("/links/", handler)
	router.Handle("/links/{name}", handler)
	for _, test := range tests {
		name := test.name
		if name == "" {
			name = test.method + " " + test.path
		}
		req := httptest.NewRequest(test.method, test.path, test.body)
		res := httptest.NewRecorder()

		router.ServeHTTP(res, req)
		assert.Equal(t, res.Code, test.expectedCode, name)
	}
}
