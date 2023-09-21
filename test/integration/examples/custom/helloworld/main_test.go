package helloworld

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/skupperproject/skupper/test/utils/base"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestMain initializes flag parsing
func TestMain(m *testing.M) {
	base.ParseFlags()
	os.Exit(m.Run())
}

// Receives a default value, that will be overridden when running on Openshift.
// Returns the value to be used.
//
// OpenShift requires RunAsUser to be configured according to annotations
// present on the namespace, lest it will fail with SCC errors.
//
// If any errors found while tring to determine the correct user, this function
// will ignore them and simply use the default value.
func getRunAsUserOrDefault(runAsUser string, cctx *base.ClusterContext) string {
	// OpenShift requires container user IDs to exist within a range; we try to satisfy it here.
	namespace, err := cctx.VanClient.KubeClient.CoreV1().Namespaces().Get(context.Background(), cctx.Namespace, metav1.GetOptions{})
	if err != nil {
		log.Printf("Unable to get namespace %q; using pre-defined runAsUser value %v", cctx.Namespace, runAsUser)
	} else {
		ns_annotations := namespace.GetAnnotations()
		if users, ok := ns_annotations["openshift.io/sa.scc.uid-range"]; ok {
			log.Printf("OpenShift UID range annotation found: %q", users)
			// format is like 1000860000/10000, where the first number is the
			// range start, and the second its length
			split_users := strings.Split(users, "/")
			if split_users[0] != "" {
				if _, err := strconv.Atoi(split_users[0]); err == nil {
					runAsUser = split_users[0]
				} else {
					log.Printf("Failed to parse openshift uid-range annotation: using default value %v", runAsUser)
				}
			} else {
				log.Printf("openshift uid-range annotation is empty, which is unexpected: using default value %v", runAsUser)
			}
		} // if annotation not found, we're not on Openshift, and we can use the default value.
	}
	return runAsUser
}
