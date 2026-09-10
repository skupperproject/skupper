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
			fmt.Sprintf("-listen-metrics=127.0.0.1:%d", p.metrics),
			fmt.Sprintf("-prometheus-api=http://127.0.0.1:%d", p.prometheus),
			fmt.Sprintf("-router-endpoint=%s", p.router),
			"-router-tls-ca=/etc/messaging/ca.crt",
			"-router-tls-cert=/etc/messaging/tls.crt",
			"-router-tls-key=/etc/messaging/tls.key",
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

func GetHostPrometheusContainer(p prometheusInstallerPorts) container.Container {
	prometheusHome := api.GetHostPrometheusHome()
	dataPath := filepath.Join(prometheusHome, "data")

	return container.Container{
		Name:  "skupper-prometheus",
		Image: images.GetPrometheusImageName(),
		Command: []string{
			"--config.file=/etc/prometheus/prometheus.yml",
			"--storage.tsdb.path=/prometheus/",
			fmt.Sprintf("--web.listen-address=127.0.0.1:%d", p.prometheus),
		},
		Env: map[string]string{},
		Labels: map[string]string{
			"application":             "skupper-v2",
			"skupper.io/v2-component": "prometheus",
		},
		FileMounts: []container.FileMount{
			{
				Source:      prometheusHome,
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
