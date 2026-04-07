package utils

import (
	"bytes"
	"fmt"
	"os/exec"
	"time"

	pkgutils "github.com/skupperproject/skupper/internal/utils"
	skupperv2alpha1 "github.com/skupperproject/skupper/pkg/apis/skupper/v2alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
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

// WriteObject writes a Kubernetes object as both .yaml and .yaml.txt to the tar archive.
// Supports both core K8s types (ConfigMap, Secret, etc.) and Skupper types (Site, Connector, etc.)
func WriteObject(rto runtime.Object, name string, tb *pkgutils.Tarball) error {
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, debugScheme, debugScheme)
	if err := s.Encode(rto, &b); err != nil {
		return err
	}
	err := WriteTar(name+".yaml", b.Bytes(), time.Now(), tb)
	if err != nil {
		return err
	}
	err = WriteTar(name+".yaml.txt", b.Bytes(), time.Now(), tb)
	if err != nil {
		return err
	}
	return nil
}
