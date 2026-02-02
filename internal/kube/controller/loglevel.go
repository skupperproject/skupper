package controller

import (
	"log/slog"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers/internalinterfaces"
)

var controllerLogLevel = new(slog.LevelVar) // defaults to Info

func init() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: controllerLogLevel}))
	slog.SetDefault(logger)
}

func skupperLogConfig() internalinterfaces.TweakListOptionsFunc {
	return func(options *metav1.ListOptions) {
		options.FieldSelector = "metadata.name=skupper-log-config"
	}
}

func convertLogLevel(logLevel string) slog.Level {
	switch strings.ToLower(logLevel) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	}
	return slog.LevelInfo
}

func (c *Controller) logConfigUpdate(key string, cm *corev1.ConfigMap) error {
	const controllerLogLevelKey = "CONTROLLER_LOG_LEVEL"
	var slogLevel slog.Level
	if cm == nil {
		// if configmap is deleted, then set log level to info
		slogLevel = slog.LevelInfo
	} else {
		logLevel := cm.Data[controllerLogLevelKey]
		slogLevel = convertLogLevel(logLevel)
	}

	if slogLevel != controllerLogLevel.Level() {
		controllerLogLevel.Set(slogLevel)
	}
	c.log.Info("Updating log level", slog.String("logLevel", slogLevel.String()))
	return nil
}
