package k8s

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/test/utils/constants"
	"gotest.tools/assert"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetTestImage() string {
	testImage := os.Getenv("TEST_IMAGE")
	if testImage == "" {
		testImage = "quay.io/skupper/skupper-tests:master"
	}
	return testImage
}

func int32Ptr(i int32) *int32 { return &i }

func CreateTestJob(ns string, kubeClient kubernetes.Interface, name string, command []string) (*batchv1.Job, error) {

	namespace := ns
	testImage := GetTestImage()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"job": name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(3),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"job": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    name,
							Image:   testImage,
							Command: command,
							Env: []apiv1.EnvVar{
								{Name: "JOB", Value: name},
							},
							ImagePullPolicy: apiv1.PullAlways,
						},
					},
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}

	jobsClient := kubeClient.BatchV1().Jobs(namespace)

	job, err := jobsClient.Create(job)

	if err != nil {
		return nil, err
	}
	return job, nil
}

func CreateTestJobWithSecret(ns string, kubeClient kubernetes.Interface, name string, command []string, secretname string) (*batchv1.Job, error) {

	namespace := ns
	testImage := GetTestImage()

	secret, err := kubeClient.CoreV1().Secrets(namespace).Get(secretname, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"job": name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(3),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						"job": name,
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:    name,
							Image:   testImage,
							Command: command,
							Env: []apiv1.EnvVar{
								{Name: "JOB", Value: name},
							},
							ImagePullPolicy: apiv1.PullAlways,
						},
					},
					RestartPolicy: apiv1.RestartPolicyNever,
				},
			},
		},
	}

	AppendSecretVolume(&job.Spec.Template.Spec.Volumes, &job.Spec.Template.Spec.Containers[0].VolumeMounts, secret.Name, "/tmp/certs/"+secretname+"/")

	jobsClient := kubeClient.BatchV1().Jobs(namespace)

	job, err = jobsClient.Create(job)

	if err != nil {
		return nil, err
	}
	return job, nil
}

func WaitForJob(ns string, kubeClient kubernetes.Interface, jobName string, timeout time.Duration) (*batchv1.Job, error) {

	if timeout < constants.DefaultTick {
		return nil, fmt.Errorf("timeout too small: %v", timeout)
	}

	jobsClient := kubeClient.BatchV1().Jobs(ns)

	timeoutCh := time.After(timeout)
	tick := time.Tick(constants.DefaultTick)
	for {
		select {
		case <-timeoutCh:
			return nil, fmt.Errorf("Timeout: Job is still active: %s", jobName)
		case <-tick:
			job, _ := jobsClient.Get(jobName, metav1.GetOptions{})

			if job.Status.Active > 0 {
				fmt.Println("Job is still active")
			} else {
				if job.Status.Succeeded > 0 {
					fmt.Println("Job Successful!")
					return job, nil
				}
				fmt.Printf("Job failed?, status = %v\n", job.Status)
				return job, fmt.Errorf("Job failed. Status: %v", job.Status)
			}
		}
	}

}

func AssertJob(t *testing.T, job *batchv1.Job) {
	t.Helper()
	assert.Equal(t, int(job.Status.Succeeded), 1)
	assert.Equal(t, int(job.Status.Active), 0)

	if job.Status.Failed > 0 {
		t.Logf("WARNING! THIS JOB NEEDED RETRIES TO SUCCEED! Job.Status.Failed = %d\n", job.Status.Failed)
	}
}

func GetJobLogs(ns string, kubeClient kubernetes.Interface, name string) (string, error) {
	return GetJobsLogs(ns, kubeClient, name, false)
}

// Returns the logs of the pods related to a given job.  If 'all' is true, all runs will be
// returned; failures to get individual logs will be reported on the output, but not cause
// the function to fail.
//
// If 'all' is false, only the last run's logs will be returned.
func GetJobsLogs(ns string, kubeClient kubernetes.Interface, name string, all bool) (string, error) {
	pods, err := kube.GetPods(fmt.Sprintf("job=%s", name), ns, kubeClient)
	if err != nil {
		return "", err
	}
	if len(pods) == 0 {
		return "", fmt.Errorf("No pods found for job %s on namespace %s", name, ns)
	}
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].CreationTimestamp.Time.Before(pods[j].CreationTimestamp.Time)
	})
	var fullLogs string
	if all {
		for i, pod := range pods {
			fullLogs += fmt.Sprintf("\n# %v/%v - %v - %v:\n", i+1, len(pods), pod.Name, pod.CreationTimestamp)
			log, err := kube.GetPodContainerLogs(pod.Name, pod.Spec.Containers[0].Name, ns, kubeClient)
			if err != nil {
				fullLogs += fmt.Sprintf(
					"Failed getting logs for %v on %v: %v\n",
					pod.Spec.Containers[0].Name,
					ns,
					err,
				)
			}
			fullLogs += log
			fullLogs += "\n"
		}
	} else {
		// Just the last one
		pod := pods[len(pods)-1]
		fullLogs, err = kube.GetPodContainerLogs(pod.Name, pod.Spec.Containers[0].Name, ns, kubeClient)
		if err != nil {
			return fullLogs, err
		}
	}
	return fullLogs, nil
}

type JobOpts struct {
	Image        string
	BackoffLimit int
	Restart      apiv1.RestartPolicy
	Env          map[string]string
	Labels       map[string]string
	Command      []string
	Args         []string
	ResourceReq  apiv1.ResourceRequirements
}

func NewJob(name, namespace string, opts JobOpts) *batchv1.Job {
	backoffLimit := int32(opts.BackoffLimit)
	envVar := []apiv1.EnvVar{}
	terminationSecs := int64(60)

	// add env vars if any provided
	for name, val := range opts.Env {
		envVar = append(envVar, apiv1.EnvVar{
			Name:  name,
			Value: val,
		})
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    opts.Labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    opts.Labels,
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: name, Image: opts.Image, Env: envVar, Command: opts.Command, Args: opts.Args, Resources: opts.ResourceReq},
					},
					RestartPolicy:                 opts.Restart,
					TerminationGracePeriodSeconds: &terminationSecs,
				},
			},
		},
	}

	return job
}

func AppendSecretVolume(volumes *[]apiv1.Volume, mounts *[]apiv1.VolumeMount, name string, path string) {
	*volumes = append(*volumes, apiv1.Volume{
		Name: name,
		VolumeSource: apiv1.VolumeSource{
			Secret: &apiv1.SecretVolumeSource{
				SecretName: name,
			},
		},
	})
	*mounts = append(*mounts, apiv1.VolumeMount{
		Name:      name,
		MountPath: path,
	})
}
