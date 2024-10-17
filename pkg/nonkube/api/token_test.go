package api

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/certs"
	"gotest.tools/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateTokens(t *testing.T) {
	/*
	 * No inter-router nor edge roles in RouterAccess
	 * RouterAccess with only one of the roles
	 * RouterAccess with both roles
	 * Bad user provided server certificate (no tls.crt in data)
	 * Good user provided server certificate (no SANs)
	 * Good user provided server certificate with valid SANs
	   * subjectAlternativeNames (from routeraccess.spec to be ignored)
	 * Good user provided server certificate with empty SANs
	   * subjectAlternativeNames (from routeraccess.spec to be used)
	*/

	var clientSecret = v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "link-fake-router-access",
		},
		Data: map[string][]byte{
			"tls.key": []byte("fake-client-key"),
			"tls.crt": []byte("fake-client-cert"),
			"ca.crt":  []byte("fake-client-ca"),
		},
	}

	tests := []struct {
		name          string
		ra            v1alpha1.RouterAccess
		serverSecret  v1.Secret
		expectedHosts []string
	}{
		{
			name:          "no-inter-router-or-edge-roles",
			ra:            fakeRouterAccessNoInterRouterEdgeRoles(),
			expectedHosts: nil,
		},
		{
			name: "inter-router-role-only",
			ra:   fakeRouterAccessInterRouterRole(),
			expectedHosts: []string{
				"127.0.0.1",
			},
		},
		{
			name: "both-roles",
			ra:   fakeRouterAccessBothRoles(),
			expectedHosts: []string{
				"127.0.0.1",
			},
		},
		{
			name:         "invalid-server-cert",
			ra:           fakeRouterAccessBothRoles(),
			serverSecret: fakeServerSecretBad(),
			expectedHosts: []string{
				"127.0.0.1",
			},
		},
		{
			name:         "server-cert-no-hosts",
			ra:           fakeRouterAccessBothRoles(),
			serverSecret: fakeServerSecret([]string{}),
			expectedHosts: []string{
				"127.0.0.1",
			},
		},
		{
			name: "sans-provided-no-server-cert",
			ra:   fakeRouterAccessWithSANs("fake.host.one", "fake.host.two"),
			expectedHosts: []string{
				"127.0.0.1",
				"fake.host.one",
				"fake.host.two",
			},
		},
		{
			name:         "sans-provided-empty-server-cert",
			ra:           fakeRouterAccessWithSANs("fake.host.one", "fake.host.two"),
			serverSecret: fakeServerSecret([]string{}),
			expectedHosts: []string{
				"127.0.0.1",
				"fake.host.one",
				"fake.host.two",
			},
		},
		{
			name:         "sans-provided-server-cert-with-hosts",
			ra:           fakeRouterAccessWithSANs("fake.host.one", "fake.host.two"),
			serverSecret: fakeServerSecret([]string{"server.host.one", "server.host.two"}),
			expectedHosts: []string{
				"127.0.0.1",
				"server.host.one",
				"server.host.two",
			},
		},
		{
			name:         "sans-provided-server-cert-with-hosts-and-ips",
			ra:           fakeRouterAccessWithSANs("fake.host.one", "fake.host.two"),
			serverSecret: fakeServerSecret([]string{"server.host.one", "server.host.two", "10.0.0.1", "10.0.0.2"}),
			expectedHosts: []string{
				"127.0.0.1",
				"10.0.0.1",
				"10.0.0.2",
				"server.host.one",
				"server.host.two",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tokens := CreateTokens(test.ra, test.serverSecret, clientSecret)
			// validate expected token count
			tokensJson, _ := json.MarshalIndent(tokens, "", "  ")
			assert.Equal(t, len(tokens), len(test.expectedHosts), "expected tokens for hosts: %v - got: %v", test.expectedHosts, string(tokensJson))
			// nothing else to validate
			if len(test.expectedHosts) == 0 {
				return
			}
			hostsFound := map[string]bool{}
			for _, host := range test.expectedHosts {
				hostsFound[host] = false
			}
			// validate tokens
			for _, token := range tokens {
				assert.Equal(t, token.Links[0].Name, fmt.Sprintf("link-%s", test.ra.Name))
				assert.Equal(t, token.Links[0].Spec.Cost, 1)
				assert.Equal(t, token.Links[0].Spec.TlsCredentials, clientSecret.Name)
				assert.Equal(t, len(token.Links[0].Spec.Endpoints), len(test.ra.Spec.Roles))
				var raRolesPorts = make(map[string]string)
				for _, role := range test.ra.Spec.Roles {
					raRolesPorts[role.Name] = strconv.Itoa(role.Port)
				}
				for _, endpoint := range token.Links[0].Spec.Endpoints {
					assert.Equal(t, endpoint.Port, raRolesPorts[endpoint.Name])
					assert.Assert(t, slices.Contains(test.expectedHosts, endpoint.Host),
						"endpoint host %q not expected in %v", endpoint.Host, test.expectedHosts)
					hostsFound[endpoint.Host] = true
				}
				assert.Equal(t, token.Secret.Name, clientSecret.Name)
			}
			for _, found := range hostsFound {
				assert.Assert(t, found, "not all hosts found: %v", hostsFound)
			}
		})
	}
}

func fakeRouterAccess() v1alpha1.RouterAccess {
	var ra v1alpha1.RouterAccess
	ra.Name = "fake-router-access"
	return ra
}

func fakeRouterAccessNoInterRouterEdgeRoles() v1alpha1.RouterAccess {
	var ra = fakeRouterAccess()
	ra.Spec.Roles = []v1alpha1.RouterAccessRole{
		{
			Name: "normal",
			Port: 5671,
		},
	}
	return ra
}

func fakeRouterAccessInterRouterRole() v1alpha1.RouterAccess {
	var ra = fakeRouterAccess()
	ra.Spec.Roles = []v1alpha1.RouterAccessRole{
		{
			Name: "inter-router",
			Port: 55671,
		},
	}
	return ra
}

func fakeRouterAccessBothRoles() v1alpha1.RouterAccess {
	var ra = fakeRouterAccessInterRouterRole()
	ra.Spec.Roles = append(ra.Spec.Roles, v1alpha1.RouterAccessRole{
		Name: "edge",
		Port: 45671,
	})
	return ra
}

func fakeRouterAccessWithSANs(sans ...string) v1alpha1.RouterAccess {
	var ra = fakeRouterAccessBothRoles()
	ra.Spec.SubjectAlternativeNames = sans
	return ra
}

func fakeServerSecretBad() v1.Secret {
	ca := certs.GenerateCASecret("fake-ca", "fake-ca")
	server := certs.GenerateSecret("fake-server-cert", "fake-server-cert", "", &ca)
	delete(server.Data, "tls.crt")
	return server
}

func fakeServerSecret(hosts []string) v1.Secret {
	hostsCsv := strings.Join(hosts, ",")
	ca := certs.GenerateCASecret("fake-ca", "fake-ca")
	server := certs.GenerateSecret("fake-server-cert", "fake-server-cert", hostsCsv, &ca)
	return server
}
