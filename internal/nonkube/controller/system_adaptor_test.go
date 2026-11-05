package controller

import (
	"testing"

	"github.com/skupperproject/skupper/internal/qdr"
	"gotest.tools/v3/assert"
)

func TestAddContainerPathToProxyProfiles(t *testing.T) {
	adaptor := &SystemAdaptor{}

	tests := []struct {
		name             string
		inputPassword    string
		expectedPassword string
	}{
		{
			name:             "authenticated proxy with placeholder path",
			inputPassword:    "file:${SSL_PROFILE_BASE_PATH}/runtime/proxies/my-proxy/password.txt",
			expectedPassword: "file:/etc/skupper-router/runtime/proxies/my-proxy/password.txt",
		},
		{
			name:             "unauthenticated proxy",
			inputPassword:    "",
			expectedPassword: "",
		},
		{
			name:             "already resolved path",
			inputPassword:    "file:/etc/skupper-router/runtime/proxies/my-proxy/password.txt",
			expectedPassword: "file:/etc/skupper-router/runtime/proxies/my-proxy/password.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := map[string]qdr.ProxyProfile{
				"my-proxy": {
					Name:     "my-proxy",
					Host:     "proxy.example.com",
					Port:     "3128",
					Username: "proxyuser",
					Password: tt.inputPassword,
				},
			}
			result := adaptor.addContainerPathToProxyProfiles(input)
			assert.Equal(t, result["my-proxy"].Password, tt.expectedPassword)
			assert.Equal(t, result["my-proxy"].Host, "proxy.example.com")
			assert.Equal(t, result["my-proxy"].Port, "3128")
		})
	}
}
