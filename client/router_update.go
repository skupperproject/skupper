package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/skupperproject/skupper/pkg/images"
	"github.com/skupperproject/skupper/pkg/version"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/site"
	"github.com/skupperproject/skupper/pkg/utils"
)

func (cli *VanClient) RouterUpdateVersion(ctx context.Context, hup bool) (bool, error) {
	return cli.RouterUpdateVersionInNamespace(ctx, hup, cli.Namespace)
}

func (cli *VanClient) updateStarted(from string, namespace string, ownerrefs []metav1.OwnerReference) error {
	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:            "skupper-update-state",
			OwnerReferences: ownerrefs,
		},
		Data: map[string]string{
			"from": from,
		},
	}
	_, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Create(context.TODO(), cm, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (cli *VanClient) updateCompleted(namespace string) error {
	return cli.KubeClient.CoreV1().ConfigMaps(namespace).Delete(context.TODO(), "skupper-update-state", metav1.DeleteOptions{})
}

func (cli *VanClient) isUpdating(namespace string) (bool, string, error) {
	cm, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), "skupper-update-state", metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return false, "", nil
	} else if err != nil {
		return false, "", err
	}
	return true, cm.Data["from"], nil
}

func (cli *VanClient) RouterUpdateVersionInNamespace(ctx context.Context, hup bool, namespace string) (bool, error) {
	// Validate if router config file needs to be renamed
	renamedSkRouter, err := cli.renameRouterConfigFile(namespace)
	if err != nil {
		return false, err
	}
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	config, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return false, err
	}
	site := config.GetSiteMetadata()
	// compare to version of library running
	updateSite := false
	if utils.LessRecentThanVersion(version.Version, site.Version) {
		// site is newer than client library, cannot update
		return false, fmt.Errorf("Site (%s) is newer than library (%s); cannot update", site.Version, version.Version)
	}
	renameFor050 := false
	addClaimsSupport := false
	addMultiportServices := false
	addClusterPolicy := false
	updateRouterPolicyRule := false
	addCertsSharedVolume := false
	substituteFlowCollector := false
	addPrometheusServer := false
	moveClaims := false
	inprogress, originalVersion, err := cli.isUpdating(namespace)
	if err != nil {
		return false, err
	}
	if inprogress {
		renameFor050 = utils.LessRecentThanVersion(originalVersion, "0.5.0")
		addClaimsSupport = utils.LessRecentThanVersion(originalVersion, "0.7.0")
		addMultiportServices = utils.LessRecentThanVersion(originalVersion, "0.8.0")
		addClusterPolicy = utils.LessRecentThanVersion(originalVersion, "0.9.0")
		updateRouterPolicyRule = utils.LessRecentThanVersion(originalVersion, "0.9.0")
		addCertsSharedVolume = utils.LessRecentThanVersion(originalVersion, "0.9.0")
		substituteFlowCollector = utils.LessRecentThanVersion(originalVersion, "1.3.0")
		addPrometheusServer = utils.LessRecentThanVersion(originalVersion, "1.4.0")
		moveClaims = utils.LessRecentThanVersion(originalVersion, "1.5.0")
	} else {
		originalVersion = site.Version
	}
	if utils.MoreRecentThanVersion(version.Version, site.Version) || (utils.EquivalentVersion(version.Version, site.Version) && version.Version != site.Version) {
		if !inprogress {
			if utils.LessRecentThanVersion(originalVersion, "0.7.0") {
				addClaimsSupport = true
				renameFor050 = utils.LessRecentThanVersion(originalVersion, "0.5.0")
			}
			if utils.LessRecentThanVersion(originalVersion, "0.8.0") {
				addMultiportServices = true
			}
			if utils.LessRecentThanVersion(originalVersion, "0.9.0") {
				addClusterPolicy = true
				updateRouterPolicyRule = true
				addCertsSharedVolume = true
			}
			if utils.LessRecentThanVersion(originalVersion, "1.3.0") {
				substituteFlowCollector = true
			}
			if utils.LessRecentThanVersion(originalVersion, "1.4.0") {
				addPrometheusServer = true
			}
			if utils.LessRecentThanVersion(originalVersion, "1.5.0") {
				moveClaims = true
			}

			err = cli.updateStarted(site.Version, namespace, configmap.ObjectMeta.OwnerReferences)
			if err != nil {
				return false, err
			}
			inprogress = true
		}

		// site is marked as older than library, need to update
		updateSite = true

		site.Version = version.Version
		config.SetSiteMetadata(&site)

		_, err = config.UpdateConfigMap(configmap)
		if err != nil {
			return false, err
		}
		_, err = cli.KubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
	}
	usingRoutes := false
	consoleUsesLoadbalancer := false
	routerExposedAsIp := false
	if renameFor050 {
		// create new resources (as copies of old ones)
		// services
		_, err = kube.CopyService("skupper-messaging", types.LocalTransportServiceName, map[string]string{}, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		_, err = kube.CopyService("skupper-internal", types.TransportServiceName, map[string]string{}, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		servingCertsAnnotation := map[string]string{
			"service.alpha.openshift.io/serving-cert-secret-name": types.ConsoleServerSecret,
		}
		controllerSvc, err := kube.CopyService("skupper-controller", types.ControllerServiceName, servingCertsAnnotation, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		if controllerSvc != nil {
			consoleUsesLoadbalancer = controllerSvc.Spec.Type == corev1.ServiceTypeLoadBalancer
		}
		// update annotation on skupper-router-console if it exists
		routerConsoleService, err := cli.KubeClient.CoreV1().Services(namespace).Get(context.TODO(), types.RouterConsoleServiceName, metav1.GetOptions{})
		if err == nil {
			if routerConsoleService.ObjectMeta.Annotations == nil {
				routerConsoleService.ObjectMeta.Annotations = map[string]string{}
			}
			routerConsoleService.ObjectMeta.Annotations["service.alpha.openshift.io/serving-cert-secret-name"] = types.OauthRouterConsoleSecret
			_, err := cli.KubeClient.CoreV1().Services(namespace).Update(context.TODO(), routerConsoleService, metav1.UpdateOptions{})
			if err != nil {
				return false, err
			}
		}

		// secrets
		// ca's just need to be copied to new secret
		err = kube.CopySecret("skupper-ca", types.LocalCaSecret, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		err = kube.CopySecret("skupper-internal-ca", types.SiteCaSecret, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		// credentials need to be regenerated to be valid for new service names
		credentials := []types.Credential{}
		credentials = append(credentials, types.Credential{
			CA:          types.LocalCaSecret,
			Name:        types.LocalServerSecret,
			Subject:     types.LocalTransportServiceName,
			Hosts:       []string{types.LocalTransportServiceName, types.QualifiedServiceName(types.LocalTransportServiceName, namespace)},
			ConnectJson: false,
		})
		credentials = append(credentials, types.Credential{
			CA:          types.LocalCaSecret,
			Name:        types.LocalClientSecret,
			Subject:     types.LocalTransportServiceName,
			Hosts:       []string{},
			ConnectJson: true,
		})
		credentials = append(credentials, types.Credential{
			CA:          types.ServiceCaSecret,
			Name:        types.ServiceClientSecret,
			Hosts:       []string{},
			ConnectJson: false,
			Post:        false,
			Simple:      true,
		})

		usingRoutes, err = cli.usingRoutes(namespace)
		if usingRoutes {
			// no need to regenerate certificate as route names have not changed
			err = kube.CopySecret("skupper-internal", types.SiteServerSecret, namespace, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return false, err
			}
		} else {
			hosts, err := cli.getTransportHosts(namespace)
			if err != nil {
				return false, err
			}
			if len(hosts) > 0 {
				ip := net.ParseIP(hosts[0])
				if ip != nil {
					routerExposedAsIp = true
				}
			}

			subject := types.TransportServiceName
			for _, host := range hosts {
				if len(host) < 64 {
					subject = host
					break
				}
			}
			credentials = append(credentials, types.Credential{
				CA:          types.SiteCaSecret,
				Name:        types.SiteServerSecret,
				Subject:     subject,
				Hosts:       hosts,
				ConnectJson: false,
			})
		}
		for _, cred := range credentials {
			var owner *metav1.OwnerReference
			if len(configmap.ObjectMeta.OwnerReferences) > 0 {
				owner = &configmap.ObjectMeta.OwnerReferences[0]
			}
			kube.NewSecret(cred, owner, namespace, cli.KubeClient)
		}

		// serviceaccounts
		err = kube.CopyServiceAccount("skupper", types.TransportServiceAccountName, map[string]string{}, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}
		annotationSubstitutions := map[string]string{
			"serviceaccounts.openshift.io/oauth-redirectreference.primary": "{\"kind\":\"OAuthRedirectReference\",\"apiVersion\":\"v1\",\"reference\":{\"kind\":\"Route\",\"name\":\"" + types.ConsoleRouteName + "\"}}",
		}
		err = kube.CopyServiceAccount("skupper-proxy-controller", types.ControllerServiceAccountName, annotationSubstitutions, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}

		// roles
		controllerRole := &rbacv1.Role{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "rbac.authorization.k8s.io/v1",
				Kind:       "Role",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:            types.ControllerRoleName,
				OwnerReferences: configmap.ObjectMeta.OwnerReferences,
			},
			Rules: types.ControllerPolicyRule,
		}
		_, err = kube.CreateRole(namespace, controllerRole, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}

		err = kube.CopyRole("skupper-view", types.TransportRoleName, namespace, cli.KubeClient)
		if err != nil && !errors.IsAlreadyExists(err) {
			return false, err
		}

		// rolebindings
		rolebindings := []rbacv1.RoleBinding{
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "RoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            types.ControllerRoleBindingName,
					OwnerReferences: configmap.ObjectMeta.OwnerReferences,
				},
				Subjects: []rbacv1.Subject{{
					Kind: "ServiceAccount",
					Name: types.ControllerServiceAccountName,
				}},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: types.ControllerRoleName,
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Kind:       "RoleBinding",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            types.TransportRoleBindingName,
					OwnerReferences: configmap.ObjectMeta.OwnerReferences,
				},
				Subjects: []rbacv1.Subject{{
					Kind: "ServiceAccount",
					Name: types.TransportServiceAccountName,
				}},
				RoleRef: rbacv1.RoleRef{
					Kind: "Role",
					Name: types.TransportRoleName,
				},
			},
		}
		for _, rolebinding := range rolebindings {
			_, err = kube.CreateRoleBinding(namespace, &rolebinding, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				return false, err
			}
		}

		if cli.RouteClient != nil {
			// routes: skupper-controller -> skupper
			original, err := cli.RouteClient.Routes(namespace).Get(context.TODO(), "skupper-controller", metav1.GetOptions{})
			if err == nil {
				route := &routev1.Route{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Route",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:            types.ConsoleRouteName,
						OwnerReferences: original.ObjectMeta.OwnerReferences,
					},
					Spec: routev1.RouteSpec{
						Path: original.Spec.Path,
						Port: original.Spec.Port,
						TLS:  original.Spec.TLS,
						To: routev1.RouteTargetReference{
							Kind: "Service",
							Name: types.ControllerServiceName,
						},
					},
				}
				_, err := cli.RouteClient.Routes(namespace).Create(context.TODO(), route, metav1.CreateOptions{})
				if err != nil && !errors.IsAlreadyExists(err) {
					return false, err
				}
			} else if !errors.IsNotFound(err) {
				return false, err
			}
			// need to update edge and inter-router routes to point at different service:
			err = kube.UpdateTargetServiceForRoute(types.EdgeRouteName, types.TransportServiceName, namespace, cli.RouteClient)
			if err != nil {
				return false, err
			}
			err = kube.UpdateTargetServiceForRoute(types.InterRouterRouteName, types.TransportServiceName, namespace, cli.RouteClient)
			if err != nil {
				return false, err
			}
		}
	}

	router, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	updateRouter := false
	if renameFor050 {
		// update deployment
		// - serviceaccount
		router.Spec.Template.Spec.ServiceAccountName = types.TransportServiceAccountName
		// - mounted secrets:
		kube.UpdateSecretVolume(&router.Spec.Template.Spec, "skupper-amqps", types.LocalServerSecret)
		kube.UpdateSecretVolume(&router.Spec.Template.Spec, "skupper-internal", types.SiteServerSecret)
		kube.UpdateSecretVolume(&router.Spec.Template.Spec, "skupper-proxy-certs", types.OauthRouterConsoleSecret)
		// -oauth proxy sidecar
		updateOauthProxyServiceAccount(&router.Spec.Template.Spec, types.TransportServiceAccountName)

		updateRouter = true
	}
	desiredRouterImage := images.GetRouterImageName()
	if router.Spec.Template.Spec.Containers[0].Image != desiredRouterImage {
		router.Spec.Template.Spec.Containers[0].Image = desiredRouterImage
		updateRouter = true
	}
	if kube.CheckProbesForTransportContainer(&router.Spec.Template.Spec.Containers[0]) {
		updateRouter = true
	}
	configSync := ConfigSyncContainer()
	if !hasContainer(configSync.Name, router.Spec.Template.Spec.Containers) {
		err = kube.UpdateRole(namespace, types.TransportRoleName, types.TransportPolicyRule, cli.KubeClient)
		if err != nil {
			return false, err
		}
		router.Spec.Template.Spec.Containers = append(router.Spec.Template.Spec.Containers, *configSync)
		updateRouter = true
	}
	if router.Spec.Template.Spec.Containers[1].Image != configSync.Image {
		router.Spec.Template.Spec.Containers[1].Image = configSync.Image
		updateRouter = true
	}
	if kube.CheckProbesForConfigSync(&router.Spec.Template.Spec.Containers[1]) {
		updateRouter = true
	}
	if renamedSkRouter {
		// Updating QDROUTERD_CONF env var
		envQdrouterdConf := kube.GetEnvVarForDeployment(router, "QDROUTERD_CONF")
		envQdrouterdConf = strings.ReplaceAll(envQdrouterdConf, "qpid-dispatch", "skupper-router")
		envQdrouterdConf = strings.ReplaceAll(envQdrouterdConf, "qdrouterd", "skrouterd")
		kube.SetEnvVarForDeployment(router, "QDROUTERD_CONF", envQdrouterdConf)

		// Updating volume mount paths
		for i, volume := range router.Spec.Template.Spec.Containers[0].VolumeMounts {
			volume.MountPath = strings.ReplaceAll(volume.MountPath, "qpid-dispatch", "skupper-router")
			router.Spec.Template.Spec.Containers[0].VolumeMounts[i] = volume
		}
		updateRouter = true
	}

	if addCertsSharedVolume {
		kube.AppendSharedVolume(&router.Spec.Template.Spec.Volumes, []*[]corev1.VolumeMount{&router.Spec.Template.Spec.Containers[0].VolumeMounts, &router.Spec.Template.Spec.Containers[1].VolumeMounts}, "skupper-router-certs", "/etc/skupper-router-certs")
		updateRouter = true
	}
	if addClaimsSupport {
		err = kube.UpdateRole(namespace, types.TransportRoleName, types.TransportPolicyRule, cli.KubeClient)
		if err != nil {
			return false, err
		}
		if !config.IsEdge() {
			if usingRoutes {
				err = cli.createClaimsRedemptionRoute(ctx, namespace)
				if err != nil {
					return false, err
				}
			}
		}
	}
	if moveClaims {
		if err := cli.moveClaimsToRouterService(ctx, namespace); err != nil {
			return false, err
		}
		latest, err := kube.WaitDeploymentReady(types.TransportDeploymentName, namespace, cli.KubeClient, time.Minute, time.Second)
		if err != nil {
			return false, err
		}
		if latest.ObjectMeta.ResourceVersion != router.ObjectMeta.ResourceVersion {
			latest.Spec = router.Spec
			router = latest
		}
		kube.AppendSecretVolumeWithVolumeName(&router.Spec.Template.Spec.Volumes, &router.Spec.Template.Spec.Containers[1].VolumeMounts, types.SiteServerSecret, "claims-cert", "/etc/skupper-internal/")
		updateRouter = true
	}

	if updateRouter || updateSite || hup {
		if !updateRouter {
			// need to trigger a router redployment to pick up the revised metadata field
			touch(router)
			updateRouter = true
		}
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), router, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		if routerExposedAsIp {
			fmt.Println("Sites previously linked to this one will require new tokens")
		}
	}

	controller, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), types.ControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	updateController := false
	if renameFor050 {
		// update deployment
		// - serviceaccount
		controller.Spec.Template.Spec.ServiceAccountName = types.ControllerServiceAccountName
		// - mounted secrets:
		kube.UpdateSecretVolume(&controller.Spec.Template.Spec, "skupper", types.LocalClientSecret)
		kube.UpdateSecretVolume(&controller.Spec.Template.Spec, "skupper-controller-certs", types.ConsoleServerSecret)
		// -oauth proxy sidecar
		updateOauthProxyServiceAccount(&controller.Spec.Template.Spec, types.ControllerServiceAccountName)
		updateController = true
	}
	if addMultiportServices {
		// disabling the controller
		controller, err = setAndWaitControllerReplicas(cli, 0, namespace)
		if err != nil {
			return false, err
		}
		if err = multiportConvertServices(ctx, cli, namespace); err != nil {
			return false, err
		}
		if err = updateGatewayMultiport(ctx, cli); err != nil {
			return false, err
		}
		updateController = true
	}
	// Add ClusterRoleBinding to allow reading SkupperClusterPolicies (otherwise policy will be disabled)
	if addClusterPolicy {
		siteConfig, _ := cli.SiteConfigInspect(ctx, nil)
		siteOwnerRef := asOwnerReference(siteConfig.Reference)
		var ownerRefs []metav1.OwnerReference
		if siteOwnerRef != nil {
			ownerRefs = []metav1.OwnerReference{*siteOwnerRef}
		}
		policyValidator := NewClusterPolicyValidator(cli)

		for _, clusterRole := range cli.ClusterRoles(siteConfig.Spec.EnableClusterPermissions) {
			// optional (in case of failure, cluster admin can add necessary cluster roles manually)
			kube.CreateOrExtendClusterRole(clusterRole, cli.KubeClient, siteConfig.Spec.EnableClusterPermissions)
		}
		for _, clusterRoleBinding := range ClusterRoleBindings(namespace) {
			clusterRoleBinding.ObjectMeta.OwnerReferences = ownerRefs
			_, err = kube.CreateClusterRoleBinding(clusterRoleBinding, cli.KubeClient)
			if err != nil && !errors.IsAlreadyExists(err) {
				if policyValidator.Enabled() {
					log.Printf("unable to define cluster role binding - %v", err)
					break
				}
			}
		}
	}

	if updateRouterPolicyRule {
		err = kube.UpdateRole(namespace, types.TransportRoleName, types.TransportPolicyRule, cli.KubeClient)
		if err != nil {
			return false, err
		}
	}

	if substituteFlowCollector {
		siteConfig, _ := cli.SiteConfigInspectInNamespace(ctx, nil, namespace)
		if siteConfig.Spec.EnableConsole {
			err = convertSiteConfigToCollectorEnabled(ctx, cli, namespace)
			if err != nil {
				return false, err
			}
			err = createNodeClusterRoleRule(ctx, cli, namespace)
			if err != nil {
				log.Printf("unable to update cluster role for nodes resource")
			}
			if siteConfig.Spec.AuthMode != string(types.ConsoleAuthModeOpenshift) {
				err = updateControllerPorts(ctx, cli, namespace)
				if err != nil {
					return false, err
				}
			}
			err = createFlowCollectorSidecar(ctx, cli, controller)
			if err != nil {
				return false, err
			}
			updateController = true
		}
	}

	if addPrometheusServer {
		siteConfig, _ := cli.SiteConfigInspectInNamespace(ctx, nil, namespace)
		if siteConfig.Spec.EnableFlowCollector && siteConfig.Spec.PrometheusServer.ExternalServer == "" {
			van := types.RouterSpec{
				Namespace: namespace,
			}
			cli.GetVanPrometheusServerSpec(siteConfig.Spec, &van)
			err := configureDeployment(&van.PrometheusServer, &siteConfig.Spec.Controller.Tuning)
			if err != nil {
				return false, err
			}
			err = cli.createPrometheus(ctx, siteConfig, van)
			if err != nil {
				return false, err
			}
		}
	}

	desiredControllerImage := images.GetServiceControllerImageName()
	if controller.Spec.Template.Spec.Containers[0].Image != desiredControllerImage {
		controller.Spec.Template.Spec.Containers[0].Image = desiredControllerImage
		updateController = true
	}
	desiredCollectorImage := images.GetFlowCollectorImageName()
	if len(controller.Spec.Template.Spec.Containers) > 1 {
		if controller.Spec.Template.Spec.Containers[1].Image != desiredCollectorImage {
			controller.Spec.Template.Spec.Containers[1].Image = desiredCollectorImage
			updateController = true
		}
	}
	if kube.CheckProbesForControllerContainer(&controller.Spec.Template.Spec.Containers[0]) {
		updateController = true
	}
	if updateController || hup {
		if !updateController {
			// trigger redeployment of service-controller to pick up latest image
			touch(controller)
			updateController = true
		}
		replicas := int32(1)
		controller.Spec.Replicas = &replicas
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), controller, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		if consoleUsesLoadbalancer {
			host := ""
			for i := 0; host == "" && i < 120; i++ {
				if i > 0 {
					time.Sleep(time.Second)
				}
				service, err := kube.GetService(types.ControllerServiceName, namespace, cli.KubeClient)
				if err != nil {
					fmt.Println("Could not determine new console url:", err.Error())
					break
				}
				host = kube.GetLoadBalancerHostOrIP(service)
			}
			if host != "" {
				fmt.Println("Console is now at", "http://"+host+":8080")
			}
		}
	}
	if renameFor050 {
		// delete old resources
		if cli.RouteClient != nil {
			err = cli.RouteClient.Routes(namespace).Delete(context.TODO(), "skupper-controller", metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}

		services := []string{
			"skupper-messaging",
			"skupper-controller",
		}
		if usingRoutes {
			// only delete skupper-internal if using
			// routes, as otherwise previously issued
			// tokens will reference it
			services = append(services, "skupper-internal")
		}
		for _, service := range services {
			err = cli.KubeClient.CoreV1().Services(namespace).Delete(context.TODO(), service, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}

		secrets := []string{
			"skupper",
			"skupper-amqps",
			"skupper-ca",
			"skupper-internal",
			"skupper-internal-ca",
		}
		for _, secret := range secrets {
			err = cli.KubeClient.CoreV1().Secrets(namespace).Delete(context.TODO(), secret, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}

		rolebindings := []string{
			"skupper-proxy-controller-skupper-edit",
			"skupper-skupper-view",
		}
		for _, rolebinding := range rolebindings {
			err = cli.KubeClient.RbacV1().RoleBindings(namespace).Delete(context.TODO(), rolebinding, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}
		serviceAccounts := []string{
			"skupper",
			"skupper-proxy-controller",
		}
		for _, serviceAccount := range serviceAccounts {
			err = cli.KubeClient.CoreV1().ServiceAccounts(namespace).Delete(context.TODO(), serviceAccount, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}
		roles := []string{
			"skupper-edit",
			"skupper-view",
		}
		for _, role := range roles {
			err = cli.KubeClient.RbacV1().Roles(namespace).Delete(context.TODO(), role, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return false, err
			}
		}
	}
	if inprogress {
		err = cli.updateCompleted(namespace)
		if err != nil {
			return true, err
		}
	}
	return updateRouter || updateController || updateSite, nil
}

func (cli *VanClient) renameRouterConfigFile(namespace string) (bool, error) {
	if namespace == "" {
		namespace = cli.Namespace
	}
	cm, err := kube.GetConfigMap(types.TransportConfigMapName, namespace, cli.KubeClient)
	if err != nil {
		return false, err
	}
	configFile, okOld := cm.Data["qdrouterd.json"]
	_, okNew := cm.Data[types.TransportConfigFile]
	// renaming
	if okOld && !okNew {
		updConfigFile := strings.ReplaceAll(configFile, "qpid-dispatch", "skupper-router")
		cm.Data[types.TransportConfigFile] = updConfigFile
		delete(cm.Data, "qdrouterd.json")
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	} else {
		return false, nil
	}
}

func setAndWaitControllerReplicas(cli *VanClient, replicas int32, namespace string) (*appsv1.Deployment, error) {
	controller, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), types.ControllerDeploymentName, metav1.GetOptions{})
	if *controller.Spec.Replicas > 0 {
		controller.Spec.Replicas = &replicas
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), controller, metav1.UpdateOptions{})
		controller, err = kube.WaitDeploymentReadyReplicas(types.ControllerDeploymentName, namespace, int(replicas), cli.KubeClient, time.Minute, time.Second)
		if err != nil {
			return controller, err
		}
	}
	return controller, err
}

func multiportConvertServices(ctx context.Context, cli *VanClient, namespace string) error {
	servicesCm, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, metav1.GetOptions{})
	if err != nil {
		return err
	}
	v1Svcs := []types.ServiceInterfaceV1{}
	for _, v := range servicesCm.Data {
		v1Svc := types.ServiceInterfaceV1{}
		err = json.Unmarshal([]byte(v), &v1Svc)
		if err != nil {
			return err
		}
		v1Svcs = append(v1Svcs, v1Svc)
	}
	outBytes, _ := json.Marshal(v1Svcs)
	defs := &types.ServiceInterfaceList{}
	err = defs.ConvertFrom(string(outBytes))
	if err != nil {
		return err
	}
	for _, svc := range *defs {
		svcBytes, _ := json.Marshal(svc)
		servicesCm.Data[svc.Address] = string(svcBytes)
		_, err = cli.KubeClient.CoreV1().ConfigMaps(namespace).Update(context.TODO(), servicesCm, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		servicesCm, _ = cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.TODO(), types.ServiceInterfaceConfigMap, metav1.GetOptions{})
	}

	return err
}

func updateGatewayMultiport(ctx context.Context, cli *VanClient) error {
	// retrieving all service definitions
	svcList, _ := cli.ServiceInterfaceList(ctx)
	svcMap := map[string]*types.ServiceInterface{}
	for _, svc := range svcList {
		svcMap[svc.Address] = svc
	}
	gwList, _ := cli.GatewayList(ctx)
	for _, gw := range gwList {
		// updating local gateways
		gatewayDir := getDataHome() + "/skupper/" + gw.Name
		newGatewayDir := getDataHome() + gatewayClusterDir + gw.Name
		// create the new base dir for gateways (and ignore errors if it already exists)
		_ = os.MkdirAll(getDataHome()+gatewayClusterDir, 0755)
		gd, err := os.Stat(gatewayDir)
		ngd, nerr := os.Stat(newGatewayDir)
		moveFiles := err == nil && gd != nil && gd.IsDir() && nerr != nil && ngd == nil
		if moveFiles {
			// renaming to new place
			err = os.Rename(gatewayDir, newGatewayDir)
			if err != nil {
				return err
			}
			// generate a router id and store it for subsequent template updates
			routerId := newUUID()
			err = ioutil.WriteFile(newGatewayDir+"/config/routerid.txt", []byte(routerId), 0644)
			if err != nil {
				return err
			}
			updateFileContent := func(fileName, oldPath, newPath string) error {
				content, err := ioutil.ReadFile(fileName)
				if err != nil {
					return err
				}
				updatedContent := strings.ReplaceAll(string(content), oldPath, newPath)
				err = ioutil.WriteFile(fileName, []byte(updatedContent), 0)
				if err != nil {
					return err
				}
				return nil
			}
			// Updating paths in service files
			err = updateFileContent(fmt.Sprintf("%s/user/%s.service", newGatewayDir, gw.Name), getDataHome()+"/skupper/", getDataHome()+gatewayClusterDir)
			if err != nil {
				return err
			}
			err = updateFileContent(getConfigHome()+"/systemd/user/"+gw.Name+".service", getDataHome()+"/skupper/", getDataHome()+gatewayClusterDir)
			if err != nil {
				return err
			}

			cmd := exec.Command("systemctl", "--user", "daemon-reload")
			err = cmd.Run()
			if err != nil {
				return fmt.Errorf("Unable to user service daemon-reload: %w", err)
			}
			cmd = exec.Command("systemctl", "--user", "restart", gw.Name+".service")
			err = cmd.Run()
			if err != nil {
				return fmt.Errorf("Unable to user service restart: %w", err)
			}
		}

		// updating router config to fix bad template issues
		configmap, err := kube.GetConfigMap(gatewayPrefix+gw.Name, cli.GetNamespace(), cli.KubeClient)
		if err != nil {
			return err
		}
		gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			return err
		}
		// updating version
		sm := qdr.SiteMetadata{}
		err = json.Unmarshal([]byte(gatewayConfig.Metadata.Metadata), &sm)
		if err != nil {
			return err
		}
		sm.Version = version.Version
		smStr, err := json.Marshal(sm)
		if err != nil {
			return err
		}
		gatewayConfig.Metadata.Metadata = string(smStr)
		// updating tcp listeners
		newTcpListeners := qdr.TcpEndpointMap{}
		for k, v := range gatewayConfig.Bridges.TcpListeners {
			name := fmt.Sprintf("%s:%d", k, svcMap[v.Address].Ports[0])
			v.Name = name
			v.Address = fmt.Sprintf("%s:%d", v.Address, svcMap[v.Address].Ports[0])
			newTcpListeners[name] = v
		}
		gatewayConfig.Bridges.TcpListeners = newTcpListeners
		// updating tcp connectors
		newTcpConnectors := qdr.TcpEndpointMap{}
		for k, v := range gatewayConfig.Bridges.TcpConnectors {
			name := fmt.Sprintf("%s:%d", k, svcMap[v.Address].Ports[0])
			v.Name = name
			v.Address = fmt.Sprintf("%s:%d", v.Address, svcMap[v.Address].Ports[0])
			newTcpConnectors[name] = v
		}
		gatewayConfig.Bridges.TcpConnectors = newTcpConnectors
		// updating http listeners
		newHttpListeners := qdr.HttpEndpointMap{}
		for k, v := range gatewayConfig.Bridges.HttpListeners {
			name := fmt.Sprintf("%s:%d", k, svcMap[v.Address].Ports[0])
			v.Name = name
			v.Address = fmt.Sprintf("%s:%d", v.Address, svcMap[v.Address].Ports[0])
			newHttpListeners[name] = v
		}
		gatewayConfig.Bridges.HttpListeners = newHttpListeners
		// updating tcp connectors
		newHttpConnectors := qdr.HttpEndpointMap{}
		for k, v := range gatewayConfig.Bridges.HttpConnectors {
			name := fmt.Sprintf("%s:%d", k, svcMap[v.Address].Ports[0])
			v.Name = name
			v.Address = fmt.Sprintf("%s:%d", v.Address, svcMap[v.Address].Ports[0])
			newHttpConnectors[name] = v
		}
		gatewayConfig.Bridges.HttpConnectors = newHttpConnectors

		// updating configmap
		_ = gatewayConfig.WriteToConfigMap(configmap)
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(ctx, configmap, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("Failed to update gateway config map: %s", err)
		}
		if err != nil {
			return err
		}

		// if it is a local gateway
		_, err = os.Stat(newGatewayDir + "/config/qdrouterd.json")
		if err == nil {
			// for update gatewayType would be "service"
			err = updateLocalGatewayConfig(newGatewayDir, "service", *gatewayConfig)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (cli *VanClient) restartRouter(namespace string) error {
	router, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	touch(router)
	_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), router, metav1.UpdateOptions{})
	return err
}

func (cli *VanClient) RouterUpdateLogging(ctx context.Context, settings *corev1.ConfigMap, hup bool) (bool, error) {
	siteConfig, err := cli.SiteConfigInspect(ctx, settings)
	if err != nil {
		return false, err
	}
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(settings.ObjectMeta.Namespace).Get(context.TODO(), types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return false, err
	}
	updated := qdr.ConfigureRouterLogging(routerConfig, siteConfig.Spec.Router.Logging)
	if updated {
		routerConfig.WriteToConfigMap(configmap)
		_, err = cli.KubeClient.CoreV1().ConfigMaps(settings.ObjectMeta.Namespace).Update(context.TODO(), configmap, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		if hup {
			err = cli.restartRouter(settings.ObjectMeta.Namespace)
			if err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

func (cli *VanClient) RouterUpdateDebugMode(ctx context.Context, settings *corev1.ConfigMap) (bool, error) {
	siteConfig, err := cli.SiteConfigInspect(ctx, settings)
	if err != nil {
		return false, err
	}
	router, err := cli.KubeClient.AppsV1().Deployments(settings.ObjectMeta.Namespace).Get(context.TODO(), types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	current := kube.GetEnvVarForDeployment(router, "QDROUTERD_DEBUG")
	if current == siteConfig.Spec.Router.DebugMode {
		return false, nil
	}
	if siteConfig.Spec.Router.DebugMode == "" {
		kube.DeleteEnvVarForDeployment(router, "QDROUTERD_DEBUG")
	} else {
		kube.SetEnvVarForDeployment(router, "QDROUTERD_DEBUG", siteConfig.Spec.Router.DebugMode)
	}
	_, err = cli.KubeClient.AppsV1().Deployments(settings.ObjectMeta.Namespace).Update(context.TODO(), router, metav1.UpdateOptions{})
	if err != nil {
		return false, err
	}
	return true, nil

}

func (cli *VanClient) updateAnnotationsOnDeployment(ctx context.Context, namespace string, name string, annotations map[string]string) (bool, error) {
	deployment, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(annotations, deployment.Spec.Template.ObjectMeta.Annotations) {
		deployment.Spec.Template.ObjectMeta.Annotations = annotations
		_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (cli *VanClient) RouterUpdateAnnotations(ctx context.Context, settings *corev1.ConfigMap) (bool, error) {
	siteConfig, err := cli.SiteConfigInspect(ctx, settings)
	if err != nil {
		return false, err
	}

	controllerAnnotations := map[string]string{}
	for key, value := range siteConfig.Spec.Annotations {
		controllerAnnotations[key] = value
	}
	for key, value := range siteConfig.Spec.Controller.PodAnnotations {
		controllerAnnotations[key] = value
	}
	updated, err := cli.updateAnnotationsOnDeployment(ctx, settings.ObjectMeta.Namespace, types.ControllerDeploymentName, controllerAnnotations)
	if err != nil {
		return updated, err
	}

	transportAnnotations := map[string]string{}
	for key, value := range types.TransportPrometheusAnnotations {
		transportAnnotations[key] = value
	}
	for key, value := range siteConfig.Spec.Annotations {
		transportAnnotations[key] = value
	}
	for key, value := range siteConfig.Spec.Router.PodAnnotations {
		transportAnnotations[key] = value
	}
	updated, err = cli.updateAnnotationsOnDeployment(ctx, settings.ObjectMeta.Namespace, types.TransportDeploymentName, transportAnnotations)
	if err != nil {
		return updated, err
	}

	prometheusAnnotations := map[string]string{}
	for key, value := range siteConfig.Spec.Annotations {
		prometheusAnnotations[key] = value
	}
	for key, value := range siteConfig.Spec.PrometheusServer.PodAnnotations {
		prometheusAnnotations[key] = value
	}
	updated, err = cli.updateAnnotationsOnDeployment(ctx, settings.ObjectMeta.Namespace, types.PrometheusDeploymentName, prometheusAnnotations)
	if err != nil {
		return updated, err
	}

	return updated, nil
}

func (cli *VanClient) RouterRestart(ctx context.Context, namespace string) error {
	router, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(ctx, types.TransportDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	touch(router)
	_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(ctx, router, metav1.UpdateOptions{})
	return err
}

func touch(deployment *appsv1.Deployment) {
	if deployment.Spec.Template.ObjectMeta.Annotations == nil {
		deployment.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	deployment.Spec.Template.ObjectMeta.Annotations[types.UpdatedAnnotation] = time.Now().Format(time.RFC1123Z)

}

func updateOauthProxyServiceAccount(spec *corev1.PodSpec, name string) {
	if len(spec.Containers) > 1 && spec.Containers[1].Name == "oauth-proxy" {
		for i, arg := range spec.Containers[1].Args {
			if strings.HasPrefix(arg, "--openshift-service-account") {
				spec.Containers[1].Args[i] = "--openshift-service-account=" + name
			}
		}
	}
}

func (cli *VanClient) usingRoutes(namespace string) (bool, error) {
	if cli.RouteClient != nil {
		_, err := kube.GetRoute(types.InterRouterRouteName, namespace, cli.RouteClient)
		if err == nil {
			return true, nil
		} else if errors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	} else {
		return false, nil
	}
}

func (cli *VanClient) getTransportHosts(namespace string) ([]string, error) {
	hosts := []string{}
	oldService, err := kube.GetService("skupper-internal", namespace, cli.KubeClient)
	if err != nil {
		return nil, err
	}
	if oldService.Spec.Type == corev1.ServiceTypeLoadBalancer {
		host := ""
		for i := 0; i < 120; i++ {
			if i > 0 {
				time.Sleep(time.Second)
			}
			service, err := kube.GetService(types.TransportServiceName, namespace, cli.KubeClient)
			if err != nil {
				return nil, err
			}
			host = kube.GetLoadBalancerHostOrIP(service)
			if host != "" {
				hosts = append(hosts, host)
				break
			}
		}
		host = kube.GetLoadBalancerHostOrIP(oldService)
		if host != "" {
			hosts = append(hosts, host)
		}
	}
	hosts = append(hosts, types.TransportServiceName)
	hosts = append(hosts, types.QualifiedServiceName(types.TransportServiceName, namespace))
	hosts = append(hosts, types.QualifiedServiceName("skupper-internal", namespace))
	return hosts, nil
}

func (cli *VanClient) addClaimsPortsToRouterService(ctx context.Context, namespace string) error {
	svc, err := cli.KubeClient.CoreV1().Services(namespace).Get(ctx, types.TransportServiceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	found := false
	for _, port := range svc.Spec.Ports {
		if port.Name == types.ClaimRedemptionPortName {
			found = true
		}
	}
	if found {
		return nil
	}
	svc.Spec.Ports = append(svc.Spec.Ports, corev1.ServicePort{
		Name:     types.ClaimRedemptionPortName,
		Protocol: "TCP",
		Port:     types.ClaimRedemptionPort,
	})
	_, err = cli.KubeClient.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (cli *VanClient) removeClaimsPortsFromControllerService(ctx context.Context, namespace string) error {
	svc, err := cli.KubeClient.CoreV1().Services(namespace).Get(ctx, types.ControllerServiceName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}
	found := false
	var ports []corev1.ServicePort
	for _, port := range svc.Spec.Ports {
		if port.Name == types.ClaimRedemptionPortName {
			found = true
		} else {
			ports = append(ports, port)
		}
	}
	if !found {
		return nil
	}
	if len(ports) > 0 {
		svc.Spec.Ports = ports
		_, err = cli.KubeClient.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	} else {
		err = cli.KubeClient.CoreV1().Services(namespace).Delete(ctx, types.ControllerServiceName, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func (cli *VanClient) updateIngressResources(ctx context.Context, namespace string) error {
	if err := kube.UpdateTargetServiceForRouteIfExists(types.ClaimRedemptionRouteName, types.TransportServiceName, namespace, cli.RouteClient); err != nil {
		return err
	}
	if err := kube.UpdateIngressRuleServiceName(types.IngressName, "claims", types.TransportServiceName, namespace, cli); err != nil {
		return err
	}
	if err := kube.UpdateContourProxyService(cli, namespace, types.ClaimsIngressPrefix, types.TransportServiceName); err != nil {
		return err
	}
	return nil
}

func (cli *VanClient) moveClaimsToRouterService(ctx context.Context, namespace string) error {
	if err := cli.addClaimsPortsToRouterService(ctx, namespace); err != nil {
		return err
	}
	if err := cli.removeClaimsPortsFromControllerService(ctx, namespace); err != nil {
		return err
	}
	if err := cli.updateIngressResources(ctx, namespace); err != nil {
		return err
	}
	ca, err := cli.KubeClient.CoreV1().Secrets(namespace).Get(ctx, types.SiteCaSecret, metav1.GetOptions{})
	if err != nil {
		return err
	}
	err = cli.regenerateSiteSecret(ctx, ca)
	if err != nil {
		return err
	}
	return nil
}

func (cli *VanClient) createClaimsRedemptionRoute(ctx context.Context, namespace string) error {
	route := &routev1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: types.ClaimRedemptionRouteName,
		},
		Spec: routev1.RouteSpec{
			Path: "",
			Port: &routev1.RoutePort{
				TargetPort: intstr.FromString(types.ClaimRedemptionPortName),
			},
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: types.TransportServiceName,
			},
			TLS: &routev1.TLSConfig{
				Termination:                   routev1.TLSTerminationPassthrough,
				InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
			},
		},
	}
	_, err := kube.CreateRoute(route, namespace, cli.RouteClient)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (cli *VanClient) restartController(namespace string) error {
	controller, err := cli.KubeClient.AppsV1().Deployments(namespace).Get(context.TODO(), types.ControllerDeploymentName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	touch(controller)
	_, err = cli.KubeClient.AppsV1().Deployments(namespace).Update(context.TODO(), controller, metav1.UpdateOptions{})
	return err
}

func (cli *VanClient) GetSiteMetadata() (*qdr.SiteMetadata, error) {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(cli.Namespace).Get(context.TODO(), types.TransportConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	config, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, err
	}
	metadata := config.GetSiteMetadata()
	return &metadata, nil
}

func hasContainer(name string, containers []corev1.Container) bool {
	for _, c := range containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

func convertSiteConfigToCollectorEnabled(ctx context.Context, cli *VanClient, namespace string) error {
	configmap, err := cli.KubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, types.SiteConfigMapName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	site.UpdateForCollectorEnabled(configmap)
	_, err = cli.KubeClient.CoreV1().ConfigMaps(namespace).Update(ctx, configmap, metav1.UpdateOptions{})
	return err
}

func createNodeClusterRoleRule(ctx context.Context, cli *VanClient, namespace string) error {
	clusterRole, err := cli.KubeClient.RbacV1().ClusterRoles().Get(ctx, types.ControllerClusterRoleName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	clusterRole.Rules = append(clusterRole.Rules, rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"nodes"},
		Verbs:     []string{"get", "list", "watch"},
	})
	_, err = cli.KubeClient.RbacV1().ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
	return err
}

func updateControllerPorts(ctx context.Context, cli *VanClient, namespace string) error {
	svc, err := cli.KubeClient.CoreV1().Services(namespace).Get(ctx, types.ConsoleRouteName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for i, port := range svc.Spec.Ports {
		if port.Name == "metrics" {
			svc.Spec.Ports[i].Port = types.FlowCollectorDefaultServicePort
			svc.Spec.Ports[i].TargetPort = intstr.FromInt(int(types.FlowCollectorDefaultServiceTargetPort))
		}
	}
	_, err = cli.KubeClient.CoreV1().Services(namespace).Update(ctx, svc, metav1.UpdateOptions{})
	return err
}

func createFlowCollectorSidecar(ctx context.Context, cli *VanClient, controller *appsv1.Deployment) error {
	for i, env := range controller.Spec.Template.Spec.Containers[0].Env {
		parts := strings.Split(env.Name, "_")
		if parts[0] == "METRICS" {
			parts[0] = "FLOW"
			controller.Spec.Template.Spec.Containers[0].Env[i].Name = strings.Join(parts, "_")
		}
	}
	flowContainer := controller.Spec.Template.Spec.Containers[0]
	flowContainer.Image = images.GetFlowCollectorImageName()
	flowContainer.Name = types.FlowCollectorContainerName
	controller.Spec.Template.Spec.Containers = append(controller.Spec.Template.Spec.Containers, flowContainer)
	return nil
}
