package adaptor

import (
	"context"
	"log"
	"os"
	paths "path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/skupperproject/skupper/internal/qdr"
)

func InitialiseConfig(client kubernetes.Interface, namespace string, path string, routerConfigMap string) error {
	ctxt := context.Background()
	current, err := client.CoreV1().ConfigMaps(namespace).Get(ctxt, routerConfigMap, metav1.GetOptions{})
	if err != nil {
		return err
	}

	config, err := qdr.GetRouterConfigFromConfigMap(current)
	if err != nil {
		return err
	}

	value, err := qdr.MarshalRouterConfig(*config)
	if err != nil {
		return err
	}
	configFile := paths.Join(path, "skrouterd.json")
	if err := os.WriteFile(configFile, []byte(value), 0777); err != nil {
		return err
	}
	log.Printf("Router configuration written to %s", configFile)

	profileSyncer := newSslProfileSyncer(path)
	for _, profile := range config.SslProfiles {
		target, _ := profileSyncer.get(profile.Name)

		secret, err := client.CoreV1().Secrets(namespace).Get(ctxt, target.name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := target.sync(secret); err != nil {
			return err
		}
		log.Printf("Resources for SslProfile %s written to %s", profile.Name, target.path)
	}
	return nil
}
