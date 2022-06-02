package common

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/utils"
	"github.com/skupperproject/skupper/test/utils/base"
	"github.com/skupperproject/skupper/test/utils/constants"
	"github.com/skupperproject/skupper/test/utils/k8s"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	stepLog = log.New(os.Stdout, "  ", log.LstdFlags)
	// stepLog = log.New(os.Stdout, "  ", log.LstdFlags|log.Lmsgprefix)
)

func subStepLog(parent *log.Logger) *log.Logger {
	return log.New(parent.Writer(), parent.Prefix()+"  ", parent.Flags())
}

func RunPerformanceTest(perfTest PerformanceTest) error {
	app := perfTest.App()
	log.Printf("- Running performance test for: %s", app.Name)
	stepLog.Printf(app.Description)

	// Deploying server app
	if err := deployServer(app); err != nil {
		return err
	}

	// Expose the target service
	if err := exposeService(app); err != nil {
		return err
	}

	// Running all client jobs
	if err := runClientJobs(perfTest); err != nil {
		return err
	}

	return nil
}

func runClientJobs(perfTest PerformanceTest) error {
	app := perfTest.App()
	logger := subStepLog(stepLog)
	clientCluster, err := getClientCluster()
	if err != nil {
		return fmt.Errorf("error getting cluster to run client app: %v", err)
	}
	serverCluster, err := getServerCluster()
	if err != nil {
		return fmt.Errorf("error getting server cluster: %v", err)
	}

	for _, job := range app.Client.Jobs {
		resultInfo := resultInfo{job: job}
		stepLog.Printf("- Running client job %s at %s", job.Name, clientCluster.Namespace)

		// Running job
		_, err := clientCluster.VanClient.KubeClient.BatchV1().Jobs(clientCluster.Namespace).Create(job.Job)
		if err != nil {
			return fmt.Errorf("error creating client job %s - %v", job.Name, err)
		}

		// Waiting for the job to complete successfully
		logger.Printf("- waiting job %s to complete", job.Name)
		_, jobErr := k8s.WaitForJob(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name, app.Client.Timeout)
		if jobErr != nil {
			testRunner.DumpTestInfo(job.Name)
		}
		// Saving job logs
		logs, err := k8s.GetJobLogs(clientCluster.Namespace, clientCluster.VanClient.KubeClient, job.Name)
		if err != nil {
			logger.Printf("- error saving logs for job %s - %v", job.Name, err)
		} else {
			// Writing job logs to file before asserting if job has passed
			logFileName := fmt.Sprintf("%s/%s.log", OutputPath, job.Name)
			fullLogFileName, _ := filepath.Abs(logFileName)
			logger.Printf("- writing logs at: %s", fullLogFileName)
			logFile, err := os.Create(logFileName)
			if err != nil {
				logger.Printf("- error creating logfile - %v", err)
			} else {
				defer logFile.Close()
				_, err = logFile.WriteString(logs)
			}
			resultInfo.logFile = fullLogFileName
		}

		// Assert job has completed
		if jobErr != nil {
			return fmt.Errorf("job completed with error - %v", jobErr)
		}

		// Parsing results
		stepLog.Printf("- Validating results")
		result := perfTest.Validate(serverCluster, clientCluster, job)
		result.App = app
		result.Sites = skupperSites
		result.Skupper = *skupperSettings
		result.Job = job

		if result.Error != nil {
			return fmt.Errorf("error detected during validation: %v", err)
		}

		// Store JSON result
		stepLog.Printf("- Saving JSON result")
		jsonFileName := fmt.Sprintf("%s/%s.json", OutputPath, job.Name)
		fullJsonFileName, _ := filepath.Abs(jsonFileName)
		logger.Printf("- writing json at: %s", fullJsonFileName)
		jsonFile, err := os.Create(jsonFileName)
		if err != nil {
			return fmt.Errorf("error creating result file for json: %v", err)
		}
		defer jsonFile.Close()
		resultJson, err := json.Marshal(&result)
		if err != nil {
			return fmt.Errorf("result cannot be converted to JSON: %v", err)
		}
		jsonBytes, err := jsonFile.Write(resultJson)
		if err != nil {
			return fmt.Errorf("error writing JSON results: %v", err)
		}
		logger.Printf("- %d bytes written into JSON file", jsonBytes)

		resultInfo.result = result
		resultInfo.jsonFile = fullJsonFileName
		summary.addResult(app, resultInfo)
	}

	return nil
}

func exposeService(app PerformanceApp) error {
	serverCluster, _ := getServerCluster()
	// Creating skupper service
	if skupperSites > 0 {
		// Exposing skupper service
		svc := app.Service
		skupperSvc := &types.ServiceInterface{
			Address:  svc.Address,
			Protocol: string(svc.Adaptor),
			Ports:    []int{svc.Port},
		}

		// Creating service
		stepLog.Printf("- Creating service %s (port %d)", skupperSvc.Address, skupperSvc.Ports)
		if err := serverCluster.VanClient.ServiceInterfaceCreate(context.Background(), skupperSvc); err != nil {
			return fmt.Errorf("error creating skupper service %s - %v", svc.Address, err)
		}
		// Binding the service to the deployment
		err := serverCluster.VanClient.ServiceInterfaceBind(context.Background(), skupperSvc, "deployment", app.Server.Deployment.Name, skupperSvc.Protocol, map[int]int{svc.Port: svc.Port})
		if err != nil {
			return fmt.Errorf("error binding service %s - %v", svc.Address, err)
		}

		// Waiting for service to be available across all namespaces/clusters
		for i := 1; i <= testRunner.Needs.PublicClusters; i++ {
			ctx, _ := testRunner.GetPublicContext(i)
			_, err := k8s.WaitForSkupperServiceToBeCreatedAndReadyToUse(ctx.Namespace, ctx.VanClient.KubeClient, skupperSvc.Address)
			if err != nil {
				return fmt.Errorf("timedout waiting for service %s to be ready - %v", skupperSvc.Address, err)
			}
		}
	} else {
		svc := app.Service
		stepLog.Printf("- Creating service %s (port %d) - without Skupper", svc.Address, svc.Port)

		// Create a simple k8s service
		_, err := serverCluster.VanClient.KubeClient.CoreV1().Services(serverCluster.Namespace).Create(&v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:   svc.Address,
				Labels: app.Server.Deployment.Labels,
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{Port: int32(svc.Port)},
				},
				Selector: app.Server.Deployment.Labels,
			},
		})
		if err != nil {
			return fmt.Errorf("error creating kubernetes service %s - %v", svc.Address, err)
		}
	}
	return nil
}

func deployServer(app PerformanceApp) error {
	// Deploying server app
	serverCluster, err := getServerCluster()
	if err != nil {
		return fmt.Errorf("error getting cluster to deploy server app: %v", err)
	}
	stepLog.Printf("- Deploying %s at %s", app.Server.Deployment.Name, serverCluster.Namespace)

	// Verify if server is already deployed
	_, err = serverCluster.VanClient.KubeClient.AppsV1().Deployments(serverCluster.Namespace).Get(app.Server.Deployment.Name, metav1.GetOptions{})
	if err == nil {
		stepLog.Printf("- %s is already running on namespace %s (ignoring)", app.Server.Deployment.Name, serverCluster.Namespace)
		return nil
	}

	// Specify resource requirements (if requested)
	var resourceReqs v1.ResourceRequirements
	if app.Server.Resources.Memory != "" || app.Server.Resources.CPU != "" {
		requests := map[v1.ResourceName]resource.Quantity{}
		if app.Server.Resources.Memory != "" {
			memoryQty, err := resource.ParseQuantity(app.Server.Resources.Memory)
			if err != nil {
				return fmt.Errorf("error parsing memory request - %v", err)
			}
			requests[v1.ResourceMemory] = memoryQty
		}
		if app.Server.Resources.CPU != "" {
			cpuQty, err := resource.ParseQuantity(app.Server.Resources.CPU)
			if err != nil {
				return fmt.Errorf("error parsing cpu request - %v", err)
			}
			requests[v1.ResourceCPU] = cpuQty
		}
		resourceReqs.Requests = requests
	}
	for _, c := range app.Server.Deployment.Spec.Template.Spec.Containers {
		c.Resources = resourceReqs
	}

	// Deploying server
	if _, err = serverCluster.VanClient.KubeClient.AppsV1().Deployments(serverCluster.Namespace).Create(app.Server.Deployment); err != nil {
		return fmt.Errorf("error deploying %s - %v", app.Server.Deployment.Name, err)
	}

	// Waiting for server deployment to be ready
	_, err = kube.WaitDeploymentReadyReplicas(app.Server.Deployment.Name, serverCluster.Namespace, 1,
		serverCluster.VanClient.KubeClient, constants.SkupperServiceReadyPeriod, constants.DefaultTick)
	if err != nil {
		return fmt.Errorf("error waiting for deployment to be ready - %v", err)
	}

	// Validating if post initialization commands defined
	if len(app.Server.PostInitCommands) > 0 {
		stepLog.Printf("- Executing post init commands")
		pods, _ := kube.GetPods(utils.StringifySelector(app.Server.Deployment.Spec.Template.Labels), serverCluster.Namespace, serverCluster.VanClient.KubeClient)
		cmdlog := subStepLog(stepLog)
		for _, cmd := range app.Server.PostInitCommands {
			for _, pod := range pods {
				cmdlog.Printf("- Running command: %v on %s", cmd, pod.Name)
				stdout, stderr, err := k8s.Execute(serverCluster.VanClient.KubeClient, serverCluster.VanClient.RestConfig, serverCluster.Namespace, pod.Name, "", cmd)
				if err != nil {
					cmdlog.Printf("error: %v", err)
					cmdlog.Printf("stdout: %s", stdout.String())
					cmdlog.Printf("stderr: %s", stderr.String())
					return fmt.Errorf("error executing post-init command: %v - %v", cmd, err)
				}
			}
		}
	}
	return nil
}

func getServerCluster() (*base.ClusterContext, error) {
	return testRunner.GetPublicContext(1)
}

func getClientCluster() (*base.ClusterContext, error) {
	if testRunner == nil {
		return nil, fmt.Errorf("unable to get test runner clusters")
	}
	cc, err := testRunner.GetPublicContext(1)
	if err == nil && testRunner.Needs.PublicClusters > 0 {
		cc, err = testRunner.GetPublicContext(testRunner.Needs.PublicClusters)
	}
	return cc, err
}
