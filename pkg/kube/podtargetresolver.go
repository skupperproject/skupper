/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kube

import (
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/pkg/event"
)

const (
	BridgeTargetEvent string = "BridgeTargetEvent"
)

type PodTargetResolver struct {
	address         string
	informer        cache.SharedIndexInformer
	stopper         chan struct{}
	skipStatusCheck bool
}

func (o *PodTargetResolver) Start() error {
	go o.informer.Run(o.stopper)
	if ok := cache.WaitForCacheSync(o.stopper, o.informer.HasSynced); !ok {
		return fmt.Errorf("Failed to wait for service targetcache to sync")
	}
	return nil
}

func (o *PodTargetResolver) Close() {
	close(o.stopper)
}

func (o *PodTargetResolver) List() []string {
	pods := o.informer.GetStore().List()
	var targets []string
	var isPodEligible bool

	for _, p := range pods {
		pod := p.(*corev1.Pod)
		isPodEligible = o.skipStatusCheck || (IsPodRunning(pod) && IsPodReady(pod))
		if isPodEligible && pod.DeletionTimestamp == nil {
			event.Recordf(BridgeTargetEvent, "Adding pod for %s: %s", o.address, pod.ObjectMeta.Name)
			targets = append(targets, pod.Status.PodIP)
		} else {
			event.Recordf(BridgeTargetEvent, "Pod for %s not ready/running: %s", o.address, pod.ObjectMeta.Name)
		}
	}
	return targets
}

func (o *PodTargetResolver) HasTarget() bool {
	pods := o.informer.GetStore().List()
	for _, p := range pods {
		pod := p.(*corev1.Pod)
		if pod.DeletionTimestamp == nil {
			return true
		}
	}
	return false
}

func (o *PodTargetResolver) AddEventHandler(handler *cache.ResourceEventHandlerFuncs) {
	o.informer.AddEventHandler(handler)
}

func NewPodTargetResolver(cli kubernetes.Interface, namespace string, address string, selector string, skipPodStatus bool) *PodTargetResolver {
	return &PodTargetResolver{
		address: address,
		informer: corev1informer.NewFilteredPodInformer(
			cli,
			namespace,
			time.Second*30,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			internalinterfaces.TweakListOptionsFunc(func(options *metav1.ListOptions) {
				options.LabelSelector = selector
			})),
		stopper:         make(chan struct{}),
		skipStatusCheck: skipPodStatus,
	}
}
