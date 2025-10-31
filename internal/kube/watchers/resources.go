package watchers

import (
	"fmt"
	"reflect"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// type aliases for EventProcessor Watch functions
type (
	DynamicHandler = Handler[*unstructured.Unstructured]
	DynamicWatcher = ResourceWatcher[*unstructured.Unstructured]

	// corev1
	ConfigMapHandler = Handler[*corev1.ConfigMap]
	ConfigMapWatcher = ResourceWatcher[*corev1.ConfigMap]
	NamespaceHandler = Handler[*corev1.Namespace]
	NamespaceWatcher = ResourceWatcher[*corev1.Namespace]
	NodeHandler      = Handler[*corev1.Node]
	NodeWatcher      = ResourceWatcher[*corev1.Node]
	PodHandler       = Handler[*corev1.Pod]
	PodWatcher       = ResourceWatcher[*corev1.Pod]
	SecretHandler    = Handler[*corev1.Secret]
	SecretWatcher    = ResourceWatcher[*corev1.Secret]
	ServiceHandler   = Handler[*corev1.Service]
	ServiceWatcher   = ResourceWatcher[*corev1.Service]

	// networking/v1
	IngressHandler = Handler[*networkingv1.Ingress]
	IngressWatcher = ResourceWatcher[*networkingv1.Ingress]

	// route/v1
	RouteHandler = Handler[*routev1.Route]
	RouteWatcher = ResourceWatcher[*routev1.Route]

	// skupper.io/v2alpha1
	AccessTokenHandler              = Handler[*v2alpha1.AccessToken]
	AccessTokenWatcher              = ResourceWatcher[*v2alpha1.AccessToken]
	AccessGrantHandler              = Handler[*v2alpha1.AccessGrant]
	AccessGrantWatcher              = ResourceWatcher[*v2alpha1.AccessGrant]
	AttachedConnectorHandler        = Handler[*v2alpha1.AttachedConnector]
	AttachedConnectorWatcher        = ResourceWatcher[*v2alpha1.AttachedConnector]
	AttachedConnectorBindingHandler = Handler[*v2alpha1.AttachedConnectorBinding]
	AttachedConnectorBindingWatcher = ResourceWatcher[*v2alpha1.AttachedConnectorBinding]
	CertificateHandler              = Handler[*v2alpha1.Certificate]
	CertificateWatcher              = ResourceWatcher[*v2alpha1.Certificate]
	ConnectorHandler                = Handler[*v2alpha1.Connector]
	ConnectorWatcher                = ResourceWatcher[*v2alpha1.Connector]
	LinkHandler                     = Handler[*v2alpha1.Link]
	LinkWatcher                     = ResourceWatcher[*v2alpha1.Link]
	ListenerHandler                 = Handler[*v2alpha1.Listener]
	ListenerWatcher                 = ResourceWatcher[*v2alpha1.Listener]
	RouterAccessHandler             = Handler[*v2alpha1.RouterAccess]
	RouterAccessWatcher             = ResourceWatcher[*v2alpha1.RouterAccess]
	SecuredAccessHandler            = Handler[*v2alpha1.SecuredAccess]
	SecuredAccessWatcher            = ResourceWatcher[*v2alpha1.SecuredAccess]
	SiteHandler                     = Handler[*v2alpha1.Site]
	SiteWatcher                     = ResourceWatcher[*v2alpha1.Site]
)

// Handler function the EventProcessor will use as a callback for a work item
type Handler[T runtime.Object] func(string, T) error

// ResourceWatcher for a specific object type. Used internally by
// EventProcessor for event processing. Also exposes an interface to access
// cached resoruce state.
type ResourceWatcher[T runtime.Object] struct {
	handler  Handler[T]
	gvk      schema.GroupVersionKind
	informer cache.SharedIndexInformer
}

func NewResourceWatcher[T runtime.Object](handler Handler[T], gv schema.GroupVersion, informer cache.SharedIndexInformer) *ResourceWatcher[T] {
	if informer == nil {
		panic("informer cannot be nil")
	}
	var tmp T
	t := reflect.TypeOf(tmp)
	if t.Kind() != reflect.Pointer {
		panic("Resource types must be pointer to a struct.")
	}
	t = t.Elem()
	return &ResourceWatcher[T]{
		handler:  handler,
		gvk:      gv.WithKind(t.Name()),
		informer: informer,
	}
}

func (w ResourceWatcher[T]) Handle(event ResourceChange) error {
	obj, err := w.Get(event.Key)
	if err != nil || w.handler == nil {
		return err
	}
	return w.handler(event.Key, obj)
}

func (w ResourceWatcher[T]) Describe(event ResourceChange) string {
	return fmt.Sprintf("%s %s", w.gvk.Kind, event.Key)
}

func (w ResourceWatcher[T]) Get(key string) (T, error) {
	entity, exists, err := w.informer.GetStore().GetByKey(key)
	if err != nil || !exists {
		var tmp T
		return tmp, err
	}
	result, ok := entity.(T)
	if !ok {
		var tmp T
		panic(fmt.Sprintf("expected type of %T but informer get returned %T", tmp, entity))
	}
	return result, nil
}

func (w ResourceWatcher[T]) List() []T {
	list := w.informer.GetStore().List()
	results := make([]T, len(list))
	var ok bool
	for i := range list {
		results[i], ok = list[i].(T)
		if !ok {
			panic(fmt.Sprintf("expected type of %T but informer list returned %T", results, list))
		}
	}
	return results
}

func (w ResourceWatcher[T]) Start(stopCh <-chan struct{}) {
	go w.informer.Run(stopCh)
}

func (w ResourceWatcher[T]) HasSynced() func() bool {
	return w.informer.HasSynced
}

func (w ResourceWatcher[T]) Sync(stopCh <-chan struct{}) bool {
	return cache.WaitForCacheSync(stopCh, w.informer.HasSynced)
}
