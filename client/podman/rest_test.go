package podman

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/skupperproject/skupper/pkg/utils"
	"gotest.tools/assert"
)

// TestNewPodmanClient if podman (binary) is available, it starts
// a podman service using both a socket file and a tcp port to validate
// if the client is created and works
func TestNewPodmanClient(t *testing.T) {
	tcs := []struct {
		name string
		tcp  bool
	}{
		{name: "tcp-endpoint", tcp: true},
		{name: "unix-endpoint", tcp: false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			endpoint, wg := StartPodmanService(t, ctx, tc.tcp)
			defer wg.Wait()
			defer cancel()
			assert.Assert(t, endpoint != "", "invalid endpoint")
			err := utils.RetryError(time.Second, 10, func() error {
				_, err := NewPodmanClient(endpoint, "")
				return err
			})
			assert.Assert(t, err, "unable to create podman rest client")
		})
	}
}

// PodmanSkipValidation skips the current test if podman binary is not
// available or if the client version found is lesser than 4.0.0.
func PodmanSkipValidation(t *testing.T) {
	stdout := new(bytes.Buffer)
	cmd := exec.Command("podman", "version", "--format=json")
	cmd.Stdout = stdout
	err := cmd.Run()
	if err != nil || stdout.Len() == 0 {
		t.Skipf("podman binary is not available - %v - %s", err, stdout.String())
	}

	jsonBytes := stdout.Bytes()
	res := map[string]interface{}{}
	err = json.Unmarshal(jsonBytes, &res)
	if err != nil {
		t.Skipf("unable to validate podman version - %s", err)
	}

	client, ok := res["Client"]
	if !ok {
		t.Skip("unable to determine podman client version")
	}
	clientMap := client.(map[string]interface{})
	version, ok := clientMap["Version"]
	if !ok {
		t.Skip("podman client version not defined")
	}

	versionStr := version.(string)
	v := utils.ParseVersion(versionStr)
	if v.Major < 4 {
		t.Skipf("podman version must be greater or equal to 4.0.0 - found: %s", versionStr)
	}
}

func StartPodmanService(t *testing.T, ctx context.Context, tcp bool) (string, *sync.WaitGroup) {
	// Validate if podman is available or skip
	PodmanSkipValidation(t)
	var endpoint string
	if tcp {
		port, err := utils.TcpPortNextFree(1024)
		assert.Assert(t, err, "no tcp ports available")
		endpoint = fmt.Sprintf("tcp://0.0.0.0:%d", port)
	} else {
		f, err := os.CreateTemp(os.TempDir(), "podman.*.sock")
		assert.Assert(t, err, "error creating temporary file")
		_ = f.Close()
		endpoint = fmt.Sprintf("unix://%s", f.Name())
	}
	t.Logf("podman service listening at endpoint: %s", endpoint)
	wg := new(sync.WaitGroup)
	wg.Add(1)
	go func() {
		cmd := exec.CommandContext(ctx, "podman", "system", "service", "--time=0", endpoint)
		_ = cmd.Run()
		wg.Done()
	}()
	return endpoint, wg
}
