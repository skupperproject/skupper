package main

import (
	"context"
	"testing"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/client"
	"github.com/skupperproject/skupper/pkg/event"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"gotest.tools/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func skupperInit(cli *client.VanClient, name string) error {
	ctx := context.Background()
	config, err := cli.SiteConfigCreate(ctx, types.SiteConfigSpec{SkupperName: name, Ingress: "none"})
	if err != nil {
		return err
	}
	return cli.RouterCreate(ctx, *config)
}

func getRouterConfig(cli *client.VanClient) (*qdr.RouterConfig, error) {
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli.Namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}
	return qdr.GetRouterConfigFromConfigMap(configmap)
}

func hasVolume(spec *corev1.PodSpec, name string) bool {
	for _, volume := range spec.Volumes {
		if volume.Name == name {
			return true
		}
	}
	return false
}

func hasVolumeMount(spec *corev1.PodSpec, name string) bool {
	for i, _ := range spec.Containers {
		for _, mount := range spec.Containers[i].VolumeMounts {
			if mount.Name == name {
				return true
			}
		}
	}
	return false
}

func checkVolumeMount(spec *corev1.PodSpec, name string) bool {
	return hasVolume(spec, name) && hasVolumeMount(spec, name)
}

func createToken(name string, annotations map[string]string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				types.SkupperTypeQualifier: types.TypeToken,
			},
			Annotations: annotations,
		},
		Data: map[string][]byte{
			"a": []byte("1"),
			"b": []byte("2"),
		},
	}
}

func TestTokenHandler(t *testing.T) {

	event.StartDefaultEventStore(nil)
	cli := &client.VanClient{
		Namespace:  "claim-handler-test",
		KubeClient: fake.NewSimpleClientset(),
	}

	handler := newTokenHandler(cli, "site-a")

	name := "foo"
	err := skupperInit(cli, name)
	assert.Check(t, err, name)

	var tests = []struct {
		name              string
		annotations       map[string]string
		expectedConnector *qdr.Connector
	}{
		{
			name: "one",
			annotations: map[string]string{
				types.TokenCost:     "4",
				"inter-router-host": "myrouter.com",
				"inter-router-port": "55671",
			},
			expectedConnector: &qdr.Connector{
				Name:       "one",
				Host:       "myrouter.com",
				Port:       "55671",
				Cost:       int32(4),
				SslProfile: "one-profile",
			},
		},
		{
			name: "two",
			annotations: map[string]string{
				types.TokenCost:     "foo",
				"inter-router-host": "myrouter.com",
				"inter-router-port": "55671",
			},
			expectedConnector: &qdr.Connector{
				Name:       "two",
				Host:       "myrouter.com",
				Port:       "55671",
				Cost:       0,
				SslProfile: "two-profile",
			},
		},
		{
			name: "three",
			annotations: map[string]string{
				"inter-router-host": "myrouter.com",
				"inter-router-port": "55671",
			},
			expectedConnector: &qdr.Connector{
				Name:       "three",
				Host:       "myrouter.com",
				Port:       "55671",
				Cost:       0,
				SslProfile: "three-profile",
			},
		},
		{
			name:        "four",
			annotations: nil,
			expectedConnector: &qdr.Connector{
				Name:       "four",
				Host:       "",
				Port:       "",
				Cost:       0,
				SslProfile: "four-profile",
			},
		},
		{
			name: "five",
			annotations: map[string]string{
				types.TokenCost:        "4",
				types.TokenGeneratedBy: "site-a",
				"inter-router-host":    "myrouter.com",
				"inter-router-port":    "55671",
			},
			expectedConnector: nil,
		},
	}
	for _, test := range tests {
		token := createToken(test.name, test.annotations)
		_, err = cli.KubeClient.CoreV1().Secrets(cli.Namespace).Create(token)
		assert.Check(t, err, name)

		err = handler.handler.Handle(test.name, token)
		assert.Check(t, err, test.name)
		config, err := getRouterConfig(cli)
		assert.Check(t, err, test.name)
		connector, ok := config.Connectors[test.name]
		if test.expectedConnector == nil {
			assert.Assert(t, !ok, test.name)
			config, err = getRouterConfig(cli)
			assert.Check(t, err, test.name)
			connector, ok = config.Connectors[test.name]
			assert.Assert(t, !ok, test.name)
			deployment, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
			assert.Check(t, err, test.name)
			assert.Assert(t, !checkVolumeMount(&deployment.Spec.Template.Spec, test.name), test.name)
		} else {
			assert.Assert(t, ok, test.name)
			assert.Equal(t, connector.Name, test.expectedConnector.Name, test.name)
			assert.Equal(t, connector.Host, test.expectedConnector.Host, test.name)
			assert.Equal(t, connector.Port, test.expectedConnector.Port, test.name)
			assert.Equal(t, connector.Cost, test.expectedConnector.Cost, test.name)
			assert.Equal(t, connector.SslProfile, test.expectedConnector.SslProfile, test.name)
			deployment, err := kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
			assert.Check(t, err, test.name)
			assert.Assert(t, checkVolumeMount(&deployment.Spec.Template.Spec, test.name), test.name)
			//now disconnect:
			err = handler.handler.Handle(test.name, nil)
			assert.Check(t, err, test.name)
			config, err = getRouterConfig(cli)
			assert.Check(t, err, test.name)
			connector, ok = config.Connectors[test.name]
			assert.Assert(t, !ok, test.name)
			deployment, err = kube.GetDeployment(types.TransportDeploymentName, cli.Namespace, cli.KubeClient)
			assert.Check(t, err, test.name)
			assert.Assert(t, !checkVolumeMount(&deployment.Spec.Template.Spec, test.name), test.name)
		}
	}
}
