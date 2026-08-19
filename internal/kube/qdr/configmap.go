package qdr

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/internal/qdr"
	corev1 "k8s.io/api/core/v1"
)

type ConfigMapWriter struct {
	CompressionThreshold int
}

func (w ConfigMapWriter) WriteConfigMap(rc *qdr.RouterConfig, cm *corev1.ConfigMap) error {
	marshalled, err := qdr.MarshalRouterConfig(*rc)
	if err != nil {
		return err
	}
	return w.writeConfigMapData(marshalled, cm)
}

func (w ConfigMapWriter) usesDesiredRepresentation(rc *qdr.RouterConfig, cm *corev1.ConfigMap) (bool, error) {
	marshalled, err := qdr.MarshalRouterConfig(*rc)
	if err != nil {
		return false, err
	}
	_, plain := cm.Data[types.TransportConfigFile]
	_, compressed := cm.BinaryData[types.TransportConfigFileCompressed]
	if w.CompressionThreshold > 0 && len(marshalled) >= w.CompressionThreshold {
		return compressed && !plain, nil
	}
	return plain && !compressed, nil
}

func (w ConfigMapWriter) writeConfigMapData(marshalled string, cm *corev1.ConfigMap) error {
	if w.CompressionThreshold > 0 && len(marshalled) >= w.CompressionThreshold {
		compressed, err := compressConfig(marshalled)
		if err != nil {
			return err
		}
		if cm.BinaryData == nil {
			cm.BinaryData = map[string][]byte{}
		}
		cm.BinaryData[types.TransportConfigFileCompressed] = compressed
		delete(cm.Data, types.TransportConfigFile)
		return nil
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[types.TransportConfigFile] = marshalled
	delete(cm.BinaryData, types.TransportConfigFileCompressed)
	return nil
}

func compressConfig(config string) ([]byte, error) {
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write([]byte(config)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func GetRouterConfigData(cm *corev1.ConfigMap) (string, error) {
	if compressed, ok := cm.BinaryData[types.TransportConfigFileCompressed]; ok {
		return decompressConfig(compressed)
	}
	return cm.Data[types.TransportConfigFile], nil
}

func GetRouterConfigFromConfigMap(cm *corev1.ConfigMap) (*qdr.RouterConfig, error) {
	data, err := GetRouterConfigData(cm)
	if err != nil {
		return nil, err
	}
	if data == "" {
		return nil, nil
	}
	routerConfig, err := qdr.UnmarshalRouterConfig(data)
	if err != nil {
		return nil, err
	}
	return &routerConfig, nil
}

func decompressConfig(compressed []byte) (string, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return "", fmt.Errorf("error reading compressed router config: %s", err)
	}
	defer reader.Close()
	config, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("error decompressing router config: %s", err)
	}
	return string(config), nil
}
