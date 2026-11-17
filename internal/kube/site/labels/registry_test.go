package labels

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/v3/assert"
)

type Update struct {
	key    string
	config *corev1.ConfigMap
}

func TestPolicySnapshot(t *testing.T) {
	live := NewLabelsAndAnnotations("controller")
	local := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "local",
		Labels:      map[string]string{"skupper.io/label-template": "true", "priority": "local"},
		Annotations: map[string]string{"selected": "original"}},
		Data: map[string]string{"kind": "Secret", "labelSelector": "role=server"}}
	assert.NilError(t, live.Update("test/local", local))
	for _, name := range []string{"z", "a"} {
		cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "controller", Name: name,
			Labels: map[string]string{"skupper.io/label-template": "true", "priority": name}}, Data: map[string]string{"kind": "Secret"}}
		assert.NilError(t, live.Update("controller/"+name, cm))
	}
	frozen := live.Snapshot("test")
	local.Annotations["selected"] = "changed"
	local.Data["labelSelector"] = "role=client"
	assert.NilError(t, live.Update("test/local", local))
	assert.NilError(t, live.Update("controller/z", nil))
	for i := 0; i < 20; i++ {
		values := map[string]string{"role": "server"}
		frozen.SetLabels("test", "leaf", "Secret", values)
		assert.Equal(t, values["priority"], "z", "controller policy overrides local; template name breaks ties")
		annotations := map[string]string{"role": "server"}
		frozen.SetAnnotations("test", "leaf", "Secret", annotations)
		assert.Equal(t, annotations["selected"], "original")
		unmatched := map[string]string{"role": "client"}
		frozen.SetAnnotations("test", "leaf", "Secret", unmatched)
		assert.Equal(t, unmatched["selected"], "")
	}
}

func TestLabels(t *testing.T) {
	type Expectation struct {
		namespace   string
		name        string
		kind        string
		labels      map[string]string
		annotations map[string]string
	}
	tests := []struct {
		name                string
		controllerNamespace string
		config              []Update
		expectations        []Expectation
	}{
		{
			name: "namespace scoped labels matching everything",
			config: []Update{
				update("test/labels", map[string]string{"foo": "bar"}, nil, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					labels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name:                "namespace scoped annotations matching everything",
			controllerNamespace: "skupper",
			config: []Update{
				update("test/labels", nil, map[string]string{"foo": "bar"}, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		{
			name: "controller scoped labels override some entries",
			config: []Update{
				update("default/labels", map[string]string{"foo": "baz"}, nil, "", ""),
				update("test/labels", map[string]string{"foo": "bar", "bing": "bong"}, nil, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					labels: map[string]string{
						"foo":  "baz",
						"bing": "bong",
					},
				},
			},
		},
		{
			name: "controller scoped anntations override some entries",
			config: []Update{
				update("default/labels", nil, map[string]string{"foo": "baz"}, "", ""),
				update("test/labels", nil, map[string]string{"foo": "bar", "bing": "bong"}, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"foo":  "baz",
						"bing": "bong",
					},
				},
			},
		},
		{
			name: "update of label configuration",
			config: []Update{
				update("test/labels", map[string]string{"foo": "bar"}, nil, "", ""),
				update("test/labels", map[string]string{"foo": "baz"}, nil, "", ""),
				update("test/labels", map[string]string{"foo": "baz"}, nil, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					labels: map[string]string{
						"foo": "baz",
					},
				},
			},
		},
		{
			name: "update of annotation configuration",
			config: []Update{
				update("test/annotations", nil, map[string]string{"foo": "bar"}, "", ""),
				update("test/annotations", nil, map[string]string{"foo": "baz"}, "", ""),
				update("test/annotations", nil, map[string]string{"foo": "baz"}, "", ""),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"foo": "baz",
					},
				},
			},
		},
		{
			name: "deletion of label configuration",
			config: []Update{
				update("default/labels", map[string]string{"foo": "baz"}, nil, "", ""),
				update("test/labels", map[string]string{"foo": "bar", "bing": "bong"}, nil, "", ""),
				{
					key: "default/labels",
				},
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					labels: map[string]string{
						"foo":  "bar",
						"bing": "bong",
					},
				},
			},
		},
		{
			name: "deletion of annotation configuration",
			config: []Update{
				update("default/annotations", nil, map[string]string{"foo": "baz"}, "", ""),
				update("test/annotations", nil, map[string]string{"foo": "bar", "bing": "bong"}, "", ""),
				{
					key: "test/annotations",
				},
				{
					key: "never/existed",
				},
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"foo": "baz",
					},
				},
			},
		},
		{
			name: "restricted label configuration",
			config: []Update{
				update("default/all", map[string]string{"ding": "dong"}, nil, "", ""),
				update("default/svc-specific", map[string]string{"foo": "faa"}, nil, "Service", ""),
				update("test/labels", map[string]string{"foo": "bar", "bing": "bong"}, nil, "", "xyz"),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					labels: map[string]string{
						"foo":  "bar",
						"bing": "bong",
						"ding": "dong",
					},
				},
				{
					namespace: "test",
					name:      "abc",
					kind:      "Service",
					labels: map[string]string{
						"foo":  "faa",
						"ding": "dong",
					},
				},
				{
					namespace: "test",
					name:      "xyz",
					kind:      "Service",
					labels: map[string]string{
						"foo":  "faa",
						"bing": "bong",
						"ding": "dong",
					},
				},
				{
					namespace: "other",
					name:      "def",
					kind:      "Deployment",
					labels: map[string]string{
						"ding": "dong",
					},
				},
			},
		},
		{
			name: "restricted annotation configuration",
			config: []Update{
				update("default/all", nil, map[string]string{"ding": "dong"}, "", ""),
				update("default/svc-specific", nil, map[string]string{"foo": "faa"}, "Service", ""),
				update("test/annotations", nil, map[string]string{"foo": "bar", "bing": "bong"}, "", "xyz"),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"foo":  "bar",
						"bing": "bong",
						"ding": "dong",
					},
				},
				{
					namespace: "test",
					name:      "abc",
					kind:      "Service",
					annotations: map[string]string{
						"foo":  "faa",
						"ding": "dong",
					},
				},
				{
					namespace: "test",
					name:      "xyz",
					kind:      "Service",
					annotations: map[string]string{
						"foo":  "faa",
						"bing": "bong",
						"ding": "dong",
					},
				},
				{
					namespace: "other",
					name:      "def",
					kind:      "Deployment",
					annotations: map[string]string{
						"ding": "dong",
					},
				},
			},
		},
		{
			name: "excludes",
			config: []Update{
				updateWithExcludes("test/labels", nil, map[string]string{"acme.com/foo": "bar", "dont/copy/me": "xyz", "or-me": ""}, "", "", "dont/copy/me,or-me"),
			},
			expectations: []Expectation{
				{
					namespace: "test",
					name:      "xyz",
					kind:      "ConfigMap",
					annotations: map[string]string{
						"acme.com/foo": "bar",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewLabelsAndAnnotations(tt.controllerNamespace)
			for _, update := range tt.config {
				registry.Update(update.key, update.config)
			}
			for _, expectation := range tt.expectations {
				if expectation.labels != nil {
					actual := map[string]string{}
					result := registry.SetLabels(expectation.namespace, expectation.name, expectation.kind, actual)
					//t.Logf("Checking labels for %s %s/%s", expectation.kind, expectation.namespace, expectation.name)
					assert.DeepEqual(t, actual, expectation.labels)
					assert.Assert(t, result)
				}
				if expectation.annotations != nil {
					actual := map[string]string{}
					result := registry.SetAnnotations(expectation.namespace, expectation.name, expectation.kind, actual)
					//t.Logf("Checking annotations for %s %s/%s", expectation.kind, expectation.namespace, expectation.name)
					assert.DeepEqual(t, actual, expectation.annotations)
					assert.Assert(t, result)
				}
			}
		})
	}
}

func update(key string, labels map[string]string, annotations map[string]string, kind string, name string) Update {
	return updateWithExcludes(key, labels, annotations, kind, name, "")
}

func updateWithExcludes(key string, labels map[string]string, annotations map[string]string, kind string, name string, excludes string) Update {
	return updateWithOptions(key, labels, annotations, kind, name, excludes, "")
}

func updateWithIncludePods(key string, labels map[string]string, annotations map[string]string, kind string, name string, includePods bool) Update {
	includePodsValue := "false"
	if includePods {
		includePodsValue = "true"
	}
	return updateWithOptions(key, labels, annotations, kind, name, "", includePodsValue)
}

func updateWithOptions(key string, labels map[string]string, annotations map[string]string, kind string, name string, excludes string, includePods string) Update {
	data := map[string]string{}
	if kind != "" {
		data["kind"] = kind
	}
	if name != "" {
		data["name"] = name
	}
	if excludes != "" {
		data["exclude"] = excludes
	}
	if includePods != "" {
		data["includePods"] = includePods
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels["skupper.io/label-template"] = "true"
	if len(data) == 0 {
		data = nil
	}
	return configmap(key, data, labels, annotations)
}

func configmap(key string, data map[string]string, labels map[string]string, annotations map[string]string) Update {
	return Update{
		key: key,
		config: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      labels,
				Annotations: annotations,
			},
			Data: data,
		},
	}
}

func updateWithSelector(key string, labels map[string]string, annotations map[string]string, selector string) Update {
	data := map[string]string{}
	if selector != "" {
		data["labelSelector"] = selector
	}
	if labels == nil {
		labels = map[string]string{}
	}
	labels["skupper.io/label-template"] = "true"
	if len(data) == 0 {
		data = nil
	}
	return configmap(key, data, labels, annotations)
}

func TestLabelSelectorFiltering(t *testing.T) {
	registry := NewLabelsAndAnnotations("default")
	// apply a configmap with a selector that matches listener=my-listener
	registry.Update("test/selector", updateWithSelector("test/selector", map[string]string{"acme.com/foo": "bar"}, nil, "internal.skupper.io/listener in (my-listener)").config)
	// Case 1: labels match selector
	actual := map[string]string{"internal.skupper.io/listener": "my-listener"}
	changed := registry.SetLabels("test", "svc-a", "Service", actual)
	assert.Assert(t, changed)
	assert.Equal(t, actual["acme.com/foo"], "bar")
	// Case 2: labels do not match selector
	actual2 := map[string]string{"internal.skupper.io/listener": "other"}
	changed2 := registry.SetLabels("test", "svc-b", "Service", actual2)
	assert.Assert(t, !changed2)
	_, present := actual2["acme.com/foo"]
	assert.Assert(t, !present)
	// Case 3: invalid selector should be ignored (no application)
	registry.Update("test/invalid", updateWithSelector("test/invalid", map[string]string{"acme.com/bar": "baz"}, nil, "this is not a valid selector").config)
	actual3 := map[string]string{"internal.skupper.io/listener": "my-listener"}
	_ = registry.SetLabels("test", "svc-c", "Service", actual3)
	_, present = actual3["acme.com/bar"]
	assert.Assert(t, !present)
}

func TestIncludePods(t *testing.T) {
	registry := NewLabelsAndAnnotations("default")

	// Without includePods, resource metadata is labelled but pods are not.
	registry.Update("test/no-pods", update("test/no-pods", map[string]string{"acme.com/foo": "bar"}, map[string]string{"acme.com/annot": "baz"}, "", "").config)
	resourceMeta := &metav1.ObjectMeta{}
	assert.Assert(t, registry.SetObjectMetadata("test", "skupper-router", "Deployment", resourceMeta))
	assert.Equal(t, resourceMeta.Labels["acme.com/foo"], "bar")
	assert.Equal(t, resourceMeta.Annotations["acme.com/annot"], "baz")

	podMeta := &metav1.ObjectMeta{}
	assert.Assert(t, !registry.SetPodObjectMetadata("test", "skupper-router", "Deployment", podMeta))
	_, present := podMeta.Labels["acme.com/foo"]
	assert.Assert(t, !present)
	_, present = podMeta.Annotations["acme.com/annot"]
	assert.Assert(t, !present)

	// With includePods=true, the same labels/annotations also apply to pods.
	registry.Update("test/with-pods", updateWithIncludePods("test/with-pods", map[string]string{"acme.com/pod": "yes"}, map[string]string{"acme.com/pod-annot": "yes"}, "", "", true).config)
	podMeta = &metav1.ObjectMeta{}
	assert.Assert(t, registry.SetPodObjectMetadata("test", "skupper-router", "Deployment", podMeta))
	assert.Equal(t, podMeta.Labels["acme.com/pod"], "yes")
	assert.Equal(t, podMeta.Annotations["acme.com/pod-annot"], "yes")
	// Templates without includePods still must not appear on pods.
	_, present = podMeta.Labels["acme.com/foo"]
	assert.Assert(t, !present)

	// Resource metadata still receives labels from both templates.
	resourceMeta = &metav1.ObjectMeta{}
	assert.Assert(t, registry.SetObjectMetadata("test", "skupper-router", "Deployment", resourceMeta))
	assert.Equal(t, resourceMeta.Labels["acme.com/foo"], "bar")
	assert.Equal(t, resourceMeta.Labels["acme.com/pod"], "yes")

	// includePods values other than "true" do not opt in.
	registry.Update("test/false-pods", updateWithIncludePods("test/false-pods", map[string]string{"acme.com/no": "pods"}, nil, "", "", false).config)
	podMeta = &metav1.ObjectMeta{}
	_ = registry.SetPodObjectMetadata("test", "skupper-router", "Deployment", podMeta)
	_, present = podMeta.Labels["acme.com/no"]
	assert.Assert(t, !present)
	// The earlier includePods=true template still applies.
	assert.Equal(t, podMeta.Labels["acme.com/pod"], "yes")
}
