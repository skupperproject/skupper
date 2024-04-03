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
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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

func writeObject(rto runtime.Object, path string, ext string, tw *tar.Writer) error {
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	if err := s.Encode(rto, &b); err != nil {
		return err
	}
	return writeTar(path+ext, b.Bytes(), time.Now(), tw)
}

func (cli *VanClient) SkupperDump(ctx context.Context, tarName string, version string, kubeConfigPath string, kubeConfigContext string) (string, error) {
	configMaps := []string{types.SiteConfigMapName, types.ServiceInterfaceConfigMap, types.TransportConfigMapName, "skupper-sasl-config", types.NetworkStatusConfigMapName, types.SiteLeaderLockName}
	deployments := []string{"skupper-site-controller", "skupper-router", "skupper-service-controller", "skupper-prometheus"}
	services := []string{"skupper", "skupper-router", "skupper-router-local", "skupper-prometheus"}
	routes := []string{"claims", "skupper", "skupper-edge", "skupper-inter-router"}
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

	kv, err := runCommand("kubectl", "version", "-o", "yaml", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
	if err == nil {
		writeTar("/skupper-info/k8s-versions.yaml", kv, time.Now(), tw)
	}

	events, err := runCommand("kubectl", "events", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
	if err == nil {
		writeTar("/skupper-info/events.txt", events, time.Now(), tw)
	}

	endpoints, err := runCommand("kubectl", "get", "endpoints", "-o", "yaml", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
	if err == nil {
		writeTar("/skupper-info/endpoints.yaml", endpoints, time.Now(), tw)
	}

	if cli.RouteClient != nil {
		ocv, err := runCommand("oc", "version", "-o", "yaml", "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
		if err == nil {
			writeTar("/skupper-info/oc-versions.yaml", ocv, time.Now(), tw)
		}
	}

	_, err = runCommand("skupper", "version", "manifest")
	if err == nil {
		manifest, err := os.ReadFile("./manifest.json")
		if err == nil {
			writeTar("/skupper-info/manifest.json", manifest, time.Now(), tw)
			os.Remove("./manifest.json")
		}
	}

	for i := range deployments {
		deployment, err := cli.KubeClient.AppsV1().Deployments(cli.Namespace).Get(context.TODO(), deployments[i], metav1.GetOptions{})
		if err != nil {
			continue
		}
		err = writeObject(deployment, "/deployments/"+deployment.Name, ".yaml", tw)
		if err != nil {
			return dumpFile, err
		}

		component, ok := deployment.Spec.Template.Labels["skupper.io/component"]
		if !ok {
			continue
		}

		podList, err := cli.KubeClient.CoreV1().Pods(cli.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: "skupper.io/component=" + component})
		if err != nil {
			continue
		}

		for _, pod := range podList.Items {
			pod, err := cli.KubeClient.CoreV1().Pods(cli.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err != nil {
				continue
			} else {
				err := writeObject(pod, "/pods/"+pod.Name+"/pod", ".yaml", tw)
				if err != nil {
					return dumpFile, err
				}
			}
			top, err := runCommand("kubectl", "top", "pod", pod.Name, "--kubeconfig="+kubeConfigPath, "--context="+kubeConfigContext)
			if err == nil {
				writeTar("/pods/"+pod.Name+"/top-pod.txt", top, time.Now(), tw)
			}

			for container := range pod.Spec.Containers {
				if pod.Spec.Containers[container].Name == "router" {
					// while we are here collect qdstats, logs will show these operations
					for x := range qdstatFlags {
						qdr, err := kube.ExecCommandInContainer([]string{"skstat", qdstatFlags[x]}, pod.Name, "router", cli.Namespace, cli.KubeClient, cli.RestConfig)
						if err == nil {
							writeTar("/pods/"+pod.Name+"/skstat/skstat"+qdstatFlags[x]+".txt", qdr.Bytes(), time.Now(), tw)
						} else {
							continue

						}
					}
				} else if pod.Spec.Containers[container].Name == "service-controller" {
					events, err := kube.ExecCommandInContainer([]string{"get", "events"}, pod.Name, "service-controller", cli.Namespace, cli.KubeClient, cli.RestConfig)
					if err == nil {
						writeTar("/pods/"+pod.Name+"/events/"+pod.Spec.Containers[container].Name+"-events.txt", events.Bytes(), time.Now(), tw)
					}
					eventsJson, err := kube.ExecCommandInContainer([]string{"get", "events", "-o", "json"}, pod.Name, "service-controller", cli.Namespace, cli.KubeClient, cli.RestConfig)
					if err == nil {
						writeTar("/pods/"+pod.Name+"/events/"+pod.Spec.Containers[container].Name+"-events.json", eventsJson.Bytes(), time.Now(), tw)
					}
					policies, err := kube.ExecCommandInContainer([]string{"get", "policies", "list"}, pod.Name, "service-controller", cli.Namespace, cli.KubeClient, cli.RestConfig)
					if err == nil {
						writeTar("/pods/"+pod.Name+"/policies/"+pod.Spec.Containers[container].Name+"-policies.txt", policies.Bytes(), time.Now(), tw)
					}
				}

				log, err := kube.GetPodContainerLogs(pod.Name, pod.Spec.Containers[container].Name, cli.Namespace, cli.KubeClient)
				if err == nil {
					writeTar("/pods/"+pod.Name+"/logs/"+pod.Spec.Containers[container].Name+"-logs.txt", []byte(log), time.Now(), tw)
				}

				if hasRestartedContainer(*pod) {
					prevLog, err := kube.GetPodContainerLogsWithOpts(pod.Name, pod.Spec.Containers[container].Name, cli.Namespace, cli.KubeClient, v1.PodLogOptions{Previous: true})
					if err == nil {
						writeTar("/pods/"+pod.Name+"/logs/"+pod.Spec.Containers[container].Name+"-logs-previous.txt", []byte(prevLog), time.Now(), tw)
					}
				}

			}
		}
	}

	for i := range configMaps {
		cm, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(context.TODO(), configMaps[i], metav1.GetOptions{})
		if err == nil {
			err := writeObject(cm, "/configmaps/"+cm.Name, ".yaml", tw)
			if err != nil {
				return dumpFile, err
			}
		}
	}

	for i := range services {
		service, err := cli.KubeClient.CoreV1().Services(cli.Namespace).Get(context.TODO(), services[i], metav1.GetOptions{})
		if err == nil {
			err := writeObject(service, "/services/"+service.Name, ".yaml", tw)
			if err != nil {
				return dumpFile, err
			}
		}
	}

	if cli.RouteClient != nil {
		for i := range routes {
			route, err := cli.GetRouteClient().Routes(cli.Namespace).Get(context.TODO(), routes[i], metav1.GetOptions{})
			if err == nil {
				err := writeObject(route, "/routes/"+route.Name, ".yaml", tw)
				if err != nil {
					return dumpFile, err
				}
			}
		}
	}

	return dumpFile, nil
}

func hasRestartedContainer(pod v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.RestartCount > 0 {
			return true
		}
	}
	return false
}

func (cli *VanClient) execInServiceControllerPod(command []string) (*bytes.Buffer, error) {
	pods, err := kube.GetPods("skupper.io/component=service-controller", cli.Namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}
	if len(pods) < 1 {
		return nil, fmt.Errorf("No service-controller pod found")
	}
	return kube.ExecCommandInContainer(command, pods[0].Name, "service-controller", cli.Namespace, cli.KubeClient, cli.RestConfig)
}

func addOutputFlag(command []string, verbose bool) []string {
	if verbose {
		return append(command, "-o=json")
	}
	return command
}

func (cli *VanClient) SkupperEvents(verbose bool) (*bytes.Buffer, error) {
	return cli.execInServiceControllerPod(addOutputFlag([]string{"get", "events"}, verbose))
}

func (cli *VanClient) SkupperCheckService(service string, verbose bool) (*bytes.Buffer, error) {
	return cli.execInServiceControllerPod(addOutputFlag([]string{"get", "servicecheck", service}, verbose))
}

func (cli *VanClient) SkupperPolicies(verbose bool) (*bytes.Buffer, error) {
	return cli.execInServiceControllerPod(addOutputFlag([]string{"get", "policies", "list"}, verbose))
}
