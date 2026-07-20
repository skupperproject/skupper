package networkobserver

import (
	"fmt"
	"path/filepath"

	"github.com/skupperproject/skupper/internal/images"
	"github.com/skupperproject/skupper/pkg/container"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

func GetNetworkObserverContainer(namespace string, p ports) container.Container {
	namespacePath := api.GetHostNamespaceHome(namespace)
	clientCertsPath := filepath.Join(namespacePath, string(api.CertificatesPath), "skupper-local-client")

	return container.Container{
		Name:  fmt.Sprintf("%s-skupper-network-observer", namespace),
		Image: images.GetNetworkObserverImageName(),
		Command: []string{
			fmt.Sprintf("-listen=127.0.0.1:%d", p.netobs),
			fmt.Sprintf("-prometheus-api=http://127.0.0.1:%d", p.prometheus),
			fmt.Sprintf("-router-endpoint=%s", p.router),
			"-router-tls-ca=/etc/messaging/ca.crt",
			"-router-tls-cert=/etc/messaging/tls.crt",
			"-router-tls-key=/etc/messaging/tls.key",
			fmt.Sprintf("-listen-metrics=:%d", p.metrics),
		},
		Env: map[string]string{},
		Labels: map[string]string{
			"application":             "skupper-v2",
			"skupper.io/v2-component": "network-observer",
		},
		FileMounts: []container.FileMount{
			{
				Source:      clientCertsPath,
				Destination: "/etc/messaging",
				Options:     []string{"z"},
			},
		},
		Networks:      map[string]container.ContainerNetworkInfo{},
		RestartPolicy: "always",
	}
}

func GetPrometheusContainer(namespace string, p ports) container.Container {
	namespacePath := api.GetHostNamespaceHome(namespace)
	prometheusDir := filepath.Join(namespacePath, "network-observer", "prometheus")
	dataPath := filepath.Join(namespacePath, "network-observer", "prometheus", "data")

	return container.Container{
		Name:  fmt.Sprintf("%s-skupper-prometheus", namespace),
		Image: images.GetPrometheusImageName(),
		Command: []string{
			"--config.file=/etc/prometheus/prometheus.yml",
			"--storage.tsdb.path=/prometheus/",
			fmt.Sprintf("--web.listen-address=:%d", p.prometheus),
		},
		Env: map[string]string{},
		Labels: map[string]string{
			"application":             "skupper-v2",
			"skupper.io/v2-component": "prometheus",
		},
		FileMounts: []container.FileMount{
			{
				Source:      prometheusDir,
				Destination: "/etc/prometheus",
				Options:     []string{"z"},
			},
			{
				Source:      dataPath,
				Destination: "/prometheus",
				Options:     []string{"z"},
			},
		},
		Networks:      map[string]container.ContainerNetworkInfo{},
		RestartPolicy: "always",
	}
}

func GetNginxContainer(namespace string) container.Container {
	namespacePath := api.GetHostNamespaceHome(namespace)
	nginxConfDir := filepath.Join(namespacePath, "network-observer", "nginx", "conf.d")
	htpasswdDir := filepath.Join(namespacePath, "network-observer", "htpasswd")
	certsPath := filepath.Join(namespacePath, "network-observer", "certs")

	return container.Container{
		Name:  fmt.Sprintf("%s-skupper-nginx", namespace),
		Image: images.GetNginxImageName(),
		Env:   map[string]string{},
		Labels: map[string]string{
			"application":             "skupper-v2",
			"skupper.io/v2-component": "nginx-proxy",
		},
		FileMounts: []container.FileMount{
			{
				Source:      nginxConfDir,
				Destination: "/etc/nginx/conf.d",
				Options:     []string{"z"},
			},
			{
				Source:      certsPath,
				Destination: "/etc/certificates",
				Options:     []string{"z"},
			},
			{
				Source:      htpasswdDir,
				Destination: "/etc/httpusers",
				Options:     []string{"z"},
			},
		},
		Networks:      map[string]container.ContainerNetworkInfo{},
		RestartPolicy: "always",
	}
}
