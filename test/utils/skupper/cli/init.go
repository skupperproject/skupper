package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/skupper/console"
	v12 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// InitTester runs `skupper init` and validates output, console,
// as well as skupper resources that should be availble in the cluster.
type InitTester struct {
	ConsoleAuth           string
	ConsoleUser           string
	ConsolePassword       string
	Ingress               string
	ConsoleIngress        string
	RouterDebugMode       string
	RouterLogging         string
	RouterMode            string
	RouterCPU             string
	RouterMemory          string
	ControllerCPU         string
	ControllerMemory      string
	RouterCPULimit        string
	RouterMemoryLimit     string
	ControllerCPULimit    string
	ControllerMemoryLimit string
	SiteName              string
	EnableConsole         bool
	EnableFlowCollector   bool
	RunAsUser             string
	RunAsGroup            string
}

func (s *InitTester) Command(platform types.Platform, cluster *base.ClusterContext) []string {
	// Populate common skupper options
	args := SkupperCommonOptions(platform, cluster)
	args = append(args, "init")

	if s.ConsoleAuth != "" {
		args = append(args, "--console-auth", s.ConsoleAuth)
	}
	if s.ConsoleUser != "" {
		args = append(args, "--console-user", s.ConsoleUser)
	}
	if s.ConsolePassword != "" {
		args = append(args, "--console-password", s.ConsolePassword)
	}
	if s.Ingress != "" {
		args = append(args, "--ingress", s.Ingress)
	}
	if s.ConsoleIngress != "" {
		args = append(args, "--console-ingress", s.ConsoleIngress)
	}
	if s.RouterDebugMode == "" && os.Getenv(base.ENV_SKIP_DEBUG) == "" {
		s.RouterDebugMode = "gdb"
	}
	args = append(args, "--router-debug-mode", s.RouterDebugMode)
	if s.RouterLogging == "" {
		s.RouterLogging = "trace"
	}
	args = append(args, "--router-logging", s.RouterLogging)
	if s.RouterMode != "" {
		args = append(args, "--router-mode", s.RouterMode)
	}
	if s.RouterCPU != "" {
		args = append(args, "--router-cpu", s.RouterCPU)
	}
	if s.RouterMemory != "" {
		args = append(args, "--router-memory", s.RouterMemory)
	}
	if s.ControllerCPU != "" {
		args = append(args, "--controller-cpu", s.ControllerCPU)
	}
	if s.ControllerMemory != "" {
		args = append(args, "--controller-memory", s.ControllerMemory)
	}
	if s.RouterCPULimit != "" {
		args = append(args, "--router-cpu-limit", s.RouterCPULimit)
	}
	if s.RouterMemoryLimit != "" {
		args = append(args, "--router-memory-limit", s.RouterMemoryLimit)
	}
	if s.ControllerCPULimit != "" {
		args = append(args, "--controller-cpu-limit", s.ControllerCPULimit)
	}
	if s.ControllerMemoryLimit != "" {
		args = append(args, "--controller-memory-limit", s.ControllerMemoryLimit)
	}
	if s.SiteName != "" {
		args = append(args, "--site-name", s.SiteName)
	}
	args = append(args, fmt.Sprintf("--enable-console=%v", s.EnableConsole))
	args = append(args, fmt.Sprintf("--enable-flow-collector=%v", s.EnableFlowCollector))
	if s.RunAsUser != "" {
		args = append(args, "--run-as-user", s.RunAsUser)
	}
	if s.RunAsGroup != "" {
		args = append(args, "--run-as-group", s.RunAsGroup)
	}
	return args
}

func (s *InitTester) Run(platform types.Platform, cluster *base.ClusterContext) (stdout string, stderr string, err error) {
	stdout, stderr, err = RunSkupperCli(s.Command(platform, cluster))
	if err != nil {
		return
	}

	// Validate if init output contains the namespace where skupper was installed
	log.Println("Validating 'skupper init'")
	if !strings.Contains(stdout, fmt.Sprintf("Skupper is now installed in namespace '%s'.", cluster.Namespace)) {
		err = fmt.Errorf("init output is valid - missing namespace info")
	}

	// Wait for skupper pods to be running
	log.Println("Waiting on Skupper pods to be running")
	if err = base.WaitSkupperRunning(cluster); err != nil {
		return
	}

	// Validating the console based on Init Tester flags
	if s.EnableConsole {
		log.Println("Validating console")
		if err = s.ValidateConsole(cluster); err != nil {
			return
		}
	}

	// Validating Ingress
	log.Println("Validating ingress")
	if err = s.ValidateIngress(cluster); err != nil {
		return
	}

	// Validating Console Ingress
	if s.EnableConsole {
		log.Println("Validating console ingress")
		if err = s.ValidateConsoleIngress(cluster); err != nil {
			return
		}
	}

	// Validating Router Debug Mode
	log.Println("Validating router debug mode")
	if err = s.ValidateRouterDebugMode(cluster); err != nil {
		return
	}

	// Validating router logging
	log.Println("Validating router logging")
	if err = s.ValidateRouterLogging(cluster); err != nil {
		return
	}

	// Validating router mode
	log.Println("Validating router mode")
	if err = s.validateRouterMode(cluster); err != nil {
		return
	}

	// Validating site name
	log.Println("Validating site name")
	if err = s.validateSiteName(cluster); err != nil {
		return
	}

	// Validating router cpu and memory requests
	log.Println("Validating router cpu and memory requests")
	if err = s.validateRouterCPUMemory(cluster); err != nil {
		return
	}

	// Validating controller cpu and memory requests
	log.Println("Validating controller cpu and memory requests")
	if err = s.validateControllerCPUMemory(cluster); err != nil {
		return
	}

	// Validating security context
	log.Println("Validating deployment pod security context")
	if err = s.validatePodSecurityContext(cluster); err != nil {
		return
	}
	return
}

func (s *InitTester) ValidateConsole(cluster *base.ClusterContext) error {

	consoleEnabled := console.IsConsoleEnabled(cluster)

	// If console enabled, verify console
	if !s.EnableConsole {
		if consoleEnabled {
			return fmt.Errorf("skupper console was supposed to be disabled, but it is enabled")
		}
		return nil
	}

	// Ensure it is effectively enabled
	if !consoleEnabled {
		return fmt.Errorf("skupper console was supposed to be enabled, but it is disabled")
	}

	// Verifying console authentication (just internal and unsecured for now)
	if s.ConsoleAuth != "openshift" {
		var user, pass string

		// retrieve password
		err, user, pass := console.GetInternalCredentials(cluster)
		if err != nil {
			return fmt.Errorf("error retrieving internal credentials - %v", err)
		}

		// validating if user and password match
		if s.ConsoleUser != "" {
			if user != s.ConsoleUser {
				return fmt.Errorf("console username not defined as requested - expected: %s - found: %s", s.ConsoleUser, user)
			}
		}
		if s.ConsolePassword != "" {
			if pass != s.ConsolePassword {
				return fmt.Errorf("console password not defined as requested - expected: %s - found: %s", s.ConsoleUser, user)
			}
		}

		ctx, cancelFn := context.WithTimeout(context.Background(), constants.SkupperServiceReadyPeriod)
		defer cancelFn()
		err = utils.RetryWithContext(ctx, constants.DefaultTick, func() (bool, error) {
			if _, err = base.GetConsoleData(cluster, user, pass); err != nil {
				log.Printf("unable to get console data from /DATA endpoint - %v", err)
				return false, nil
			}
			return true, nil
		})
	}

	// if running against an OpenShift cluster, check for route
	routeClient := cluster.VanClient.RouteClient
	if routeClient != nil {
		route, err := routeClient.Routes(cluster.Namespace).Get(context.TODO(), types.ControllerServiceName, v1.GetOptions{})
		if err != nil {
			return fmt.Errorf("error retrieving route: %s - %v", types.ControllerServiceName, err)
		}
		if route.Spec.To.Kind != "Service" || route.Spec.To.Name != types.ControllerServiceName {
			return fmt.Errorf("console route is not targetting the correct service - expected: Service/%s - found: %s/%s",
				types.ControllerServiceName, route.Spec.To.Kind, route.Spec.To.Name)
		}
	}

	return nil
}

func (s *InitTester) ValidateIngress(cluster *base.ClusterContext) error {
	// If edge mode assert there is no skupper-router service defined
	if s.RouterMode == string(types.TransportModeEdge) {
		_, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), types.TransportServiceName, v1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	ingress := s.Ingress
	if ingress == "" {
		ingress = cluster.VanClient.GetIngressDefault()
	}
	return s.validateIngressFor(cluster, ingress, types.TransportServiceName, types.EdgeRouteName, types.InterRouterRouteName)
}

func (s *InitTester) ValidateConsoleIngress(cluster *base.ClusterContext) error {
	if !s.EnableConsole {
		return nil
	}
	ingress := s.ConsoleIngress
	if ingress == "" {
		if s.Ingress != "" {
			ingress = s.Ingress
		} else {
			ingress = cluster.VanClient.GetIngressDefault()
		}
	}
	return s.validateIngressFor(cluster, ingress, types.ControllerServiceName, types.ConsoleRouteName)
}

func (s *InitTester) validateIngressFor(cluster *base.ClusterContext, ingress string, service string, routes ...string) error {
	if ingress == "" {
		// If not specified, used the default one from Skupper API
		ingress = cluster.VanClient.GetIngressDefault()
	}

	// Verifying the transport service
	svc, err := cluster.VanClient.KubeClient.CoreV1().Services(cluster.Namespace).Get(context.TODO(), service, v1.GetOptions{})
	if err != nil && ingress != "none" {
		return fmt.Errorf("could not find service: %s - %v", service, err)
	} else if err == nil {
		// validating service type
		var expectedType v12.ServiceType = v12.ServiceTypeClusterIP
		if ingress == "loadbalancer" {
			expectedType = v12.ServiceTypeLoadBalancer
		}
		if svc.Spec.Type != expectedType {
			return fmt.Errorf("invalid service type for service: %s - expected: %s - found: %s",
				service, expectedType, svc.Spec.Type)
		}
	}

	// if running against an OpenShift cluster, check for route
	routeClient := cluster.VanClient.RouteClient
	if routeClient != nil {
		for _, routeName := range routes {
			_, err := routeClient.Routes(cluster.Namespace).Get(context.TODO(), routeName, v1.GetOptions{})
			// route expected at this point
			if err != nil && ingress == "route" {
				return fmt.Errorf("expected route not found: %s - %v", routeName, err)
			}
			if err == nil && ingress == "none" {
				return fmt.Errorf("route is not expected using --ingress=none: %s", routeName)
			}
		}
	}

	return nil
}

func (s *InitTester) validateRouterMode(cluster *base.ClusterContext) error {

	// Loading config map
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cluster.Namespace, cluster.VanClient.KubeClient)
	if err != nil {
		log.Printf("%s config map not found - %v", types.TransportConfigMapName, err)
		return err
	}

	// Loading RouterConfig
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		log.Printf("unable to retrieve RouterConfig from config map - %v", err)
		return err
	}

	// Validating mode on router config
	if string(routerConfig.Metadata.Mode) != s.RouterMode {
		return fmt.Errorf("incorrect router mode - expected: %s - found: %s", s.RouterMode, routerConfig.Metadata.Mode)
	}

	routeClient := cluster.VanClient.RouteClient
	if routeClient != nil && s.Ingress == "route" {

		for _, routeName := range []string{types.EdgeRouteName, types.InterRouterRouteName} {
			targetPortName := routeName[strings.Index(routeName, "-")+1:]
			route, err := routeClient.Routes(cluster.Namespace).Get(context.TODO(), routeName, v1.GetOptions{})

			// route expected at this point
			if err != nil && s.RouterMode == "interior" {
				return fmt.Errorf("expected route not found: %s - %v", routeName, err)
			}

			// route not expected using edge mode
			if err == nil && s.RouterMode == "edge" {
				return fmt.Errorf("route not expected using --router-mode=edge: %s", routeName)
			}

			// if edge mode, continue
			if s.RouterMode == "edge" {
				continue
			}

			// Verify routes
			if route.Spec.To.Kind != "Service" || route.Spec.To.Name != types.TransportServiceName {
				return fmt.Errorf("controller route is not targetting the correct service - expected: Service/%s - found: %s/%s",
					types.TransportServiceName, route.Spec.To.Kind, route.Spec.To.Name)
			}
			if route.Spec.Port.String() != targetPortName {
				return fmt.Errorf("controller route is not targetting the correct service - expected: Service/%s - found: %s/%s",
					types.TransportServiceName, route.Spec.To.Kind, route.Spec.To.Name)
			}
		}
	}

	return nil
}

func (s *InitTester) ValidateRouterDebugMode(cluster *base.ClusterContext) error {
	dep, err := cluster.VanClient.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(context.TODO(), types.TransportDeploymentName, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected deployment not found: %s - %v", types.TransportDeploymentName, err)
	}
	if s.RouterDebugMode == "" {
		return nil
	}
	found := false
	for _, container := range dep.Spec.Template.Spec.Containers {
		if container.Name == "router" {
			for _, envVar := range container.Env {
				if envVar.Name == "QDROUTERD_DEBUG" {
					found = true
					if envVar.Value != s.RouterDebugMode {
						return fmt.Errorf("incorrect debug mode defined - expected: %s - found: %s", s.RouterDebugMode, envVar.Value)
					}
				}
			}
		}
	}
	if !found {
		return fmt.Errorf("%s deployment is missing the QDROUTERD_DEBUG environment variable", types.TransportDeploymentName)
	}
	return nil
}

func (s *InitTester) ValidateRouterLogging(cluster *base.ClusterContext) error {

	// Loading config map
	configmap, err := kube.GetConfigMap(types.TransportConfigMapName, cluster.Namespace, cluster.VanClient.KubeClient)
	if err != nil {
		log.Printf("%s config map not found - %v", types.TransportConfigMapName, err)
		return err
	}

	// Loading RouterConfig
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		log.Printf("unable to retrieve RouterConfig from config map - %v", err)
		return err
	}

	// Validating log levels
	parsedLogConfig, err := qdr.ParseRouterLogConfig(s.RouterLogging)
	if s.RouterLogging != "" && parsedLogConfig == nil {
		return fmt.Errorf("router logging is not configured properly (empty) - expected: %s", s.RouterLogging)
	} else if s.RouterLogging == "" {
		// at this point there is nothing else to validate
		return nil
	}
	if err != nil {
		log.Printf("unable to parse logging config - %v", err)
		return err
	}
	for _, log := range parsedLogConfig {
		if log.Module == "" {
			log.Module = "DEFAULT"
		}
		if !strings.HasSuffix(log.Level, "+") {
			log.Level += "+"
		}
		curModule, ok := routerConfig.LogConfig[log.Module]
		if !ok {
			return fmt.Errorf("requested log level is not defined in router config: %s", log.Module)
		}
		if curModule.Enable != log.Level {
			return fmt.Errorf("logging level not configured properly for module: %s - expected: %s - found: %s",
				log.Module, log.Level, curModule.Enable)
		}
	}

	return nil
}

func (s *InitTester) validateSiteName(cluster *base.ClusterContext) error {
	inspect, err := cluster.VanClient.RouterInspect(context.Background())
	if err != nil {
		log.Printf("error inspecting router - %v", err)
	}
	expectedSiteName := cluster.Namespace
	if s.SiteName != "" {
		expectedSiteName = s.SiteName
	}
	if expectedSiteName != inspect.Status.SiteName {
		return fmt.Errorf("incorrect site name - expected: %s - found: %s", expectedSiteName, inspect.Status.SiteName)
	}
	return nil
}

func (s *InitTester) validateRouterCPUMemory(cluster *base.ClusterContext) error {
	// expect resources to be defined at router container
	expectResources := s.RouterCPU != "" || s.RouterMemory != ""

	// retrieving the router pods
	routerSelector := fmt.Sprintf("%s=%s", types.ComponentAnnotation, types.TransportComponentName)
	pods, err := kube.GetPods(routerSelector, cluster.Namespace, cluster.VanClient.KubeClient)
	if err != nil {
		return err
	}

	// looping through pods
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.Name == "router" {
				if !expectResources {
					// If not expecting requests, but something is defined throw an error
					if len(container.Resources.Requests) > 0 {
						return fmt.Errorf("resources not requested but defined in the router (pod: %s, container: %s) - requests: %v",
							pod.Name, container.Name, container.Resources.Requests)
					}
				} else {
					// If requests have been specified, assert the correct values have been defined
					cpu := container.Resources.Requests.Cpu().String()
					mem := container.Resources.Requests.Memory().String()

					if s.RouterCPU != cpu {
						return fmt.Errorf("--router-cpu defined as: [%s] but container has: [%s]", s.RouterCPU, cpu)
					}
					if s.RouterMemory != mem {
						return fmt.Errorf("--router-memory defined as: [%s] but container has: [%s]", s.RouterMemory, mem)
					}
				}
			}
		}
	}
	return nil
}

func (s *InitTester) validateControllerCPUMemory(cluster *base.ClusterContext) error {
	// expect resources to be defined at service controller container
	expectResources := s.ControllerCPU != "" || s.ControllerMemory != ""

	// retrieving the service controller pods
	controllerSelector := fmt.Sprintf("%s=%s", types.ComponentAnnotation, types.ControllerComponentName)
	pods, err := kube.GetPods(controllerSelector, cluster.Namespace, cluster.VanClient.KubeClient)
	if err != nil {
		return err
	}

	// looping through pods
	for _, pod := range pods {
		for _, container := range pod.Spec.Containers {
			if container.Name == "router" {
				if !expectResources {
					// If not expecting requests, but something is defined throw an error
					if len(container.Resources.Requests) > 0 {
						return fmt.Errorf("resources not requested but defined in the controller (pod: %s, container: %s) - requests: %v",
							pod.Name, container.Name, container.Resources.Requests)
					}
				} else {
					// If requests have been specified, assert the correct values have been defined
					cpu := container.Resources.Requests.Cpu().String()
					mem := container.Resources.Requests.Memory().String()

					if s.ControllerCPU != cpu {
						return fmt.Errorf("--controller-cpu defined as: [%s] but container has: [%s]", s.ControllerCPU, cpu)
					}
					if s.ControllerMemory != mem {
						return fmt.Errorf("--controller-memory defined as: [%s] but container has: [%s]", s.ControllerMemory, mem)
					}
				}
			}
		}
	}
	return nil
}

func (s *InitTester) validatePodSecurityContext(cluster *base.ClusterContext) error {
	dep, err := cluster.VanClient.KubeClient.AppsV1().Deployments(cluster.Namespace).Get(types.TransportDeploymentName, v1.GetOptions{})
	if err != nil {
		return fmt.Errorf("expected deployment not found: %s - %v", types.TransportDeploymentName, err)
	}
	psc := dep.Spec.Template.Spec.SecurityContext

	if psc == nil {
		return fmt.Errorf("expected security context not found: %s", types.TransportDeploymentName)
	}

	if psc.RunAsNonRoot != nil && *psc.RunAsNonRoot != true {
		return fmt.Errorf("expected security context to RunAsNonRoot")
	}

	if psc.RunAsUser != nil {
		rau, _ := strconv.ParseInt(s.RunAsUser, 10, 64)
		if *psc.RunAsUser != rau {
			return fmt.Errorf("--run-as-user defined as: [%s] but deployment has [%d]", s.RunAsUser, *psc.RunAsUser)
		}
	}

	if psc.RunAsGroup != nil {
		rag, _ := strconv.ParseInt(s.RunAsGroup, 10, 64)
		if *psc.RunAsGroup != rag {
			return fmt.Errorf("--run-as-group defined as: [%s] but deployment has [%d]", s.RunAsGroup, *psc.RunAsGroup)
		}
	}
	return nil
}
