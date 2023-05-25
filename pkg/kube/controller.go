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
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	corev1informer "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	routev1client "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"

	"github.com/skupperproject/skupper/pkg/event"
)

type ResourceChange struct {
	Handler ResourceChangeHandler
	Key     string
}

type ResourceChangeHandler interface {
	Handle(event ResourceChange) error
	Describe(event ResourceChange) string
}

func ListByName(name string) internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=" + name
	}
}

func ListByLabelSelector(selector string) internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = selector
	}
}

type Controller struct {
	eventKey        string
	errorKey        string
	client          kubernetes.Interface
	routeClient     *routev1client.RouteV1Client
	dynamicClient   dynamic.Interface
	discoveryClient *discovery.DiscoveryClient
	queue           workqueue.RateLimitingInterface
	resync          time.Duration
}

func NewController(name string, clients Clients) *Controller {
	return &Controller{
		eventKey:        name + "Event",
		errorKey:        name + "Error",
		client:          clients.GetKubeClient(),
		routeClient:     clients.GetRouteClient(),
		discoveryClient: clients.GetDiscoveryClient(),
		dynamicClient:   clients.GetDynamicClient(),
		queue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		resync:          time.Minute*5,
	}
}
func (c *Controller)  GetKubeClient() kubernetes.Interface {
	return c.client
}

func (c *Controller)  GetDynamicClient() dynamic.Interface {
	return c.dynamicClient
}

func (c *Controller)  GetDiscoveryClient() *discovery.DiscoveryClient {
	return c.discoveryClient
}

func (c *Controller)  GetRouteClient() *routev1client.RouteV1Client {
	return c.routeClient
}

func (c *Controller) NewWatchers(client kubernetes.Interface) Watchers {
	return &Controller{
		eventKey: c.eventKey,
		errorKey: c.errorKey,
		client: client,
		queue: c.queue,
	}
}

func (c *Controller) AddEvent(o interface{}) {
	c.queue.Add(o)
}

func (c *Controller) Start(stopCh <-chan struct{}) {
	go wait.Until(c.run, time.Second, stopCh)
}

func (c *Controller) run() {
	for c.process() {
	}
}

func (c *Controller) process() bool {
	obj, shutdown := c.queue.Get()

	if shutdown {
		return false
	}

	retry := false
	defer c.queue.Done(obj)
	if evt, ok := obj.(ResourceChange); ok {
		event.Recordf(c.eventKey, evt.Handler.Describe(evt))
		err := evt.Handler.Handle(evt)
		if err != nil {
			retry = true
			event.Recordf(c.errorKey, "Error while handling %s: %s", evt.Handler.Describe(evt), err)
			log.Printf("[%s] Error while handling %s: %s", c.errorKey, evt.Handler.Describe(evt), err)
		}
	} else {
		event.Recordf(c.errorKey, "Invalid object on event queue: %#v", obj)
		log.Printf("Invalid object on event queue for %q: %#v", c.errorKey, obj)
	}
	c.queue.Forget(obj)

	if retry && c.queue.NumRequeues(obj) < 5 {
		c.queue.AddRateLimited(obj)
	}

	return true
}


func (c *Controller) Stop() {
	c.queue.ShutDown()
}

func (c *Controller) Empty() bool {
	return c.queue.Len() == 0
}

func (c *Controller) newEventHandler(handler ResourceChangeHandler) *cache.ResourceEventHandlerFuncs {
	evt := ResourceChange {
		Handler: handler,
	}
	return &cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.queue.Add(evt)
			}
		},
		UpdateFunc: func(old, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.queue.Add(evt)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err != nil {
				utilruntime.HandleError(err)
			} else {
				evt.Key = key
				c.queue.Add(evt)
			}
		},
	}
}

type Watchers interface{
	WatchConfigMaps(options internalinterfaces.TweakListOptionsFunc, namespace string, handler ConfigMapHandler) *ConfigMapWatcher
	WatchSecrets(options internalinterfaces.TweakListOptionsFunc, namespace string, handler SecretHandler) *SecretWatcher
}

func (c *Controller) WatchConfigMaps(options internalinterfaces.TweakListOptionsFunc, namespace string, handler ConfigMapHandler) *ConfigMapWatcher {
	watcher := &ConfigMapWatcher{
		handler:   handler,
		informer:  corev1informer.NewFilteredConfigMapInformer(
			c.client,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options),
		namespace: namespace,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type ConfigMapHandler func(string, *corev1.ConfigMap) error

type ConfigMapWatcher struct {
	handler   ConfigMapHandler
	informer  cache.SharedIndexInformer
	namespace string
}

func (w *ConfigMapWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *ConfigMapWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *ConfigMapWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("ConfigMap %s", event.Key)
}

func (w *ConfigMapWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *ConfigMapWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *ConfigMapWatcher) Get(key string) (*corev1.ConfigMap, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.ConfigMap), nil
}

func (w *ConfigMapWatcher) List() []*corev1.ConfigMap {
	list := w.informer.GetStore().List()
	results := []*corev1.ConfigMap{}
	for _, o := range list {
		results = append(results, o.(*corev1.ConfigMap))
	}
	return results
}

func (c *Controller) WatchSecrets(options internalinterfaces.TweakListOptionsFunc, namespace string, handler SecretHandler) *SecretWatcher {
	watcher := &SecretWatcher{
		handler:   handler,
		informer:  corev1informer.NewFilteredSecretInformer(
			c.client,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options),
		namespace: namespace,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type SecretHandler func(string, *corev1.Secret) error

type SecretWatcher struct {
	handler   SecretHandler
	informer  cache.SharedIndexInformer
	namespace string
}

func (w *SecretWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *SecretWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *SecretWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Secret %s", event.Key)
}

func (w *SecretWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *SecretWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *SecretWatcher) Get(key string) (*corev1.Secret, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.Secret), nil
}

func (w *SecretWatcher) List() []*corev1.Secret {
	list := w.informer.GetStore().List()
	results := []*corev1.Secret{}
	for _, o := range list {
		results = append(results, o.(*corev1.Secret))
	}
	return results
}

type ServiceHandler func(string, *corev1.Service) error

func (c *Controller) WatchServices(options internalinterfaces.TweakListOptionsFunc, namespace string, handler ServiceHandler) *ServiceWatcher {
	watcher := &ServiceWatcher{
		client:  c.client,
		handler: handler,
		informer: corev1informer.NewFilteredServiceInformer(
			c.client,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options,
		),
		namespace: namespace,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type ServiceWatcher struct {
	client    kubernetes.Interface
	handler   ServiceHandler
	informer  cache.SharedIndexInformer
	namespace string
}

func (w *ServiceWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *ServiceWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Service %s", event.Key)
}

func (w *ServiceWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *ServiceWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *ServiceWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *ServiceWatcher) Get(key string) (*corev1.Service, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.Service), nil
}

func (w *ServiceWatcher) CreateService(svc *corev1.Service) (*corev1.Service, error) {
	return w.client.CoreV1().Services(w.namespace).Create(context.TODO(), svc, metav1.CreateOptions{})
}

func (w *ServiceWatcher) UpdateService(svc *corev1.Service) (*corev1.Service, error) {
	return w.client.CoreV1().Services(w.namespace).Update(context.TODO(), svc, metav1.UpdateOptions{})
}

func (w *ServiceWatcher) GetService(name string) (*corev1.Service, error) {
	return w.Get(name)
}

type PodHandler func(string, *corev1.Pod) error

func (c *Controller) WatchAllPods(namespace string, handler PodHandler) *PodWatcher {
	watcher := &PodWatcher{
		handler: handler,
		informer: corev1informer.NewPodInformer(
			c.client,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
		),
		namespace: namespace,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

func (c *Controller) WatchPods(selector string, namespace string, handler PodHandler) *PodWatcher {
	log.Printf("WatchPods(%s, %s)", selector, namespace)
	options := ListByLabelSelector(selector)
	watcher := &PodWatcher{
		handler: handler,
		informer: corev1informer.NewFilteredPodInformer(
			c.client,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options,
		),
		namespace: namespace,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type PodWatcher struct {
	handler   PodHandler
	informer  cache.SharedIndexInformer
	namespace string
}

func (w *PodWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *PodWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Pod %s", event.Key)
}

func (w *PodWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *PodWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *PodWatcher) Get(key string) (*corev1.Pod, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.Pod), nil
}

func (w *PodWatcher) List() []*corev1.Pod {
	list := w.informer.GetStore().List()
	pods := []*corev1.Pod{}
	for _, p := range list {
		pods = append(pods, p.(*corev1.Pod))
	}
	return pods
}

func (c *Controller) WatchDynamic(resource schema.GroupVersionResource, options dynamicinformer.TweakListOptionsFunc, namespace string, handler DynamicHandler) *DynamicWatcher {
	watcher := &DynamicWatcher{
		handler:   handler,
		informer:  dynamicinformer.NewFilteredDynamicInformer(
			c.dynamicClient,
			resource,
			namespace,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options).Informer(),
		namespace: namespace,
		resource: resource,
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type DynamicHandler func(string, *unstructured.Unstructured) error

type DynamicWatcher struct {
	handler   DynamicHandler
	informer  cache.SharedIndexInformer
	namespace string
	resource  schema.GroupVersionResource
}

func (w *DynamicWatcher) Resource() schema.GroupVersionResource {
	return w.resource
}

func (w *DynamicWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *DynamicWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *DynamicWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Dynamic %s", event.Key)
}

func (w *DynamicWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *DynamicWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *DynamicWatcher) Get(key string) (*unstructured.Unstructured, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*unstructured.Unstructured), nil
}

func (w *DynamicWatcher) List() []*unstructured.Unstructured {
	list := w.informer.GetStore().List()
	results := []*unstructured.Unstructured{}
	for _, o := range list {
		results = append(results, o.(*unstructured.Unstructured))
	}
	return results
}

type Callback func(context string) error

type CallbackHandler struct {
	callback   Callback
	context    string
}

func (c *CallbackHandler) Handle(event ResourceChange) error {
	return c.callback(c.context)
}

func (c *CallbackHandler) Describe(event ResourceChange) string {
	return fmt.Sprintf("Callback %v(%s)", c.callback, c.context)
}

func (c *Controller) CallbackAfter(delay time.Duration, callback Callback, context string) {
	evt := ResourceChange {
		Handler: &CallbackHandler{
			callback: callback,
			context:  context,
		},
	}
	c.queue.AddAfter(evt, delay)
}

func (c *Controller) WatchNamespaces(options internalinterfaces.TweakListOptionsFunc, handler NamespaceHandler) *NamespaceWatcher {
	watcher := &NamespaceWatcher{
		handler:   handler,
		informer:  corev1informer.NewFilteredNamespaceInformer(
			c.client,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
			options),
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type NamespaceHandler func(string, *corev1.Namespace) error

type NamespaceWatcher struct {
	handler   NamespaceHandler
	informer  cache.SharedIndexInformer
}

func (w *NamespaceWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *NamespaceWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *NamespaceWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Namespace %s", event.Key)
}

func (w *NamespaceWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *NamespaceWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *NamespaceWatcher) Get(key string) (*corev1.Namespace, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.Namespace), nil
}

func (w *NamespaceWatcher) List() []*corev1.Namespace {
	list := w.informer.GetStore().List()
	results := []*corev1.Namespace{}
	for _, o := range list {
		results = append(results, o.(*corev1.Namespace))
	}
	return results
}

func (c *Controller) WatchNodes(handler NodeHandler) *NodeWatcher {
	watcher := &NodeWatcher{
		handler:   handler,
		informer:  corev1informer.NewNodeInformer(
			c.client,
			c.resync,
			cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}),
	}

	watcher.informer.AddEventHandler(c.newEventHandler(watcher))
	return watcher
}

type NodeHandler func(string, *corev1.Node) error

type NodeWatcher struct {
	handler   NodeHandler
	informer  cache.SharedIndexInformer
}

func (w *NodeWatcher) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w *NodeWatcher) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w *NodeWatcher) Describe(event ResourceChange) string {
	return fmt.Sprintf("Node %s", event.Key)
}

func (w *NodeWatcher) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w *NodeWatcher) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}

func (w *NodeWatcher) Get(key string) (*corev1.Node, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}
	return entity.(*corev1.Node), nil
}

func (w *NodeWatcher) List() []*corev1.Node {
	list := w.informer.GetStore().List()
	results := []*corev1.Node{}
	for _, o := range list {
		results = append(results, o.(*corev1.Node))
	}
	return results
}
