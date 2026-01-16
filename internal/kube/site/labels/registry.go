package labels

import (
	"log/slog"
	"strings"

	corev1 "k8s.io/api/core/v1"
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
		registry.setLabels(name, kind, desired)
	}
	if namespace != l.controllerNamespace {
		if registry, ok := l.namespaces[l.controllerNamespace]; ok {
			registry.setLabels(name, kind, desired)
		}
	}
	return setValues(desired, labels)
}

func (l *LabelsAndAnnotations) SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool {
	desired := map[string]string{}
	if registry, ok := l.namespaces[namespace]; ok {
		registry.setAnnotations(name, kind, desired)
	}
	if namespace != l.controllerNamespace {
		if registry, ok := l.namespaces[l.controllerNamespace]; ok {
			registry.setAnnotations(name, kind, desired)
		}
	}
	return setValues(desired, annotations)
}

type Registry struct {
	config map[string]*corev1.ConfigMap
	log    *slog.Logger
}

func newRegistry(log *slog.Logger) *Registry {
	return &Registry{
		config: map[string]*corev1.ConfigMap{},
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
	r.config[key] = cm
	return nil
}

func (r *Registry) setLabels(name string, kind string, labels map[string]string) bool {
	return r.filter(name, kind, labels, nil)
}

func (r *Registry) setAnnotations(name string, kind string, annotations map[string]string) bool {
	return r.filter(name, kind, nil, annotations)
}

func (r *Registry) filter(name string, kind string, labels map[string]string, annotations map[string]string) bool {
	changed := false
	for _, cm := range r.config {
		if !matchKey(cm, "name", name) {
			continue
		}
		if !matchKey(cm, "kind", kind) {
			continue
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
				if v2, ok := labels[k]; !ok || v != v2 {
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
