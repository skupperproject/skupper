package cli

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/test/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

var (
	expectUpdatedSecrets    = []string{types.SiteCaSecret, types.SiteServerSecret, types.ClaimsServerSecret}
	expectUpdatedComponents = []string{types.RouterComponent, types.ControllerComponentName}
)

const (
	// timeout defines how long to wait for revoke-access to complete
	timeout = time.Minute
)

// RevokeAccessTester allows running and validating `skupper revoke-access`.
type RevokeAccessTester struct {
	ExpectClaimRecordsDeleted bool
	secretInformer            cache.SharedIndexInformer
	podInformer               cache.SharedIndexInformer
	claimRecordsDeleted       bool
}

func (d *RevokeAccessTester) Command(cluster *base.ClusterContext) []string {
	args := SkupperCommonOptions(cluster)
	args = append(args, "revoke-access")
	return args
}

func (d *RevokeAccessTester) Run(cluster *base.ClusterContext) (stdout string, stderr string, err error) {

	//
	// Creating informers to monitor secrets and pods (before revoke-access is issued):
	// - Removal of those labeled as 'skupper.io/type=token-claim-record'
	// - Updates to: skupper-site-ca secret, skupper-site-server, skupper-claims-server
	// - router and service-controller pods restarted (after deployment updated)
	//
	stopCh := make(chan struct{})
	defer close(stopCh)
	doneCh := d.initializeInformers(cluster, stopCh)

	// Execute revoke-access command
	stdout, stderr, err = RunSkupperCli(d.Command(cluster))
	if err != nil {
		return
	}

	//
	// output is currently empty so we must validate if secrets have been recycled
	//
	log.Printf("Validating 'skupper revoke-access'")
	if stdout != "" {
		err = fmt.Errorf("expected an empty output - found: %s", stdout)
		return
	}

	//
	// Waiting for secret updates to complete or timeout
	//
	log.Printf("validating secrets deleted and updated")
	timeoutCh := time.After(timeout)
	select {
	case <-doneCh:
		log.Println("access has been revoked successfully")
	case <-timeoutCh:
		err = fmt.Errorf("timed out waiting on CA regeneration to complete")
	}

	return
}

// initializeInformers Defines secret and pod informers to validate that
// expected secrets are updated or deleted and that the router and service
// controller pods are restarted.
func (d *RevokeAccessTester) initializeInformers(cluster *base.ClusterContext, stop <-chan struct{}) chan struct{} {
	var updatedSecrets []string
	var updatedComponents []string
	done := make(chan struct{})

	// Validate all expected changes are in place
	validateDone := func() {
		claimRecordsDeleted := !d.ExpectClaimRecordsDeleted || d.claimRecordsDeleted
		secretsUpdated := utils.AllStrIn(expectUpdatedSecrets, updatedSecrets...)
		componentsRecycled := utils.AllStrIn(expectUpdatedComponents, updatedComponents...)
		log.Println("claim records deleted =", claimRecordsDeleted)
		log.Println("updated secrets       =", secretsUpdated)
		log.Println("updated components    =", componentsRecycled)
		if claimRecordsDeleted && secretsUpdated && componentsRecycled {
			close(done)
		}
	}

	// Secret informer
	factory := informers.NewSharedInformerFactory(cluster.VanClient.KubeClient, 0)
	d.secretInformer = factory.Core().V1().Secrets().Informer()
	d.secretInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			// Watches for secrets whose data section has been updated
			oldSecret := oldObj.(*v1.Secret)
			newSecret := newObj.(*v1.Secret)
			log.Printf("secret has been updated: %s", newSecret.Name)
			if !reflect.DeepEqual(oldSecret.Data, newSecret.Data) {
				updatedSecrets = append(updatedSecrets, newSecret.Name)
			} else {
				log.Println("DATA section NOT updated - ignoring secret update")
			}
			validateDone()
		}, DeleteFunc: func(obj interface{}) {
			// Watches for deleted token claim records
			secret := obj.(*v1.Secret)
			log.Printf("secret has been deleted: %s", secret.Name)
			if secret.ObjectMeta.Labels != nil {
				if skupperType, ok := secret.ObjectMeta.Labels[types.SkupperTypeQualifier]; ok && skupperType == types.TypeClaimRecord {
					d.claimRecordsDeleted = true
					validateDone()
				}
			}
		},
	})

	// Watch for new router and service-controller pods
	d.podInformer = factory.Core().V1().Pods().Informer()
	d.podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			if pod.Namespace != cluster.Namespace || !strings.HasPrefix(pod.Name, "skupper-") || pod.Status.Phase != v1.PodRunning {
				// log.Printf("ignoring pod status change: %s.%s [%s]", pod.Namespace, pod.Name, pod.Status.Phase)
				return
			}
			if component, ok := pod.Labels[types.ComponentAnnotation]; ok {
				updatedComponents = append(updatedComponents, component)
				log.Printf("component has been recycled: %s", component)
				validateDone()
			}
		},
	})

	// Starting informers
	go d.secretInformer.Run(stop)
	go d.podInformer.Run(stop)

	return done
}
