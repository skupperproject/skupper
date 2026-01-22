package utils

import (
	"archive/tar"
	"bytes"
	"fmt"
	"os/exec"
	"time"

	"github.com/skupperproject/skupper/pkg/generated/client/clientset/versioned/scheme"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
)

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

func WriteTar(name string, data []byte, ts time.Time, tw *tar.Writer) error {
	hdr := &tar.Header{
		Name:    name,
		Mode:    0600,
		Size:    int64(len(data)),
		ModTime: ts,
	}
	err := tw.WriteHeader(hdr)
	if err != nil {
		return fmt.Errorf("Failed to write tar file header: %w", err)
	}
	_, err = tw.Write(data)
	if err != nil {
		return fmt.Errorf("Failed to write to tar archive: %w", err)
	}
	return nil
}

func WriteObject(rto runtime.Object, name string, tw *tar.Writer) error {
	var b bytes.Buffer
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)
	if err := s.Encode(rto, &b); err != nil {
		return err
	}
	err := WriteTar(name+".yaml", b.Bytes(), time.Now(), tw)
	if err != nil {
		return err
	}
	err = WriteTar(name+".yaml.txt", b.Bytes(), time.Now(), tw)
	if err != nil {
		return err
	}
	return nil
}
