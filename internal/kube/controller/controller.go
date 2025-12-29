package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers/internalinterfaces"
	"k8s.io/client-go/tools/cache"

	"github.com/skupperproject/skupper/internal/kube/certificates"
	internalclient "github.com/skupperproject/skupper/internal/kube/client"
	"github.com/skupperproject/skupper/internal/kube/grants"
	"github.com/skupperproject/skupper/internal/kube/securedaccess"
	"github.com/skupperproject/skupper/internal/kube/site"
	"github.com/skupperproject/skupper/internal/kube/site/labels"
	"github.com/skupperproject/skupper/internal/kube/site/sizing"
	"github.com/skupperproject/skupper/internal/kube/watchers"
	"github.com/skupperproject/skupper/internal/network"
	"github.com/skupperproject/skupper/internal/qdr"
	"github.com/skupperproject/skupper/internal/version"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
)

type Controller struct {
	self                 skupperv2alpha1.Controller
	deploymentName       string
	deploymentUid        string
	eventProcessor       *watchers.EventProcessor
	stopCh               <-chan struct{}
	siteWatcher          *watchers.SiteWatcher
	listenerWatcher      *watchers.ListenerWatcher
	connectorWatcher     *watchers.ConnectorWatcher
	linkAccessWatcher    *watchers.RouterAccessWatcher
	grantWatcher         *watchers.AccessGrantWatcher
	serviceWatcher       *watchers.ServiceWatcher
	sites                map[string]*site.Site
	startGrantServer     func()
	accessMgr            *securedaccess.SecuredAccessManager
	accessRecovery       *securedaccess.SecuredAccessResourceWatcher
	certMgr              *certificates.CertificateManagerImpl
	siteSizing           *sizing.Registry
	siteSizingWatcher    *watchers.ConfigMapWatcher
	labelling            *labels.LabelsAndAnnotations
	labellingWatcher     *watchers.ConfigMapWatcher
	attachableConnectors map[string]*skupperv2alpha1.AttachedConnector
	disableSecContext    bool
	log                  *slog.Logger
	namespaces           *NamespaceConfig
	observedServices     map[string]string
}

func skupperRouterConfig() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/router-config"
	}
}
func skupperNetworkStatus() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-network-status"
	}
}

func listenerServices() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "internal.skupper.io/listener"
	}
}

func sansSkupperListenerServices() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "!internal.skupper.io/listener"
	}
}

func skupperSiteSizingConfig() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = sizing.SiteSizingLabel
	}
}

func labelling() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.LabelSelector = "skupper.io/label-template"
	}
}

func NewController(cli internalclient.Clients, config *Config, options ...watchers.EventProcessorCustomizer) (*Controller, error) {
	controller := &Controller{
		eventProcessor:       watchers.NewEventProcessor("Controller", cli, options...),
		sites:                map[string]*site.Site{},
		siteSizing:           sizing.NewRegistry(),
		labelling:            labels.NewLabelsAndAnnotations(config.Namespace),
		attachableConnectors: map[string]*skupperv2alpha1.AttachedConnector{},
		log:                  slog.New(slog.Default().Handler()).With(slog.String("component", "kube.controller")),
		observedServices:     map[string]string{},
		disableSecContext:    config.DisableSecurityContext,
	}

	hostname := os.Getenv("HOSTNAME")
	owner, err := controller.getDeploymentForPod(hostname, config.Namespace)
	name := config.Name
	if err != nil {
		controller.log.Info("Could not deduce owning deployment, some resources may need to be manually deleted when controller is deleted", slog.Any("error", err))
		if name == "" {
			if hostname != "" {
				name = hostname
			} else {
				name = "skupper-controller"
			}
		}
	} else {
		controller.deploymentName = owner.Name
		controller.deploymentUid = string(owner.UID)
		if name == "" {
			name = owner.Name
		}
	}
	controller.namespaces = newNamespaceConfig(config.Namespace+"/"+name, config.requireExplicitControl(), newControlLogging(config.WatchingAllNamespaces(), controller.log))
	controller.self.Name = name
	controller.self.Namespace = config.Namespace
	controller.self.Version = version.Version

	controller.siteWatcher = controller.eventProcessor.WatchSites(config.WatchNamespace, filter(controller, controller.checkSite))
	controller.listenerWatcher = controller.eventProcessor.WatchListeners(config.WatchNamespace, filter(controller, controller.checkListener))
	controller.eventProcessor.WatchServices(listenerServices(), config.WatchNamespace, filter(controller, controller.checkListenerService))
	controller.serviceWatcher = controller.eventProcessor.WatchServices(sansSkupperListenerServices(), config.WatchNamespace, filter(controller, controller.checkObservedService))
	controller.connectorWatcher = controller.eventProcessor.WatchConnectors(config.WatchNamespace, filter(controller, controller.checkConnector))
	controller.linkAccessWatcher = controller.eventProcessor.WatchRouterAccesses(config.WatchNamespace, filter(controller, controller.checkRouterAccess))
	controller.eventProcessor.WatchAttachedConnectors(config.WatchNamespace, filter(controller, controller.checkAttachedConnector))
	controller.eventProcessor.WatchAttachedConnectorBindings(config.WatchNamespace, filter(controller, controller.checkAttachedConnectorBinding))
	controller.eventProcessor.WatchLinks(config.WatchNamespace, filter(controller, controller.checkLink))
	controller.eventProcessor.WatchConfigMaps(skupperNetworkStatus(), config.WatchNamespace, filter(controller, controller.networkStatusUpdate))
	controller.eventProcessor.WatchConfigMaps(skupperRouterConfig(), config.WatchNamespace, filter(controller, controller.routerConfigUpdate))
	controller.eventProcessor.WatchAccessTokens(config.WatchNamespace, filter(controller, controller.checkAccessToken))
	controller.eventProcessor.WatchPods("skupper.io/component=router,skupper.io/type=site", config.WatchNamespace, filter(controller, controller.routerPodEvent))
	controller.siteSizingWatcher = controller.eventProcessor.WatchConfigMaps(skupperSiteSizingConfig(), config.Namespace, filter(controller, controller.siteSizing.Update))
	controller.namespaces.watch(controller.eventProcessor, config.WatchNamespace)
	controller.labellingWatcher = controller.eventProcessor.WatchConfigMaps(labelling(), config.WatchNamespace, controller.labelling.Update)

	controller.certMgr = certificates.NewCertificateManager(controller.eventProcessor)
	controller.certMgr.SetControllerContext(controller)
	controller.certMgr.Watch(config.WatchNamespace)

	controller.accessMgr = securedaccess.NewSecuredAccessManager(controller.eventProcessor, controller.certMgr, config.SecuredAccessConfig, controller)
	controller.accessRecovery = securedaccess.NewSecuredAccessResourceWatcher(controller.accessMgr)
	controller.accessRecovery.WatchResources(controller.eventProcessor, config.WatchNamespace)
	controller.accessRecovery.WatchSecuredAccesses(controller.eventProcessor, config.WatchNamespace, controller.checkSecuredAccess)
	controller.accessRecovery.WatchGateway(controller.eventProcessor, config.Namespace)

	controller.startGrantServer = grants.Initialise(controller.eventProcessor, config.Namespace, config.WatchNamespace, config.GrantConfig, controller.generateLinkConfig, controller.IsControlled)

	controller.eventProcessor.WatchConfigMaps(skupperLogConfig(), config.Namespace, controller.logConfigUpdate)

	return controller, nil
}

func (c *Controller) IsControlled(namespace string) bool {
	return c.namespaces.isControlled(namespace)
}

func (c *Controller) SetLabels(namespace string, name string, kind string, labels map[string]string) bool {
	return c.labelling.SetLabels(namespace, name, kind, labels)
}

func (c *Controller) SetAnnotations(namespace string, name string, kind string, annotations map[string]string) bool {
	return c.labelling.SetAnnotations(namespace, name, kind, annotations)
}

func (c *Controller) SetObjectMetadata(namespace string, name string, kind string, meta *metav1.ObjectMeta) bool {
	return c.labelling.SetObjectMetadata(namespace, name, kind, meta)
}

func (c *Controller) Namespace() string {
	return c.self.Namespace
}

func (c *Controller) Name() string {
	return c.deploymentName
}

func (c *Controller) UID() string {
	return c.deploymentUid
}

func (c *Controller) getDeploymentForPod(podName string, namespace string) (*appsv1.Deployment, error) {
	re := regexp.MustCompile(`^(\S+)\-[a-z0-9]{9,10}\-[a-z0-9]{5}$`)
	matches := re.FindStringSubmatch(podName)
	if len(matches) != 2 {
		return nil, fmt.Errorf("Could not determine deployment name from %s", podName)
	}
	deploymentName := matches[1]
	deployment, err := c.eventProcessor.GetKubeClient().AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve controller deployment for %s/%s: %s", namespace, deploymentName, err)
	}
	return deployment, nil
}

func (c *Controller) Run(stopCh <-chan struct{}) error {
	if err := c.init(stopCh); err != nil {
		return err
	}
	return c.start(stopCh)
}

func (c *Controller) init(stopCh <-chan struct{}) error {
	c.log.Info("Starting informers")
	c.eventProcessor.StartWatchers(stopCh)
	c.stopCh = stopCh

	c.log.Info("Waiting for informer caches to sync")
	if ok := c.eventProcessor.WaitForCacheSync(stopCh); !ok {
		return fmt.Errorf("Failed to wait for caches to sync")
	}
	c.namespaces.recover()

	for _, config := range c.siteSizingWatcher.List() {
		c.log.Info("Recovering site sizing",
			slog.String("namespace", config.Namespace),
			slog.String("name", config.Name),
		)
		c.siteSizing.Update(config.Namespace+"/"+config.Name, config)
	}
	for _, config := range c.labellingWatcher.List() {
		c.log.Info("Recovering label and annotation configuration",
			slog.String("namespace", config.Namespace),
			slog.String("name", config.Name),
		)
		c.labelling.Update(config.Namespace+"/"+config.Name, config)
	}
	// get observed services prior to restoring listeners
	for _, svc := range c.serviceWatcher.List() {
		c.observedServices[svc.Namespace+"/"+svc.ObjectMeta.Name] = svc.ObjectMeta.Name
	}
	//recover existing sites & bindings
	siteRecovery := site.NewSiteRecovery(c.eventProcessor.GetKubeClient())
	for _, site := range c.siteWatcher.List() {
		if !c.namespaces.isControlled(site.Namespace) {
			continue
		}
		if !siteRecovery.IsActive(site) {
			c.log.Info("Skipping site recovery as it is not the active site",
				slog.String("namespace", site.Namespace),
				slog.String("name", site.Name),
			)
			continue
		}
		c.log.Info("Recovering site",
			slog.String("namespace", site.Namespace),
			slog.String("name", site.Name),
		)
		err := c.getSite(site.ObjectMeta.Namespace).StartRecovery(site)
		if err != nil {
			c.log.Error("Error recovering site",
				slog.String("namespace", site.Namespace),
				slog.String("name", site.Name),
				slog.Any("error", err),
			)
		}
	}
	for _, connector := range c.connectorWatcher.List() {
		if !c.namespaces.isControlled(connector.Namespace) {
			continue
		}
		site := c.getSite(connector.ObjectMeta.Namespace)
		c.log.Info("Recovering connector",
			slog.String("namespace", connector.Namespace),
			slog.String("name", connector.Name),
		)
		site.CheckConnector(connector.ObjectMeta.Name, connector)
	}
	for _, listener := range c.listenerWatcher.List() {
		if !c.namespaces.isControlled(listener.Namespace) {
			continue
		}
		site := c.getSite(listener.ObjectMeta.Namespace)
		c.log.Info("Recovering listener",
			slog.String("namespace", listener.Namespace),
			slog.String("name", listener.Name),
		)
		_, svcExists := c.observedServices[listener.ObjectMeta.Namespace+"/"+listener.Spec.Host]
		site.CheckListener(listener.ObjectMeta.Name, listener, svcExists)
	}
	for _, la := range c.linkAccessWatcher.List() {
		if !c.namespaces.isControlled(la.Namespace) {
			continue
		}
		site := c.getSite(la.ObjectMeta.Namespace)
		c.log.Info("Recovering router access",
			slog.String("namespace", la.Namespace),
			slog.String("name", la.Name),
		)
		site.CheckRouterAccess(la.ObjectMeta.Name, la)
	}
	for _, site := range c.siteWatcher.List() {
		if !c.namespaces.isControlled(site.Namespace) {
			continue
		}
		site.Status.Controller = &c.self
		err := c.getSite(site.ObjectMeta.Namespace).Reconcile(site)
		if err != nil {
			c.log.Error("Error recovering site",
				slog.String("namespace", site.Namespace),
				slog.String("name", site.Name),
				slog.Any("error", err),
			)
		} else {
			c.log.Info("Recovered site",
				slog.String("namespace", site.Namespace),
				slog.String("name", site.Name),
			)
		}
	}
	c.certMgr.Recover()
	c.accessRecovery.Recover()
	if c.startGrantServer != nil {
		c.startGrantServer()
	}
	return nil
}

func (c *Controller) start(stopCh <-chan struct{}) error {
	c.log.Info("Starting event loop")
	c.eventProcessor.Start(stopCh)
	<-stopCh
	c.log.Info("Shutting down")
	return nil
}

func (c *Controller) getSite(namespace string) *site.Site {
	if existing, ok := c.sites[namespace]; ok {
		return existing
	}
	site := site.NewSite(namespace, c.eventProcessor, c.certMgr, c.accessMgr, c.siteSizing, c, c.disableSecContext)
	c.sites[namespace] = site
	return site
}

func (c *Controller) checkSite(key string, site *skupperv2alpha1.Site) error {
	c.log.Debug("checkSite", slog.String("key", key))
	if site != nil {
		if !c.namespaces.isControlled(site.Namespace) {
			c.log.Info("Ignoring site as it not controlled by this controller", slog.String("key", key))
			return nil
		}
		site.Status.Controller = &c.self
		err := c.getSite(site.ObjectMeta.Namespace).Reconcile(site)
		if err != nil {
			c.log.Info("Error initialising site",
				slog.String("key", key),
				slog.Any("error", err),
			)
		}
	} else {
		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			return err
		}
		s := c.getSite(namespace)
		if s.NameMatches(name) {
			s.Deleted()
			delete(c.sites, namespace)
		}
	}
	return nil
}

func (c *Controller) checkConnector(key string, connector *skupperv2alpha1.Connector) error {
	c.log.Debug("checkConnector", slog.String("key", key))
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckConnector(name, connector)
}

func (c *Controller) checkListener(key string, listener *skupperv2alpha1.Listener) error {
	c.log.Debug("checkListener", slog.String("key", key))
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	svcExists := false
	if listener != nil {
		_, svcExists = c.observedServices[namespace+"/"+listener.Spec.Host]
	}
	return c.getSite(namespace).CheckListener(name, listener, svcExists)
}

func (c *Controller) checkListenerService(key string, svc *corev1.Service) error {
	c.log.Debug("checkListenerService", slog.String("key", key))
	if svc == nil {
		return nil
	}
	return c.getSite(svc.Namespace).CheckListenerService(svc)
}

func (c *Controller) checkObservedService(key string, svc *corev1.Service) error {
	c.log.Debug("checkObservedService", slog.String("key", key))

	if svc == nil {
		delete(c.observedServices, key)
	} else {
		c.observedServices[key] = svc.ObjectMeta.Name
	}
	return nil
}

func (c *Controller) checkLink(key string, linkconfig *skupperv2alpha1.Link) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckLink(name, linkconfig)
}

func (c *Controller) checkAccessToken(key string, token *skupperv2alpha1.AccessToken) error {
	if token == nil || token.IsRedeemed() {
		return nil
	}
	site := c.getSite(token.Namespace).GetSite()
	if site == nil {
		return nil
	}
	return grants.RedeemAccessToken(token, site, c.eventProcessor)
}

func (c *Controller) routerPodEvent(key string, pod *corev1.Pod) error {
	namespace, _, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).RouterPodEvent(key, pod)
}

func (c *Controller) generateLinkConfig(namespace string, name string, subject string, writer io.Writer) error {
	site := c.getSite(namespace).GetSite()
	if site == nil {
		return fmt.Errorf("Site not yet defined for %s", namespace)
	}
	generator, err := grants.NewTokenGenerator(site, c.eventProcessor)
	if err != nil {
		return err
	}
	token, err := generator.NewCertToken(name, subject)
	if err != nil {
		return err
	}
	return token.Write(writer)
}

func (c *Controller) checkSecuredAccess(key string, se *skupperv2alpha1.SecuredAccess) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckSecuredAccess(name, se)
}

func (c *Controller) checkRouterAccess(key string, ra *skupperv2alpha1.RouterAccess) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckRouterAccess(name, ra)
}

func (c *Controller) checkAttachedConnectorBinding(key string, binding *skupperv2alpha1.AttachedConnectorBinding) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.getSite(namespace).CheckAttachedConnectorBinding(namespace, name, binding)
}

func (c *Controller) checkAttachedConnector(key string, connector *skupperv2alpha1.AttachedConnector) error {
	if connector == nil {
		if previous, ok := c.attachableConnectors[key]; ok {
			c.log.Info("AttachedConnector deleted", slog.String("key", key))
			delete(c.attachableConnectors, key)
			return c.getSite(previous.Spec.SiteNamespace).AttachedConnectorDeleted(previous.Namespace, previous.Name)
		} else {
			return nil
		}
	} else {
		if previous, ok := c.attachableConnectors[key]; ok {
			if previous.Spec.SiteNamespace != connector.Spec.SiteNamespace {
				c.log.Info("AttachedConnector site namespace has changed",
					slog.String("key", key),
					slog.String("from", previous.Spec.SiteNamespace),
					slog.String("to", connector.Spec.SiteNamespace),
				)
				err := c.getSite(previous.Spec.SiteNamespace).AttachedConnectorUnreferenced(previous)
				if err != nil {
					c.log.Error("Error removing AttachedConnector reference from previous namespace",
						slog.String("key", key),
						slog.String("previous", previous.Spec.SiteNamespace))
				}
			}
		}
		c.attachableConnectors[key] = connector
		return c.getSite(connector.Spec.SiteNamespace).AttachedConnectorUpdated(connector)
	}
}
func (c *Controller) routerConfigUpdate(_ string, cm *corev1.ConfigMap) error {
	if cm == nil {
		return nil
	}
	config, err := qdr.GetRouterConfigFromConfigMap(cm)
	if err != nil {
		return err
	}
	c.getSite(cm.Namespace).CheckSslProfiles(config)
	return nil
}

func (c *Controller) networkStatusUpdate(key string, cm *corev1.ConfigMap) error {
	if cm == nil {
		return nil
	}
	encoded := cm.Data["NetworkStatus"]
	if encoded == "" {
		c.log.Info("No network status found", slog.String("site", key))
		return nil
	}
	var status network.NetworkStatusInfo
	err := json.Unmarshal([]byte(encoded), &status)
	if err != nil {
		c.log.Error("Error unmarshalling network status", slog.String("site", key), slog.Any("error", err))
		return nil
	}
	c.log.Debug("Updating network status", slog.String("site", key))
	return c.getSite(cm.ObjectMeta.Namespace).NetworkStatusUpdated(network.ExtractSiteRecords(status))
}

func filter[V any](controller *Controller, handler func(string, V) error) func(string, V) error {
	return watchers.FilterByNamespace(controller.IsControlled, handler)
}
