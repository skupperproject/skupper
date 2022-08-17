package qdr

import (
	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

func AddSslProfile(secretName string, cli types.ConfigMaps) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}

	if _, ok := current.SslProfiles[secretName]; !ok {
		current.AddSslProfile(qdr.SslProfile{
			Name: secretName,
		})
	}
	_, err = current.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}

	_, err = cli.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}
	return nil

}

func RemoveSslProfile(secretName string, cli types.ConfigMaps) error {

	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cli)
	if err != nil {
		return err
	}
	current, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return err
	}

	if _, ok := current.SslProfiles[secretName]; ok {
		current.RemoveSslProfile(secretName)
	}

	_, err = current.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}

	_, err = cli.UpdateConfigMap(configmap)
	if err != nil {
		return err
	}
	return nil
}
