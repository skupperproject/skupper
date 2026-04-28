package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	pkgutils "github.com/skupperproject/skupper/internal/utils"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

// debugScheme contains both K8s core types and Skupper CRD types
var debugScheme = newDebugScheme()

// newDebugScheme creates and configures a scheme that can serialize both
// K8s core types (ConfigMap, Secret) and Skupper types (Site, Connector, etc.)
func newDebugScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	utilruntime.Must(scheme.AddToScheme(s))
	utilruntime.Must(skupperv2alpha1.AddToScheme(s))
	return s
}

// GetDebugScheme returns the shared debug scheme for serialization
func GetDebugScheme() *runtime.Scheme {
	return debugScheme
}

// RunCommand executes an external command and returns its output
func RunCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// WriteTar adds a file to the tar archive using the Tarball utility
func WriteTar(name string, data []byte, ts time.Time, tb *pkgutils.Tarball) error {
	err := tb.AddFileData(name, 0600, ts, data)
	if err != nil {
		return fmt.Errorf("Failed to write to tar archive: %w", err)
	}
	return nil
}
