package base

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	vanClient "github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/k8s"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

// ClusterContext represents a cluster that is available for testing
type ClusterContext struct {
	Namespace  string
	nsCreated  bool
	KubeConfig string
	VanClient  *vanClient.VanClient
	Private    bool
	Id         int
}

func _exec(command string) ([]byte, error) {
	var output []byte
	var err error
	fmt.Println(command)
	cmd := exec.Command("sh", "-c", command)
	output, err = cmd.CombinedOutput()
	fmt.Println(string(output))
	return output, err
}

func (cc *ClusterContext) exec(main_command string, sub_command string) ([]byte, error) {
	return _exec("KUBECONFIG=" + cc.KubeConfig + " " + main_command + " " + cc.Namespace + " " + sub_command)
}

func (cc *ClusterContext) KubectlExec(command string) ([]byte, error) {
	return cc.exec("kubectl -n ", command)
}

func (cc *ClusterContext) CreateNamespace() error {
	if ShouldSkipNamespaceSetup() {
		log.Printf("Skipping namespace creation for %v", cc.Namespace)
		ns, err := cc.VanClient.KubeClient.CoreV1().Namespaces().Get(cc.Namespace, metav1.GetOptions{})
		if err == nil {
			if ns != nil {
				// As we're skipping the creation of namespaces, we're adopting whatever
				// we find; we will destroy these when DeleteNamespace is called, unless
				// ShouldSkipNamespaceTeardown returns true.
				log.Printf("Reusing existing namespace %v", cc.Namespace)
				cc.nsCreated = true
			} else {
				// Assertion; this should never happen
				return fmt.Errorf("Namespace check returned nil response, but no errors")
			}
		} else {
			if errors.IsNotFound(err) {
				return fmt.Errorf("Namespace %v did not exist and namespace creation skipping was requested", cc.Namespace)
			}
			return err
		}
	}
	_, err := kube.NewNamespace(cc.Namespace, cc.VanClient.KubeClient)
	if err == nil {
		cc.nsCreated = true
	}
	return err
}

func (cc *ClusterContext) DeleteNamespace() error {
	if !cc.nsCreated {
		log.Printf("namespace [%s] will not be deleted as it was not created by ClusterContext", cc.Namespace)
		return nil
	}
	if ShouldSkipNamespaceTeardown() {
		log.Print("Skipping namespace tear down, per env variables")
		return nil
	}
	if err := k8s.DeleteNamespaceAndWait(cc.VanClient.KubeClient, cc.Namespace); err != nil {
		return err
	}

	cc.nsCreated = false
	return nil
}

// As the name says, it will add label to this namespace
func (cc *ClusterContext) LabelNamespace(label string, value string) (err error) {

	payload := fmt.Sprintf(`{"metadata": {"labels": {"%v": "%v"}}}`, label, value)

	_, err = cc.VanClient.KubeClient.CoreV1().Namespaces().Patch(cc.Namespace, types.MergePatchType, []byte(payload))

	return
}

func (cc *ClusterContext) waitForSkupperServiceToBeCreated(name string, retryFn func() (*apiv1.Service, error), backoff wait.Backoff) (*apiv1.Service, error) {
	var service *apiv1.Service = nil
	var err error
	isError := func(err error) bool {
		return err != nil
	}

	_retryFn := func() (*apiv1.Service, error) {
		cc.KubectlExec("get pods -o wide")
		return cc.VanClient.KubeClient.CoreV1().Services(cc.Namespace).Get(name, metav1.GetOptions{})
	}

	if retryFn == nil {
		retryFn = _retryFn
	}

	return service, retry.OnError(backoff, isError, func() error {
		service, err = retryFn()
		return err
	})
}

func (cc *ClusterContext) DumpTestInfo(dirName string) {
	if !strings.HasPrefix(dirName, "tmp/") {
		dirName = fmt.Sprintf("tmp/%s", dirName)
	}
	f, err := os.Stat(dirName)
	if f != nil && !f.IsDir() {
		log.Printf("unable to dump test info: %s is not a directory", dirName)
		return
	} else if f == nil {
		if err := os.MkdirAll(dirName, 0755); err != nil {
			log.Printf("unable to dump test info: %v", err)
			return
		}
	}
	log.Printf("===> Dumping test information for: %s", cc.Namespace)
	tarBall, err := cc.VanClient.SkupperDump(context.Background(), fmt.Sprintf("%s/%s.tar.gz", dirName, cc.Namespace), cc.VanClient.GetVersion("service-controller", "service-controller"), "", "")
	if err == nil {
		absPath, _ := filepath.Abs(tarBall)
		log.Printf("Saved: %s", absPath)
	} else {
		log.Printf("error dumping test info: %v", err)
	}
	out, err := cc.KubectlExec("get -o wide job,pod,service")
	if err != nil {
		log.Printf("failed getting kube info: %v", err)
	}
	log.Printf("kube info: \n%v", string(out))
}
