package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
)

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func writeTar(name string, data []byte, ts time.Time, tw *tar.Writer) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(data)),
		ModTime: ts,
	}
	err := tw.WriteHeader(hdr)
	if err != nil {
		return fmt.Errorf("Failed to write tar file header: %w", err)
	}
	_, err = tw.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write to tar archive: %w", err)
	}
	return nil
}

func (cli *VanClient) writeDeployment(name string, tw *tar.Writer) error {
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	deployment, err := kube.GetDeployment(name, cli.Namespace, cli.KubeClient)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err = s.Encode(deployment, &b); err != nil {
		return err
	}

	return writeTar(name+"-deployment.yaml", b.Bytes(), time.Now(), tw)
}

func (cli *VanClient) writeConfigMap(name string, tw *tar.Writer) error {
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	cm, err := kube.GetConfigMap(name, cli.Namespace, cli.KubeClient)
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	if err = s.Encode(cm, &b); err != nil {
		return err
	}

	return writeTar(name+"-configmap.yaml", b.Bytes(), time.Now(), tw)
}

func (cli *VanClient) SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) (string, error) {
	configMaps := []string{types.SiteConfigMapName, types.ServiceInterfaceConfigMap, types.TransportConfigMapName, "skupper-sasl-config"}
	deployments := []string{"skupper-site-controller", "skupper-router", "skupper-service-controller"}
	qdstatFlags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	dumpFile := tarName

	// Add extension if not present
	if filepath.Ext(dumpFile) == "" {
		dumpFile = dumpFile + ".tar.gz"
	}

	tarFile, err := os.Create(dumpFile)
	if err != nil {
		return dumpFile, err
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	kv, err := runCommand("kubectl", "version", "--short", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
	if err == nil {
		writeTar("k8s-versions.txt", kv, time.Now(), tw)
	}

	if cli.RouteClient != nil {
		ocv, err := runCommand("oc", "version", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
		if err == nil {
			writeTar("oc-versions.txt", ocv, time.Now(), tw)
		}
	}

	var cversions []string
	vir, err := cli.RouterInspect(context.Background())
	if err == nil {
		cversions = append(cversions, fmt.Sprintf("%-30s %s", "client version", version))
		cversions = append(cversions, fmt.Sprintf("%-30s %s", "transport version", vir.TransportVersion))
		cversions = append(cversions, fmt.Sprintf("%-30s %s\n", "controller version", vir.ControllerVersion))
		writeTar("skupper-versions.txt", []byte(strings.Join(cversions, "\n")), time.Now(), tw)
	}

	for i := range deployments {
		err := cli.writeDeployment(deployments[i], tw)
		if err != nil {
			return dumpFile, err
		}

		component := kube.GetDeploymentLabel(deployments[i], "skupper.io/component", cli.Namespace, cli.KubeClient)

		podList, err := kube.GetPods("skupper.io/component="+component, cli.Namespace, cli.KubeClient)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return dumpFile, err
		}
		for _, pod := range podList {
			for container := range pod.Spec.Containers {
				if pod.Spec.Containers[container].Name == "router" {
					// while we are here collect qdstats, logs will show these operations
					for x := range qdstatFlags {
						qdr, err := kube.ExecCommandInContainer([]string{"qdstat", qdstatFlags[x]}, pod.Name, "router", cli.Namespace, cli.KubeClient, cli.RestConfig)
						if err == nil {
							writeTar(pod.Name+"-qdstat"+qdstatFlags[x]+".txt", qdr.Bytes(), time.Now(), tw)
						} else {
							continue

						}
					}
				} else if pod.Spec.Containers[container].Name == "service-controller" {
					events, err := kube.ExecCommandInContainer([]string{"get", "events"}, pod.Name, "service-controller", cli.Namespace, cli.KubeClient, cli.RestConfig)
					if err == nil {
						writeTar(pod.Name+"-events.txt", events.Bytes(), time.Now(), tw)
					}
				}

				log, err := kube.GetPodContainerLogs(pod.Name, pod.Spec.Containers[container].Name, cli.Namespace, cli.KubeClient)
				if err == nil {
					writeTar(pod.Name+"-"+pod.Spec.Containers[container].Name+"-logs.txt", []byte(log), time.Now(), tw)
				}
			}
		}
	}

	for i := range configMaps {
		err := cli.writeConfigMap(configMaps[i], tw)
		if err != nil {
			return dumpFile, err
		}
	}
	return dumpFile, nil
}
