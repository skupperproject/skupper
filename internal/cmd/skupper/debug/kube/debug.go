package kube

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

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/kube/client"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	kube "github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils/validator"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type CmdDebug struct {
	Client     skupperv2alpha1.SkupperV2alpha1Interface
	KubeClient kubernetes.Interface
	CobraCmd   *cobra.Command
	Flags      *common.CommandDebugFlags
	Namespace  string
	fileName   string
	Rest       *restclient.Config
}

func NewCmdDebug() *CmdDebug {

	skupperCmd := CmdDebug{}

	return &skupperCmd
}

func (cmd *CmdDebug) NewClient(cobraCommand *cobra.Command, args []string) {
	cli, err := client.NewClient(cobraCommand.Flag("namespace").Value.String(), cobraCommand.Flag("context").Value.String(), cobraCommand.Flag("kubeconfig").Value.String())
	if err == nil {
		cmd.Client = cli.GetSkupperClient().SkupperV2alpha1()
		cmd.KubeClient = cli.GetKubeClient()
		cmd.Namespace = cli.Namespace

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		kubeConfigPath := cmd.CobraCmd.Flag("kubeconfig").Value.String()
		if kubeConfigPath != "" {
			loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeConfigPath}
		}
		kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{
				CurrentContext: cmd.CobraCmd.Flag("context").Value.String(),
			},
		)
		restconfig, err := kubeconfig.ClientConfig()
		if err != nil {
			return
		}
		restconfig.ContentConfig.GroupVersion = &schema.GroupVersion{Version: "v1"}
		restconfig.APIPath = "/api"
		restconfig.NegotiatedSerializer = serializer.WithoutConversionCodecFactory{CodecFactory: scheme.Codecs}
		cmd.Rest = restconfig
	}
}

func (cmd *CmdDebug) ValidateInput(args []string) []error {
	var validationErrors []error
	fileStringValidator := validator.NewFilePathStringValidator()

	// Validate dump file name
	if len(args) < 1 {
		validationErrors = append(validationErrors, fmt.Errorf("file name must be configured"))
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("file name must not be empty"))
	} else {
		ok, err := fileStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("file name is not valid: %s", err))
		} else {
			cmd.fileName = args[0]
		}
	}

	if cmd.Rest == nil {
		validationErrors = append(validationErrors, fmt.Errorf("failed setting up command"))
	}

	return validationErrors
}

func (cmd *CmdDebug) Run() error {
	configMaps := []string{"skupper-router", "skupper-network-status", "prometheus-server-config"}
	routerDeployments := []string{"skupper-router", "network-observer", "network-observer-prometheus"} //"skupper-service-controller", "skupper-prometheus"}
	controllerDeployments := []string{"skupper-controller"}
	services := []string{"skupper", "skupper-router", "skupper-router-local", "network-observer", "network-observer-prometheus", "skupper-grant-server"} //, "skupper-prometheus"}
	//routes := []string{"claims", "skupper", "skupper-edge", "skupper-inter-router"}
	qdstatFlags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	dumpFile := cmd.fileName

	// Add extension if not present
	if filepath.Ext(dumpFile) == "" {
		dumpFile = dumpFile + ".tar.gz"
	}

	tarFile, err := os.Create(dumpFile)
	if err != nil {
		return fmt.Errorf("Unable to save skupper dump details: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	kv, err := runCommand("kubectl", "version", "-o", "yaml")
	if err == nil {
		writeTar("/skupper-info/k8s-versions.yaml", kv, time.Now(), tw)
	}

	events, err := runCommand("kubectl", "events")
	if err == nil {
		writeTar("/skupper-info/events.txt", events, time.Now(), tw)
	}

	endpoints, err := runCommand("kubectl", "get", "endpoints", "-o", "yaml")
	if err == nil {
		writeTar("/skupper-info/endpoints.yaml", endpoints, time.Now(), tw)
	}

	manifest, err := runCommand("skupper", "version", "-o", "json")
	if err == nil {
		writeTar("/skupper-info/manifest.json", manifest, time.Now(), tw)
	}

	// get deployments router and/or controller
	err = getDeployments(cmd, routerDeployments, "skupper.io/component", qdstatFlags, tw)
	if err != nil {
		return err
	}

	err = getDeployments(cmd, controllerDeployments, "application", nil, tw)
	if err != nil {
		return err
	}

	for i := range configMaps {
		cm, err := cmd.KubeClient.CoreV1().ConfigMaps(cmd.Namespace).Get(context.TODO(), configMaps[i], metav1.GetOptions{})
		if err == nil {
			err := writeObject(cm, "/configmaps/"+cm.Name, ".yaml", tw)
			if err != nil {
				return err
			}
		}
	}

	for i := range services {
		service, err := cmd.KubeClient.CoreV1().Services(cmd.Namespace).Get(context.TODO(), services[i], metav1.GetOptions{})
		if err == nil {
			err := writeObject(service, "/services/"+service.Name, ".yaml", tw)
			if err != nil {
				return err
			}
		}
	}

	// get resources
	accessGrantList, err := cmd.Client.AccessGrants(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, grant := range accessGrantList.Items {
		g := grant.DeepCopy()
		err := writeObject(g, "/skupper-resource/accessgrants/"+g.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	accessTokenList, err := cmd.Client.AccessTokens(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, token := range accessTokenList.Items {
		t := token.DeepCopy()
		err := writeObject(t, "/skupper-resource/accessTokens/"+t.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	attachedConnectorBindingList, err := cmd.Client.AttachedConnectorBindings(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, binding := range attachedConnectorBindingList.Items {
		b := binding.DeepCopy()
		err := writeObject(b, "/skupper-resource/attachedConnectorBinding/"+b.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	attachedConnectorList, err := cmd.Client.AttachedConnectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, attachedConnector := range attachedConnectorList.Items {
		a := attachedConnector.DeepCopy()
		err := writeObject(a, "/skupper-resource/attachedConnector/"+a.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	certificateList, err := cmd.Client.Certificates(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, certificate := range certificateList.Items {
		c := certificate.DeepCopy()
		err := writeObject(c, "/skupper-resource/certificate/"+c.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	connectorList, err := cmd.Client.Connectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, connector := range connectorList.Items {
		c := connector.DeepCopy()
		err := writeObject(c, "/skupper-resource/connector/"+c.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	linkList, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, link := range linkList.Items {
		l := link.DeepCopy()
		err := writeObject(l, "/skupper-resource/link/"+l.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	listenerList, err := cmd.Client.Listeners(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, listener := range listenerList.Items {
		l := listener.DeepCopy()
		err := writeObject(l, "/skupper-resource/listener/"+l.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, site := range siteList.Items {
		s := site.DeepCopy()
		err := writeObject(s, "/skupper-resource/site/"+s.Name, ".yaml", tw)
		if err != nil {
			return err
		}
	}

	fmt.Println("Skupper dump details written to compressed archive: ", dumpFile)
	return nil
}

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

// helper functions

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

func hasRestartedContainer(pod v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.RestartCount > 0 {
			return true
		}
	}
	return false
}

func getDeployments(cmd *CmdDebug, deployments []string, componentLabel string, flags []string, tw *tar.Writer) error {

	for i := range deployments {
		deployment, err := cmd.KubeClient.AppsV1().Deployments(cmd.Namespace).Get(context.TODO(), deployments[i], metav1.GetOptions{})
		if err != nil {
			continue
		}

		err = writeObject(deployment, "/deployments/"+deployment.Name, ".yaml", tw)
		if err != nil {
			return err
		}

		component, ok := deployment.Spec.Template.Labels[componentLabel]
		if !ok {
			continue
		}

		podList, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: componentLabel + "=" + component})
		if err != nil {
			continue
		}

		for _, pod := range podList.Items {
			pod, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err != nil {
				continue
			} else {
				err := writeObject(pod, "/pods/"+pod.Name+"/pod", ".yaml", tw)
				if err != nil {
					return err
				}
			}
			top, err := runCommand("kubectl", "top", "pod", pod.Name)
			if err == nil {
				writeTar("/pods/"+pod.Name+"/top-pod.txt", top, time.Now(), tw)
			}

			for container := range pod.Spec.Containers {

				if pod.Spec.Containers[container].Name == "router" {
					// while we are here collect qdstats, logs will show these operations
					for x := range flags {
						qdr, err := client.ExecCommandInContainer([]string{"skstat", flags[x]}, pod.Name, "router", cmd.Namespace, cmd.KubeClient, cmd.Rest)
						if err == nil {
							writeTar("/pods/"+pod.Name+"/skstat/skstat"+flags[x]+".txt", qdr.Bytes(), time.Now(), tw)
						} else {
							continue
						}
					}
				}

				log, err := kube.GetPodContainerLogs(pod.Name, pod.Spec.Containers[container].Name, cmd.Namespace, cmd.KubeClient)
				if err == nil {
					writeTar("/pods/"+pod.Name+"/logs/"+pod.Spec.Containers[container].Name+"-logs.txt", []byte(log), time.Now(), tw)
				}

				if hasRestartedContainer(*pod) {
					prevLog, err := kube.GetPodContainerLogsWithOpts(pod.Name, pod.Spec.Containers[container].Name, cmd.Namespace, cmd.KubeClient, v1.PodLogOptions{Previous: true})
					if err == nil {
						writeTar("/pods/"+pod.Name+"/logs/"+pod.Spec.Containers[container].Name+"-logs-previous.txt", []byte(prevLog), time.Now(), tw)
					}
				}
			}
		}
	}
	return nil
}

func (cmd *CmdDebug) InputToOptions()  {}
func (cmd *CmdDebug) WaitUntil() error { return nil }
