package k8s

import (
	"fmt"
	"github.com/skupperproject/skupper/test/utils/constants"
	"gotest.tools/assert"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"os"
	"testing"
	"time"
)

func getTestImage() string {
	testImage := os.Getenv("TEST_IMAGE")
	if testImage == "" {
		testImage = "quay.io/skupper/skupper-tests"
	}
	return testImage
}

func int32Ptr(i int32) *int32 { return &i }

func CreateTestJob(ns string, kubeClient kubernetes.Interface, name string, command []string) (*batchv1.Job, error) {

	namespace := ns
	testImage := getTestImage()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(3),
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
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
				return job, nil
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

func SkipTestJobIfMustBeSkipped(t *testing.T) {
	if os.Getenv("JOB") == "" {
		t.Skip("JOB environment variable not defined")
	}
}
