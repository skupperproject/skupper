//go:build policy
// +build policy

package hello_policy

import (
	"log"
	"regexp"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	skupperv1 "github.com/skupperproject/skupper/pkg/apis/skupper/v1alpha1"
	clientv1 "github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/typed/skupper/v1alpha1"
	"github.com/skupperproject/skupper/test/utils/base"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Adds the CRD to the cluster
//
// It uses kubectl apply, so it is safe to apply on a cluster that already has
// the CRD installed.
func applyCrd(cluster *base.ClusterContext) (err error) {
	var out []byte
	log.Printf("Adding CRD into the %v cluster", cluster.KubeConfig)
	out, err = cluster.KubectlExec("apply -f ../../../../../api/types/crds/skupper_cluster_policy_crd.yaml")
	if err != nil {
		log.Printf("CRD applying failed: %v", err)
		log.Print("Output:\n", string(out))
		return
	}
	return
}

// Remove the CRD from the cluster
//
// No-op if the CRD was not installed in the first place
func removeCrd(cluster *base.ClusterContext) (changed bool, err error) {
	changed = true

	log.Printf("Removing CRD from the cluster %v", cluster.KubeConfig)

	installed := client.NewClusterPolicyValidator(cluster.VanClient).CrdDefined()

	if !installed {
		changed = false
		log.Print("CRD was not present, so not changing anything")
		return
	}

	if _, err = cluster.KubectlExec("delete crd skupperclusterpolicies.skupper.io"); err != nil {
		log.Printf("Removal of CRD failed: %v", err)
	}
	return
}

// Remove the cluster role, but do not fail if it is not there
func removeClusterRole(cluster *base.ClusterContext) (changed bool, err error) {
	log.Printf("Removing cluster role %v from the CRD definition", types.ControllerServiceAccountName)

	// Is it there?
	role, err := cluster.VanClient.KubeClient.RbacV1().ClusterRoles().Get(types.ControllerServiceAccountName, metav1.GetOptions{})
	if role == nil && err != nil {
		log.Print("The role did not exist on the cluster; skipping removal")
		err = nil
		return
	}
	err = cluster.VanClient.KubeClient.RbacV1().ClusterRoles().Delete(types.ControllerServiceAccountName, nil)
	if err == nil {
		changed = true
	}
	return
}

// Removes all policies from the cluster.
//
// If policies are provided, only those will be removed, instead
func removePolicies(cluster *base.ClusterContext, policies ...string) (err error) {

	log.Print("Removing policies")

	var list *skupperv1.SkupperClusterPolicyList

	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}

	if len(policies) == 0 {
		policies = []string{}
		// We're listing and removing everything
		list, err = listPolicies(cluster)
		if err != nil {
			return
		}
		for _, item := range list.Items {
			policies = append(policies, item.Name)
		}
	}

	for _, item := range policies {
		log.Printf("- %v", item)
		item_err := skupperCli.SkupperClusterPolicies().Delete(item, &metav1.DeleteOptions{})
		if item_err != nil {
			log.Printf("  removal failed: %v", item_err)
			if err == nil {
				// We'll return the first error from the list, but keep trying the others
				err = item_err
			}
		}
	}

	return
}

// A wrapper around clientv1.SkupperV1alpha1Client.SkupperClusterPolicies().List()
//
// This function will always return an empty list for clusters without a CRD
// installed
func listPolicies(cluster *base.ClusterContext) (list *skupperv1.SkupperClusterPolicyList, err error) {

	list = &skupperv1.SkupperClusterPolicyList{}

	installed := client.NewClusterPolicyValidator(cluster.VanClient).CrdDefined()

	if !installed {
		log.Print("The CRD is not installed, so considering the policy list as empty")
		return
	}

	skupperCli, err := clientv1.NewForConfig(cluster.VanClient.RestConfig)
	if err != nil {
		return
	}

	list, err = skupperCli.SkupperClusterPolicies().List(metav1.ListOptions{})
	if err != nil {
		log.Print("Failed listing policies")
		return
	}

	return
}

// Removes all policies in the cluster, except for those that match any
// of the provided regexp patterns.
func removeAllPoliciesExcept(cluster *base.ClusterContext, exceptions []regexp.Regexp) (err error) {

	policyList := []string{}

	list, err := listPolicies(cluster)
	if err != nil {
		return
	}

	for _, item := range list.Items {
		var matched bool
		for _, re := range exceptions {
			if re.MatchString(item.Name) {
				matched = true
				break
			}
		}
		if !matched {
			policyList = append(policyList, item.Name)
		}
	}

	if len(policyList) == 0 {
		return
	}

	err = removePolicies(cluster, policyList...)

	return

}

// Apply a SkupperClusterPolicySpec with the given name on the requested
// cluster.
//
// If a policy with the same name already exists on the cluster, it will be
// updated with the new specification.  Otherwise, it will be created anew.
func applyPolicy(name string, spec skupperv1.SkupperClusterPolicySpec, cluster *base.ClusterContext) (err error) {

	log.Printf("Applying policy %v (%+v)...", name, spec)
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
		log.Printf("... as a new policy")
		_, err = skupperCli.SkupperClusterPolicies().Create(&policy)
		if err != nil {
			return err
		}
	} else {
		log.Printf("... as an update to an existing policy")
		policy.ObjectMeta.ResourceVersion = existing.ObjectMeta.ResourceVersion
		_, err := skupperCli.SkupperClusterPolicies().Update(&policy)
		if err != nil {
			return err
		}
	}

	return
}
