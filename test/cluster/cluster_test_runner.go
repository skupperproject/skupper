package cluster

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"testing"
	"time"

	"gotest.tools/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	batchv1 "k8s.io/api/batch/v1"

	vanClient "github.com/skupperproject/skupper/client"
)

type ClusterTestRunnerInterface interface {
	Build(t *testing.T, ns_suffix string) //is this interface used?
	Run()
}

type ClusterTestRunnerBase struct {
	Pub1Cluster  *ClusterContext
	Pub2Cluster  *ClusterContext
	Priv1Cluster *ClusterContext
	Priv2Cluster *ClusterContext
	T            *testing.T
}

func (r *ClusterTestRunnerBase) Build(t *testing.T, ns_suffix string) {

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homedir, err := os.UserHomeDir()
		assert.Assert(t, err)
		kubeconfig = path.Join(homedir, ".kube/config")
	}

	//TODO assign here uniq, publicX and privateX namespaces instead of
	//generic ones
	r.Pub1Cluster = BuildClusterContext(t, "public1-"+ns_suffix, kubeconfig, vanClient.NewClient)
	r.Pub2Cluster = BuildClusterContext(t, "public2-"+ns_suffix, kubeconfig, vanClient.NewClient)
	r.Priv1Cluster = BuildClusterContext(t, "private1-"+ns_suffix, kubeconfig, vanClient.NewClient)
	r.Priv2Cluster = BuildClusterContext(t, "private2-"+ns_suffix, kubeconfig, vanClient.NewClient)
	r.T = t
}

type ClusterContext struct {
	NamespacePrefix   string
	CurrentNamespace  string
	Namespaces        []string
	ClusterConfigFile string
	VanClient         *vanClient.VanClient
	t                 *testing.T
}

func BuildClusterContext(t *testing.T, namespacePrefix string, configFile string, newVanClient func(namespace, context, kubeConfigPath string) (*vanClient.VanClient, error)) *ClusterContext {
	var err error
	cc := &ClusterContext{}
	cc.t = t
	cc.NamespacePrefix = namespacePrefix
	cc.ClusterConfigFile = configFile
	cc.VanClient, err = newVanClient("", "", cc.ClusterConfigFile)
	assert.Check(cc.t, err)
	return cc
}

func _exec(command string, wait bool) *exec.Cmd {
	var output []byte
	var err error
	fmt.Println(command)
	cmd := exec.Command("sh", "-c", command)
	if wait {
		output, err = cmd.CombinedOutput()
		fmt.Println(string(output))
		if err != nil {
			panic(err)
		}
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
	}
	return cmd
}

func (cc *ClusterContext) exec(main_command string, sub_command string, wait bool) *exec.Cmd {
	return _exec("KUBECONFIG="+cc.ClusterConfigFile+" "+main_command+" "+cc.CurrentNamespace+" "+sub_command, wait)
}

//TODO remove this
func (cc *ClusterContext) SkupperExec(command string) *exec.Cmd {
	return cc.exec("./skupper -n ", command, true)
}

func (cc *ClusterContext) _kubectl_exec(command string, wait bool) *exec.Cmd {
	return cc.exec("kubectl -n ", command, wait)
}

//TODO return error instead of panic in case of exit code != 0
func (cc *ClusterContext) KubectlExec(command string) *exec.Cmd {
	return cc._kubectl_exec(command, true)
}

func (cc *ClusterContext) KubectlExecAsync(command string) *exec.Cmd {
	return cc._kubectl_exec(command, false)
}

func (cc *ClusterContext) getNextNamespace() string {
	return cc.NamespacePrefix + "-" + strconv.Itoa((len(cc.Namespaces) + 1))
}

func (cc *ClusterContext) moveToNextNamespace() {
	next := cc.getNextNamespace()
	cc.Namespaces = append(cc.Namespaces, next)
	cc.CurrentNamespace = next
	cc.VanClient.Namespace = cc.CurrentNamespace
}

func (cc *ClusterContext) CreateNamespace() error {
	ns := cc.getNextNamespace()
	NsSpec := &apiv1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}
	_, err := cc.VanClient.KubeClient.CoreV1().Namespaces().Create(NsSpec)
	if err != nil {
		return err
	}
	cc.moveToNextNamespace()
	return nil
}

func (cc *ClusterContext) deleteNamespace(ns string) {
	//remove from the list
	err := cc.VanClient.KubeClient.CoreV1().Namespaces().Delete(ns, &metav1.DeleteOptions{})
	assert.Check(cc.t, err)
}

func (cc *ClusterContext) DeleteNamespaces() {
	for _, ns := range cc.Namespaces {
		cc.deleteNamespace(ns)
	}
	cc.Namespaces = cc.Namespaces[:0]
	cc.CurrentNamespace = ""
}

func (cc *ClusterContext) DeleteNamespace() {
	assert.Equal(cc.t, 1, len(cc.Namespaces), "Use DeleteNamespaces")
	cc.DeleteNamespaces()
}

func (cc *ClusterContext) GetService(name string, timeout_S time.Duration) *apiv1.Service {
	timeout := time.After(timeout_S * time.Second)
	tick := time.Tick(3 * time.Second)
	for {
		select {
		case <-timeout:
			log.Panicln("Timed Out Waiting for service.")
		case <-tick:
			service, err := cc.VanClient.KubeClient.CoreV1().Services(cc.CurrentNamespace).Get(name, metav1.GetOptions{})
			if err == nil {
				return service
			} else {
				log.Println("Service not ready yet, current pods state: ")
				cc.KubectlExec("get pods -o wide") //TODO use clientset
			}

		}
	}
}

func getTestImage() string {
	testImage := os.Getenv("TEST_IMAGE")
	if testImage == "" {
		testImage = "quay.io/skupper/skupper-tests"
	}
	return testImage
}

func int32Ptr(i int32) *int32 { return &i }

func (cc *ClusterContext) CreateTestJob(name string, command []string) (*batchv1.Job, error) {

	namespace := cc.CurrentNamespace
	testImage := getTestImage()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(1), //one retry (no retries)
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    name,
							Image:   testImage,
							Command: command,
							Env: []apiv1.EnvVar{
								{Name: "JOB", Value: name},
							},
							ImagePullPolicy: apiv1.PullIfNotPresent,
						},
					},
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}

	jobsClient := cc.VanClient.KubeClient.BatchV1().Jobs(namespace)

	job, err := jobsClient.Create(job)

	if err != nil {
		return nil, err
	}
	return job, nil
}

//TODO evaluate modifying this implementation to use informers instead of
//pooling.
func (cc *ClusterContext) WaitForJob(jobName string, timeout_S time.Duration) (*batchv1.Job, error) {

	jobsClient := cc.VanClient.KubeClient.BatchV1().Jobs(cc.CurrentNamespace)

	defer cc.KubectlExec("logs job/" + jobName)

	cmd := fmt.Sprintf("get job/%s -o wide", jobName)
	for {
		select {
		case <-time.After(timeout_S * time.Second):
			return nil, fmt.Errorf("Timeout: Job is still active")
		case <-time.Tick(5 * time.Second):
			job, _ := jobsClient.Get(jobName, metav1.GetOptions{})
			if job.Status.Active > 0 {
				fmt.Println("Job is still active")
				cc.KubectlExec(cmd)
			} else {
				if job.Status.Succeeded > 0 {
					fmt.Println("Job Successful!!")
					cc.KubectlExec(cmd)
					return job, nil
				}
				fmt.Printf("Job failed!!!??, status = %v\n", job.Status)
				cc.KubectlExec(cmd)
				return job, nil
			}
		}
	}

}
