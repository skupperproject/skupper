package labels

import (
	"log/slog"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

type LabelsAndAnnotations struct {
	namespaces          map[string]*Registry
	controllerNamespace string
	log                 *slog.Logger
}

func NewLabelsAndAnnotations(controllerNamespace string) *LabelsAndAnnotations {
	if controllerNamespace == "" {
		controllerNamespace = "default"
	}
	return &LabelsAndAnnotations{
		namespaces:          map[string]*Registry{},
		controllerNamespace: controllerNamespace,
		log: slog.New(slog.Default().Handler()).With(
			slog.String("component", "kube.site.labels.registry"),
		),
	}
}

// Snapshot copies the namespace and controller-wide policy inputs without
// evaluating them. The returned registry is independent of subsequent updates.
func (l *LabelsAndAnnotations) Snapshot(namespace string) *LabelsAndAnnotations {
	result := NewLabelsAndAnnotations(l.controllerNamespace)
	for _, ns := range []string{namespace, l.controllerNamespace} {
		if registry, ok := l.namespaces[ns]; ok {
			copied := newRegistry(l.log)
			for key, entry := range registry.config {
				clone := *entry
				clone.cm = entry.cm.DeepCopy()
				if entry.selector != nil {
					clone.selector = entry.selector.DeepCopySelector()
				}
				copied.config[key] = &clone
			}
			result.namespaces[ns] = copied
		}
	}
	return result
}

func (l *LabelsAndAnnotations) Update(key string, cm *corev1.ConfigMap) error {
	namespace, _, _ := cache.SplitMetaNamespaceKey(key)
	if existing, ok := l.namespaces[namespace]; ok {
		return existing.update(key, cm)
	} else if cm != nil {
		created := newRegistry(l.log)
		l.namespaces[namespace] = created
		return created.update(key, cm)
	}
	return nil
}

func (l *LabelsAndAnnotations) SetLabels(namespace string, name string, kind string, labels map[string]string) bool {
	desired := map[string]string{}
	if registry, ok := l.namespaces[namespace]; ok {
		registry.setLabels(name, kind, labels, desired, false)
	}
	if namespace != l.controllerNamespace {
		if registry, ok := l.namespaces[l.controllerNamespace]; ok {
			registry.setLabels(name, kind, labels, desired, false)
		}
	}
	return setValues(desired, labels)
}

func (l *LabelsAndAnnotations) SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool {
	desired := map[string]string{}
	if registry, ok := l.namespaces[namespace]; ok {
		registry.setAnnotations(name, kind, annotations, desired, false)
	}
	if namespace != l.controllerNamespace {
		if registry, ok := l.namespaces[l.controllerNamespace]; ok {
			registry.setAnnotations(name, kind, annotations, desired, false)
		}
	}
	return setValues(desired, annotations)
}

func (l *LabelsAndAnnotations) SetObjectMetadata(namespace string, name string, kind string, meta *metav1.ObjectMeta) bool {
	return l.setObjectMetadata(namespace, name, kind, meta, false)
}

func (l *LabelsAndAnnotations) SetPodObjectMetadata(namespace string, name string, kind string, meta *metav1.ObjectMeta) bool {
	return l.setObjectMetadata(namespace, name, kind, meta, true)
}

func (l *LabelsAndAnnotations) setObjectMetadata(namespace string, name string, kind string, meta *metav1.ObjectMeta, requireIncludePods bool) bool {
	if meta == nil {
		return false
	}
	if meta.Labels == nil {
		meta.Labels = map[string]string{}
	}
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	}
	changed := false
	if registry, ok := l.namespaces[namespace]; ok {
		if registry.filter(name, kind, meta.Labels, meta.Labels, meta.Annotations, requireIncludePods) {
			changed = true
		}
	}
	if namespace != l.controllerNamespace {
		if registry, ok := l.namespaces[l.controllerNamespace]; ok {
			if registry.filter(name, kind, meta.Labels, meta.Labels, meta.Annotations, requireIncludePods) {
				changed = true
			}
		}
	}
	return changed
}

type Registry struct {
	config map[string]*templateEntry
	log    *slog.Logger
}

type templateEntry struct {
	cm       *corev1.ConfigMap
	selector k8slabels.Selector
	invalid  bool
}

func newRegistry(log *slog.Logger) *Registry {
	return &Registry{
		config: map[string]*templateEntry{},
		log:    log,
	}
}

func (r *Registry) update(key string, cm *corev1.ConfigMap) error {
	_, ok := label(cm, "skupper.io/label-template")
	if !ok {
		delete(r.config, key)
		namespace, name, _ := cache.SplitMetaNamespaceKey(key)
		r.log.Info("Removing label and annotation configuration",
			slog.String("namespace", namespace),
			slog.String("name", name),
		)
		return nil
	}
	if _, ok := r.config[key]; !ok {
		r.log.Info("Loading label and annotation configuration",
			slog.String("namespace", cm.Namespace),
			slog.String("name", cm.Name),
		)
	}
	entry := &templateEntry{cm: cm}
	if cm.Data != nil {
		if selector, ok := cm.Data["labelSelector"]; ok && selector != "" {
			req, err := k8slabels.Parse(selector)
			if err != nil {
				r.log.Info("Ignoring label-template due to invalid labelSelector",
					slog.String("name", cm.Name),
					slog.String("namespace", cm.Namespace),
					slog.String("labelSelector", selector),
					slog.Any("error", err),
				)
				entry.invalid = true
			} else {
				entry.selector = req
			}
		}
	}
	r.config[key] = entry
	return nil
}

func (r *Registry) setLabels(name string, kind string, target map[string]string, labels map[string]string, requireIncludePods bool) bool {
	return r.filter(name, kind, target, labels, nil, requireIncludePods)
}

func (r *Registry) setAnnotations(name string, kind string, target map[string]string, annotations map[string]string, requireIncludePods bool) bool {
	return r.filter(name, kind, target, nil, annotations, requireIncludePods)
}

func (r *Registry) filter(name string, kind string, target map[string]string, labels map[string]string, annotations map[string]string, requireIncludePods bool) bool {
	changed := false
	// Conflicting templates in one namespace must have a stable precedence.
	keys := make([]string, 0, len(r.config))
	for key := range r.config {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		entry := r.config[key]
		if entry.invalid {
			continue
		}
		cm := entry.cm
		if requireIncludePods && !includePods(cm) {
			continue
		}
		if !matchKey(cm, "name", name) {
			continue
		}
		if !matchKey(cm, "kind", kind) {
			continue
		}
		if entry.selector != nil {
			if target == nil || !entry.selector.Matches(k8slabels.Set(target)) {
				continue
			}
		}
		excludes := exclude(cm)
		if labels != nil {
			for k, v := range cm.ObjectMeta.Labels {
				if isExcluded(k, excludes) {
					continue
				}
				if v2, ok := labels[k]; !ok || v != v2 {
					labels[k] = v
					changed = true
				}
			}
		}
		if annotations != nil {
			for k, v := range cm.ObjectMeta.Annotations {
				if isExcluded(k, excludes) {
					continue
				}
				if v2, ok := annotations[k]; !ok || v != v2 {
					annotations[k] = v
					changed = true
				}
			}
		}
	}
	return changed
}

func label(cm *corev1.ConfigMap, key string) (string, bool) {
	if cm == nil || cm.ObjectMeta.Labels == nil {
		return "", false
	}
	label, ok := cm.ObjectMeta.Labels[key]
	return label, ok
}

func matchKey(cm *corev1.ConfigMap, key string, expected string) bool {
	if cm.Data == nil {
		return true
	}
	actual, ok := cm.Data[key]
	if !ok {
		return true
	}
	return actual == expected
}

func includePods(cm *corev1.ConfigMap) bool {
	if cm.Data == nil {
		return false
	}
	return cm.Data["includePods"] == "true"
}

func exclude(cm *corev1.ConfigMap) []string {
	if cm.Data == nil {
		return nil
	}
	value, ok := cm.Data["exclude"]
	if !ok {
		return nil
	}
	return strings.Split(value, ",")
}

func isExcluded(key string, excluded []string) bool {
	if key == "skupper.io/label-template" || key == "kubectl.kubernetes.io/last-applied-configuration" {
		return true
	}
	for _, exclude := range excluded {
		if key == exclude {
			return true
		}
	}
	return false
}

func setValues(desired map[string]string, actual map[string]string) bool {
	changed := false
	for k, v := range desired {
		if v2, ok := actual[k]; !ok || v != v2 {
			actual[k] = v
			changed = true
		}
	}
	return changed
}
