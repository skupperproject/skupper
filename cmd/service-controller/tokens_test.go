package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"gotest.tools/assert"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/gorilla/mux"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
)

func TestGetTokens(t *testing.T) {
	event.StartDefaultEventStore(nil)
	testname := "get-tokens-test"
	cli := &client.VanClient{
		Namespace:  testname,
		KubeClient: fake.NewSimpleClientset(),
	}
	manager := newTokenManager(cli)

	err := createClaimRecord(cli, "a", []byte("abcedefg"), nil, 1)
	assert.Check(t, err, testname)
	err = createClaimRecord(cli, "b", []byte("hijklmno"), nil, 1)
	assert.Check(t, err, testname)

	tokens, err := manager.getTokens()
	assert.Check(t, err, testname)
	expectedTokens := map[string]TokenState{
		"a": TokenState{
			Name: "a",
		},
		"b": TokenState{
			Name: "b",
		},
	}
	assert.Equal(t, len(tokens), len(expectedTokens), testname)
	for _, token := range tokens {
		assert.Equal(t, token.Name, expectedTokens[token.Name].Name, testname)
		specific, err := manager.getToken(token.Name)
		assert.Check(t, err, testname)
		assert.Equal(t, specific.Name, expectedTokens[token.Name].Name, testname)
	}
	token, err := manager.getToken("idonotexist")
	assert.Check(t, err, testname)
	assert.Assert(t, token == nil, testname)
}

func skupperInitWithController(cli *client.VanClient, name string) error {
	ctx := context.Background()
	config, err := cli.SiteConfigCreate(ctx, types.SiteConfigSpec{SkupperName: name, Ingress: "none", EnableController: true})
	if err != nil {
		return err
	}
	return cli.RouterCreate(ctx, *config)
}

func TestCreateDeleteTokens(t *testing.T) {
	event.StartDefaultEventStore(nil)
	testname := "create-tokens-test"
	cli := &client.VanClient{
		Namespace:  testname,
		KubeClient: fake.NewSimpleClientset(),
	}
	manager := newTokenManager(cli)
	err := skupperInitWithController(cli, testname)
	assert.Check(t, err, testname)
	options := TokenOptions{
		Uses: 2,
	}
	_, err = manager.generateToken(&options)
	assert.Check(t, err, testname)
	tokens, err := manager.getTokens()
	assert.Check(t, err, testname)
	assert.Equal(t, len(tokens), 1, testname)
	assert.Equal(t, *tokens[0].ClaimsRemaining, 2, testname)
	ok, err := manager.deleteToken(tokens[0].Name)
	assert.Check(t, err, testname)
	assert.Assert(t, ok, testname)
	tokens, err = manager.getTokens()
	assert.Check(t, err, testname)
	assert.Equal(t, len(tokens), 0, testname)
}

func TestServeTokens(t *testing.T) {
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
			path:         "/tokens",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodGet,
			path:         "/tokens/foo",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodGet,
			path:         "/tokens/bar",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/tokens/deleteme",
			expectedCode: http.StatusOK,
		},
		{
			method:       http.MethodDelete,
			path:         "/tokens/idontexist",
			expectedCode: http.StatusNotFound,
		},
		{
			method:       http.MethodDelete,
			path:         "/tokens",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPut,
			path:         "/tokens",
			expectedCode: http.StatusMethodNotAllowed,
		},
		{
			method:       http.MethodPost,
			path:         "/tokens",
			expectedCode: http.StatusOK,
		},
	}
	testname := "serve-tokens-test"
	cli := &client.VanClient{
		Namespace:  testname,
		KubeClient: fake.NewSimpleClientset(),
	}
	err := skupperInitWithController(cli, testname)
	assert.Check(t, err, testname)
	err = createClaimRecord(cli, "foo", []byte("abcedefg"), nil, 1)
	assert.Check(t, err, testname)
	err = createClaimRecord(cli, "deleteme", []byte("hijklmno"), nil, 1)
	assert.Check(t, err, testname)
	mgr := newTokenManager(cli)
	router := mux.NewRouter()
	handler := serveTokens(mgr)
	router.Handle("/tokens", handler)
	router.Handle("/tokens/", handler)
	router.Handle("/tokens/{name}", handler)
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
