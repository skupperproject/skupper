package arch

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/skupperproject/skupper/test/utils/base"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// If the target clusters are found to contain any non-amd64 nodes, return true.
//
// Notice that only the cluster is checked to be amd64; if the VM needs checked,
// use build flags or some other technique.
//
// Usage: check first skip; only check err if skip is false.  If skip is true, error
// will be non-nil, with information on why skipping
//
// TODO: make it more granular, allow for hibrid clusters?
// TODO: allow for list of accepted archs?
func Check(clusters ...*base.ClusterContext) (err error, skip bool) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	for _, c := range clusters {
		list, err := c.VanClient.KubeClient.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return err, false
		}
		for _, node := range list.Items {
			arch := node.Labels["beta.kubernetes.io/arch"]
			if arch != "amd64" {
				return fmt.Errorf(
					"at least one cluster node is not amd64 -- skipping (%s at %s is %q)",
					node.Name,
					c.VanClient.RestConfig.Host,
					arch,
				), true
			}
		}
	}
	return nil, false
}

// Calls arch.Check, and skip the test as needed
func Skip(t *testing.T, clusters ...*base.ClusterContext) error {
	err, skip := Check(clusters...)
	if skip {
		t.Skipf("%v", err)
	}
	return err
}

// CheckOnlyS390x skips ONLY if the architecture is s390x
func CheckOnlyS390x(clusters ...*base.ClusterContext) (err error, skip bool) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()

	for _, c := range clusters {
		list, err := c.VanClient.KubeClient.CoreV1().Nodes().List(ctx, v1.ListOptions{})
		if err != nil {
			return err, false
		}
		for _, node := range list.Items {
			arch := node.Labels["beta.kubernetes.io/arch"]
			if arch == "s390x" {
				return fmt.Errorf(
					"at least one cluster node is s390x -- skipping (%s at %s is %q)",
					node.Name,
					c.VanClient.RestConfig.Host,
					arch,
				), true
			}
		}
	}
	return nil, false
}

// SkipOnlyS390x skips the test only for s390x clusters
func SkipOnlyS390x(t *testing.T, clusters ...*base.ClusterContext) error {
	err, skip := CheckOnlyS390x(clusters...)
	if skip {
		t.Skipf("%v", err)
	}
	return err
}
