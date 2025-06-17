package kube

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/skupperproject/skupper/internal/cmd/skupper/common"
	"github.com/skupperproject/skupper/internal/cmd/skupper/common/utils"
	"github.com/skupperproject/skupper/internal/kube/client"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/utils/validator"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v2alpha1"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"

	crdClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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
	crdClient  *crdClient.Clientset
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

		cmd.crdClient, err = crdClient.NewForConfig(cmd.Rest)
		if err != nil {
			return
		}
	}
}

func (cmd *CmdDebug) ValidateInput(args []string) error {
	var validationErrors []error
	fileStringValidator := validator.NewFilePathStringValidator()

	// Validate dump file name
	if len(args) < 1 {
		cmd.fileName = "skupper-dump"
	} else if len(args) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("only one argument is allowed for this command"))
	} else if args[0] == "" {
		validationErrors = append(validationErrors, fmt.Errorf("filename must not be empty"))
	} else {
		ok, err := fileStringValidator.Evaluate(args[0])
		if !ok {
			validationErrors = append(validationErrors, fmt.Errorf("filename is not valid: %s", err))
		} else {
			cmd.fileName = args[0]
		}
	}

	if cmd.Rest == nil {
		validationErrors = append(validationErrors, fmt.Errorf("failed setting up command"))
	}

	return errors.Join(validationErrors...)
}

func (cmd *CmdDebug) InputToOptions() {
	datetime := time.Now().Format("20060102150405")
	cmd.fileName = fmt.Sprintf("%s-%s-%s", cmd.fileName, cmd.Namespace, datetime)
}

func (cmd *CmdDebug) Run() error {
	configMaps := []string{"skupper-router", "skupper-network-status", "prometheus-server-config"}
	routerServices := []string{"skupper", "skupper-router", "skupper-router-local", "network-observer", "network-observer-prometheus", "skupper-grant-server"} //, "skupper-prometheus"}
	controllerServices := []string{"skupper-grant-server"}
	//routes := []string{"claims", "skupper", "skupper-edge", "skupper-inter-router"}

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

	kv, err := utils.RunCommand("kubectl", "version", "-o", "yaml")
	if err == nil {
		utils.WriteTar("/versions/kubernetes.yaml", kv, time.Now(), tw)
		utils.WriteTar("/versions/kubernetes.yaml.txt", kv, time.Now(), tw)
	}

	manifest, err := utils.RunCommand("skupper", "version", "-o", "yaml")
	if err == nil {
		utils.WriteTar("/versions/skupper.yaml", manifest, time.Now(), tw)
		utils.WriteTar("/versions/skupper.yaml.txt", manifest, time.Now(), tw)
	}

	// get resources for skupper-router
	site, err := cmd.KubeClient.AppsV1().Deployments(cmd.Namespace).Get(context.TODO(), "skupper-router", metav1.GetOptions{})
	if site != nil && err == nil {
		path := "/site-namespace/"
		rPath := path + "resources/"
		events, err := utils.RunCommand("kubectl", "events")
		if err == nil {
			utils.WriteTar(path+"events.txt", events, time.Now(), tw)
		}

		endpoints, err := utils.RunCommand("kubectl", "get", "endpoints", "-o", "yaml")
		if err == nil {
			ePath := rPath + "Endpoints-skupper-router-" + cmd.Namespace + ".yaml"
			utils.WriteTar(ePath, endpoints, time.Now(), tw)
			utils.WriteTar(ePath+".txt", endpoints, time.Now(), tw)
		}

		err = getDeployments(cmd, path, "skupper-router", tw)
		if err != nil {
			return err
		}

		// List all the existing installed CRs in the cluster
		path = path + "resources/"
		if cmd.crdClient != nil {
			crdList, err := cmd.crdClient.ApiextensionsV1().CustomResourceDefinitions().List(context.TODO(), metav1.ListOptions{})
			if err == nil && crdList != nil {
				var encodedOutput []byte
				var crds []string
				for _, crd := range crdList.Items {
					crds = append(crds, crd.Name)
				}
				encodedOutput, err = yaml.Marshal(crds)
				if err == nil {
					utils.WriteTar(path+"crds.txt", encodedOutput, time.Now(), tw)
				}
			}
		}

		for i := range configMaps {
			cm, err := cmd.KubeClient.CoreV1().ConfigMaps(cmd.Namespace).Get(context.TODO(), configMaps[i], metav1.GetOptions{})
			if err == nil {
				err := utils.WriteObject(cm, rPath+"Configmap-"+cm.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		for _, service := range routerServices {
			service, err := cmd.KubeClient.CoreV1().Services(cmd.Namespace).Get(context.TODO(), service, metav1.GetOptions{})
			if err == nil {
				err := utils.WriteObject(service, rPath+"Services-"+service.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		accessGrantList, err := cmd.Client.AccessGrants(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if accessGrantList != nil && err == nil {
			for _, grant := range accessGrantList.Items {
				g := grant.DeepCopy()
				err := utils.WriteObject(g, rPath+"Accessgrant-"+g.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		accessTokenList, err := cmd.Client.AccessTokens(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if accessTokenList != nil && err == nil {
			for _, token := range accessTokenList.Items {
				t := token.DeepCopy()
				err := utils.WriteObject(t, rPath+"AccessTokens-"+t.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		attachedConnectorBindingList, err := cmd.Client.AttachedConnectorBindings(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if attachedConnectorBindingList != nil && err == nil {
			for _, binding := range attachedConnectorBindingList.Items {
				b := binding.DeepCopy()
				err := utils.WriteObject(b, rPath+"AttachedConnectorBinding-"+b.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		attachedConnectorList, err := cmd.Client.AttachedConnectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if attachedConnectorList != nil && err == nil {
			for _, attachedConnector := range attachedConnectorList.Items {
				a := attachedConnector.DeepCopy()
				err := utils.WriteObject(a, rPath+"AttachedConnector-"+a.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		certificateList, err := cmd.Client.Certificates(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if certificateList != nil && err == nil {
			for _, certificate := range certificateList.Items {
				c := certificate.DeepCopy()
				err := utils.WriteObject(c, rPath+"Certificate-"+c.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		connectorList, err := cmd.Client.Connectors(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if connectorList != nil && err == nil {
			for _, connector := range connectorList.Items {
				c := connector.DeepCopy()
				err := utils.WriteObject(c, rPath+"Connector-"+c.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		linkList, err := cmd.Client.Links(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if linkList != nil && err == nil {
			for _, link := range linkList.Items {
				l := link.DeepCopy()
				err := utils.WriteObject(l, rPath+"Link-"+l.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		listenerList, err := cmd.Client.Listeners(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if listenerList != nil && err == nil {
			for _, listener := range listenerList.Items {
				l := listener.DeepCopy()
				err := utils.WriteObject(l, rPath+"Listener-"+l.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		siteList, err := cmd.Client.Sites(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if siteList != nil && err == nil {
			for _, site := range siteList.Items {
				s := site.DeepCopy()
				err := utils.WriteObject(s, rPath+"Site-"+s.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		routerAccessList, err := cmd.Client.RouterAccesses(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if routerAccessList != nil && err == nil {
			for _, site := range routerAccessList.Items {
				s := site.DeepCopy()
				err := utils.WriteObject(s, rPath+"RouterAccess-"+s.Name, tw)
				if err != nil {
					return err
				}
			}
		}

		securedAccessList, err := cmd.Client.SecuredAccesses(cmd.Namespace).List(context.TODO(), metav1.ListOptions{})
		if securedAccessList != nil && err == nil {
			for _, site := range securedAccessList.Items {
				s := site.DeepCopy()
				err := utils.WriteObject(s, rPath+"SecuredAccess-"+s.Name, tw)
				if err != nil {
					return err
				}
			}
		}
	}

	// get resources for skupper-controller
	controller, err := cmd.KubeClient.AppsV1().Deployments(cmd.Namespace).Get(context.TODO(), "skupper-controller", metav1.GetOptions{})
	if controller != nil && err == nil {
		path := "/controller-namespace/"
		rPath := path + "resources/"

		events, err := utils.RunCommand("kubectl", "events")
		if err == nil {
			utils.WriteTar(path+"events.txt", events, time.Now(), tw)
		}

		endpoints, err := utils.RunCommand("kubectl", "get", "endpoints", "-o", "yaml")
		if err == nil {
			ePath := rPath + "Endpoints-skuper-controller.yaml"
			utils.WriteTar(ePath, endpoints, time.Now(), tw)
			utils.WriteTar(ePath+".txt", endpoints, time.Now(), tw)
		}

		err = getDeployments(cmd, path, "skupper-controller", tw)
		if err != nil {
			return err
		}

		for i := range controllerServices {
			service, err := cmd.KubeClient.CoreV1().Services(cmd.Namespace).Get(context.TODO(), controllerServices[i], metav1.GetOptions{})
			if err == nil {
				err := utils.WriteObject(service, rPath+"Services-"+service.Name, tw)
				if err != nil {
					return err
				}
			}
		}
	}
	fmt.Println("Skupper dump details written to compressed archive: ", dumpFile)
	return nil
}

// helper functions

func hasRestartedContainer(pod v1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.RestartCount > 0 {
			return true
		}
	}
	return false
}

func getDeployments(cmd *CmdDebug, path string, deploymentType string, tw *tar.Writer) error {
	routerDeployments := []string{"skupper-router", "network-observer", "network-observer-prometheus"}
	controllerDeployments := []string{"skupper-controller"}
	flags := []string{"-g", "-c", "-l", "-n", "-e", "-a", "-m", "-p"}

	deployments := routerDeployments
	labelSelector := "app.kubernetes.io/name="
	if deploymentType == "skupper-controller" {
		deployments = controllerDeployments
		labelSelector = "application="
	}

	rPath := path + "resources/"
	for i := range deployments {
		deployment, err := cmd.KubeClient.AppsV1().Deployments(cmd.Namespace).Get(context.TODO(), deployments[i], metav1.GetOptions{})
		if err != nil {
			continue
		}

		err = utils.WriteObject(deployment, rPath+"Deployment-"+deployment.Name, tw)
		if err != nil {
			return err
		}

		podList, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector + deployments[i]})
		if err != nil {
			continue
		}

		for _, pod := range podList.Items {
			pod, err := cmd.KubeClient.CoreV1().Pods(cmd.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
			if err != nil {
				continue
			} else {
				err := utils.WriteObject(pod, rPath+"Pod-"+pod.Name, tw)
				if err != nil {
					return err
				}
			}
			top, err := utils.RunCommand("kubectl", "top", "pod", pod.Name)
			if err == nil {
				utils.WriteTar(rPath+pod.Name+"/top-pod.txt", top, time.Now(), tw)
			}

			for container := range pod.Spec.Containers {
				if pod.Spec.Containers[container].Name == "router" {
					// while we are here collect qdstats, logs will show these operations
					for x := range flags {
						qdr, err := client.ExecCommandInContainer([]string{"skstat", flags[x]}, pod.Name, "router", cmd.Namespace, cmd.KubeClient, cmd.Rest)
						if err == nil {
							utils.WriteTar(rPath+"skstat/"+pod.Name+"-skstat"+flags[x]+".txt", qdr.Bytes(), time.Now(), tw)
						} else {
							continue
						}
					}
				}

				log, err := internalclient.GetPodContainerLogs(pod.Name, pod.Spec.Containers[container].Name, cmd.Namespace, cmd.KubeClient)
				if err == nil {
					utils.WriteTar(path+"logs/"+pod.Name+"-"+pod.Spec.Containers[container].Name+".txt", []byte(log), time.Now(), tw)
				}

				if hasRestartedContainer(*pod) {
					prevLog, err := internalclient.GetPodContainerLogsWithOpts(pod.Name, pod.Spec.Containers[container].Name, cmd.Namespace, cmd.KubeClient, v1.PodLogOptions{Previous: true})
					if err == nil {
						utils.WriteTar(path+"logs/"+pod.Name+"-"+pod.Spec.Containers[container].Name+"-previous.txt", []byte(prevLog), time.Now(), tw)
					}
				}
			}
		}

		role, err := cmd.KubeClient.RbacV1().Roles(cmd.Namespace).Get(context.TODO(), deployments[i], metav1.GetOptions{})
		if err == nil && role != nil {
			err = utils.WriteObject(role, rPath+"Role-"+deployment.Name, tw)
			if err != nil {
				return err
			}
		}

		roleBinding, err := cmd.KubeClient.RbacV1().RoleBindings(cmd.Namespace).Get(context.TODO(), deployments[i], metav1.GetOptions{})
		if err == nil && roleBinding != nil {
			err = utils.WriteObject(roleBinding, rPath+"RoleBinding-"+deployment.Name, tw)
			if err != nil {
				return err
			}
		}

		replicaSetList, err := cmd.KubeClient.AppsV1().ReplicaSets(cmd.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector + deployments[i]})
		if err == nil && replicaSetList != nil {
			for _, replicaSet := range replicaSetList.Items {
				r := replicaSet.DeepCopy()
				err = utils.WriteObject(r, rPath+"ReplicaSet-"+replicaSet.Name, tw)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (cmd *CmdDebug) WaitUntil() error { return nil }
