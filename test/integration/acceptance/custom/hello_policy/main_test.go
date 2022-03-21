package hello_policy

import (
	"os"
	"strings"
	"testing"

	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	clientv1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Adds the CRD to the cluster
func applyCrd(t *testing.T, cluster *base.ClusterContext) (err error) {
	t.Logf("Adding CRD into the %v cluster", cluster.KubeConfig)
	if err = k8s.CreateResourcesFromYAML(cluster.VanClient, "../../../../../api/types/crds/skupper_cluster_policy_crd.yaml"); err != nil {
		t.Fatalf("Adding CRD failed: %v", err)
	}
	return
}

// Remove the CRD from the cluster
func removeCrd(t *testing.T, cluster *base.ClusterContext) (changed bool, err error) {
	changed = true
	var out []byte

	t.Logf("Removing CRD from the cluster %v", cluster.KubeConfig)

	// TODO: replace this by some kube API
	if out, err = cluster.KubectlExec("get crd skupperclusterpolicies.skupper.io"); err != nil {
		if strings.Contains(
			string(out),
			`Error from server (NotFound): customresourcedefinitions.apiextensions.k8s.io "skupperclusterpolicies.skupper.io" not found`) {
			changed = false
			err = nil
			t.Log("CRD was not present, so not changing anything")
			return
		} else {
			t.Logf("Output:\n%v", out)
			t.Fatalf("Failed checking CRD: %v", err)
			return
		}
	}

	if _, err := cluster.KubectlExec("delete crd skupperclusterpolicies.skupper.io"); err != nil {
		t.Fatalf("Removal of CRD failed: %v", err)
	}
	return
}

// Apply a SkupperClusterPolicySpec with the given name on the
// requested cluster
func applyPolicy(t *testing.T, name string, spec skupperv1.SkupperClusterPolicySpec, cluster *base.ClusterContext) (err error) {

	t.Logf("Applying policy %v", spec)
	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}
	var policy = skupperv1.SkupperClusterPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SkupperClusterPolicy",
			APIVersion: "skupper.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: spec,
	}

	existing, err := skupperCli.SkupperClusterPolicies().Get(name, metav1.GetOptions{})
	if err != nil {
		_, err = skupperCli.SkupperClusterPolicies().Create(&policy)
		if err != nil {
			return err
		}
	} else {
		policy.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
		_, err := skupperCli.SkupperClusterPolicies().Update(&policy)
		if err != nil {
			return err
		}
	}

	return
}

// Helper to deploy an application on Kubernetes and wait for it
type KubeDeploy struct {
	image  string
	labels map[string]string
	name   string
}

// Actual deployment, according to struct's specification, on the
// given context
func (kd KubeDeploy) deploy(ctx *base.ClusterContext) (err error) {
	frontend, _ := k8s.NewDeployment(kd.name, ctx.Namespace, k8s.DeploymentOpts{
		Image:         kd.image,
		Labels:        kd.labels,
		RestartPolicy: corev1.RestartPolicyAlways,
	})
	// Creating deployments
	if _, err = ctx.VanClient.KubeClient.AppsV1().Deployments(ctx.Namespace).Create(frontend); err != nil {
		return
	}
	// Waiting for deployments to be ready
	if _, err := kube.WaitDeploymentReady(kd.name, ctx.Namespace, ctx.VanClient.KubeClient, constants.ImagePullingAndResourceCreationTimeout, constants.DefaultTick); err != nil {
		return err
	}
	return
}

// Deploys the front-end side of quay.io/skupper/hello-world-*
func deployFrontend(ctx *base.ClusterContext) (err error) {
	err = KubeDeploy{
		image:  "quay.io/skupper/hello-world-frontend",
		labels: map[string]string{"app": "hello-world-frontend"},
		name:   "hello-world-frontend",
	}.deploy(ctx)
	return
}

//
// Deploys the backend-end side of quay.io/skupper/hello-world-*
func deployBackend(ctx *base.ClusterContext) (err error) {
	err = KubeDeploy{
		image:  "quay.io/skupper/hello-world-backend",
		labels: map[string]string{"app": "hello-world-backend"},
		name:   "hello-world-backend",
	}.deploy(ctx)
	return
}

// deployResources Deploys the hello-world-frontend and hello-world-backend
// pods and validate they are available
func deployResources(pub *base.ClusterContext, prv *base.ClusterContext) error {
	deployFrontend(pub)
	deployBackend(prv)

	return nil
}

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

func TestPolicies(t *testing.T) {
	// Because the policy changes are cluster-wise, we need to run
	// all tests in serial

	t.Run("testNamespace", testNamespace)

}
