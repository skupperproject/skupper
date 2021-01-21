package client

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (cli *VanClient) RouterUpdateVersion(ctx context.Context, hup bool) (bool, error) {
	return cli.RouterUpdateVersionInNamespace(ctx, hup, cli.Namespace)
}

func (cli *VanClient) RouterUpdateVersionInNamespace(ctx context.Context, hup bool, namespace string) (bool, error) {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	config, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return false, err
	}
	site := config.GetSiteMetadata()
	//compare to version of library running
	updateSite := false
	if utils.LessRecentThanVersion(Version, site.Version) {
		// site is newer than client library, cannot update
		return false, fmt.Errorf("Site (%s) is newer than library (%s); cannot update", site.Version, Version)
	}
	if utils.MoreRecentThanVersion(Version, site.Version) || (utils.EquivalentVersion(Version, site.Version) && Version != site.Version) {
		// site is older than library, may require some upgrade steps
		updateSite = true
		site.Version = Version
		config.SetSiteMetadata(&site)
	}
	if updateSite {
		_, err = config.UpdateConfigMap(configmap)
		if err != nil {
			return false, err
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(namespace).Update(configmap)
		if err != nil {
			return false, err
		}
		// Any version specific upgrade actions go here:
	}

	router, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	updateRouter := false
	desiredRouterImage := GetRouterImageName()
	if router.Spec.Template.Spec.Containers[0].Image != desiredRouterImage {
		router.Spec.Template.Spec.Containers[0].Image = desiredRouterImage
		updateRouter = true
	} else if updateSite || hup {
		//need to trigger a router redployment to pick up the revised metadata field
		touch(router)
		updateRouter = true
	}
	if updateRouter {
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(router)
		if err != nil {
			return false, err
		}
	}

	controller, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(types.ControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	updateController := false
	desiredControllerImage := GetServiceControllerImageName()
	if controller.Spec.Template.Spec.Containers[0].Image != desiredControllerImage {
		controller.Spec.Template.Spec.Containers[0].Image = desiredControllerImage
		updateController = true
	} else if hup {
		//trigger redeployment of service-controller to pick up latest image
		touch(controller)
		updateController = true
	}
	if updateController {
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(controller)
		if err != nil {
			return false, err
		}
	}
	return updateRouter || updateController || updateSite, nil
}

func touch(deployment *appsv1.Deployment) {
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	deployment.Spec.Template.ObjectMeta.Annotations[types.UpdatedAnnotation] = time.Now().Format(time.RFC1123Z)

}
