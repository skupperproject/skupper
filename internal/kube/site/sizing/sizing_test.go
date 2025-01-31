package sizing

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gotest.tools/v3/assert"

	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

func TestSizing(t *testing.T) {
	type Update struct {
		key    string
		config *corev1.ConfigMap
	}
	type Expectation struct {
		site   *skupperv2alpha1.Site
		sizing Sizing
		err    string
	}
	tests := []struct {
		name         string
		config       []Update
		expectations []Expectation
	}{
		{
			name: "no sizing config supplied",
			expectations: []Expectation{
				{
					site: f.site(""),
				},
			},
		},
		{
			name: "simple match",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site:   f.site("mysize"),
					sizing: f.sizing().routerRequest("cpu", "0.5").sizing,
				},
			},
		},
		{
			name: "all values",
			config: []Update{
				{
					key: "foo/bar",
					config: f.config("mysize", false).entry(
						"router-cpu-request", "0.5",
					).entry(
						"router-cpu-limit", "0.6",
					).entry(
						"router-memory-request", "300M",
					).entry(
						"router-memory-limit", "400M",
					).entry(
						"adaptor-cpu-request", "0.3",
					).entry(
						"adaptor-cpu-limit", "0.4",
					).entry(
						"adaptor-memory-request", "100M",
					).entry(
						"adaptor-memory-limit", "200M",
					).configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site: f.site("mysize"),
					sizing: f.sizing().routerRequest(
						"cpu", "0.5",
					).routerRequest(
						"memory", "300M",
					).routerLimit(
						"cpu", "0.6",
					).routerLimit(
						"memory", "400M",
					).adaptorRequest(
						"cpu", "0.3",
					).adaptorRequest(
						"memory", "100M",
					).adaptorLimit(
						"cpu", "0.4",
					).adaptorLimit(
						"memory", "200M",
					).sizing,
				},
			},
		},
		{
			name: "default config",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
				{
					key:    "foo/baz",
					config: f.config("anothersize", true).entry("router-cpu-limit", "4").configmap("baz", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site:   f.site(""),
					sizing: f.sizing().routerLimit("cpu", "4").sizing,
				},
			},
		},
		{
			name: "bad value",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").entry("router-cpu-limit", "1xyz").configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site:   f.site("mysize"),
					sizing: f.sizing().routerRequest("cpu", "0.5").sizing,
					err:    "Bad value for router-cpu-limit in foo/bar",
				},
			},
		},
		{
			name: "unknown key",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").entry("flibbertygibbet", "500M").configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site:   f.site("mysize"),
					sizing: f.sizing().routerRequest("cpu", "0.5").sizing,
					err:    "Ignoring key flibbertygibbet in foo/bar",
				},
			},
		},
		{
			name: "config changed",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.6").configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site:   f.site("mysize"),
					sizing: f.sizing().routerRequest("cpu", "0.6").sizing,
				},
			},
		},
		{
			name: "config deleted",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
				{
					key: "foo/bar",
				},
			},
			expectations: []Expectation{
				{
					site: f.site("mysize"),
				},
			},
		},
		{
			name: "label removed",
			config: []Update{
				{
					key:    "foo/bar",
					config: f.config("mysize", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
				{
					key:    "foo/bar",
					config: f.config("", false).entry("router-cpu-request", "0.5").configmap("bar", "foo"),
				},
			},
			expectations: []Expectation{
				{
					site: f.site("mysize"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			for _, update := range tt.config {
				registry.Update(update.key, update.config)
			}
			for _, expectation := range tt.expectations {
				actual, err := registry.GetSizing(expectation.site)
				if expectation.err != "" {
					assert.ErrorContains(t, err, expectation.err)
				} else {
					assert.Assert(t, err)
				}
				assert.DeepEqual(t, expectation.sizing, actual)
				assert.Equal(t, expectation.sizing.Router.NotEmpty(), actual.Router.NotEmpty())
				assert.Equal(t, expectation.sizing.Adaptor.NotEmpty(), actual.Adaptor.NotEmpty())
			}
		})
	}
}

type factory struct{}

func (*factory) site(size string) *skupperv2alpha1.Site {
	if size == "" {
		return &skupperv2alpha1.Site{}
	}
	return &skupperv2alpha1.Site{
		Spec: skupperv2alpha1.SiteSpec{
			Settings: map[string]string{
				"size": size,
			},
		},
	}
}

type ConfigBuilder struct {
	labels      map[string]string
	annotations map[string]string
	data        map[string]string
}

func (*factory) config(size string, isDefault bool) *ConfigBuilder {
	config := &ConfigBuilder{
		data: map[string]string{},
	}
	if size != "" {
		config.label(SiteSizingLabel, size)
	}
	if isDefault {
		config.annotation(DefaultSiteSizingAnnotation, "")
	}
	return config
}

func (c *ConfigBuilder) entry(key string, value string) *ConfigBuilder {
	c.data[key] = value
	return c
}

func (c *ConfigBuilder) label(key string, value string) *ConfigBuilder {
	if c.labels == nil {
		c.labels = map[string]string{}
	}
	c.labels[key] = value
	return c
}

func (c *ConfigBuilder) annotation(key string, value string) *ConfigBuilder {
	if c.annotations == nil {
		c.annotations = map[string]string{}
	}
	c.annotations[key] = value
	return c
}

func (c *ConfigBuilder) configmap(name string, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      c.labels,
			Annotations: c.annotations,
		},
		Data: c.data,
	}
}

type SizingBuilder struct {
	sizing Sizing
}

func (*factory) sizing() *SizingBuilder {
	return &SizingBuilder{
		sizing: Sizing{
			Router: ContainerResources{
				Requests: map[string]string{},
				Limits:   map[string]string{},
			},
			Adaptor: ContainerResources{
				Requests: map[string]string{},
				Limits:   map[string]string{},
			},
		},
	}
}

func (s *SizingBuilder) routerRequest(key string, value string) *SizingBuilder {
	s.sizing.Router.Requests[key] = value
	return s
}

func (s *SizingBuilder) routerLimit(key string, value string) *SizingBuilder {
	s.sizing.Router.Limits[key] = value
	return s
}

func (s *SizingBuilder) adaptorRequest(key string, value string) *SizingBuilder {
	s.sizing.Adaptor.Requests[key] = value
	return s
}

func (s *SizingBuilder) adaptorLimit(key string, value string) *SizingBuilder {
	s.sizing.Adaptor.Limits[key] = value
	return s
}

var f factory
