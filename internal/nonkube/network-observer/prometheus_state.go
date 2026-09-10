package networkobserver

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/skupperproject/skupper/internal/utils"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
)

type PrometheusState struct {
	Port int `json:"port"`
}

func prometheusStateFile() string {
	return filepath.Join(api.GetHostPrometheusHome(), "prometheus.yml.state")
}

func WritePrometheusState(port int) error {
	data, err := json.Marshal(PrometheusState{Port: port})
	if err != nil {
		return fmt.Errorf("failed to marshal prometheus state: %w", err)
	}
	if err := os.WriteFile(prometheusStateFile(), data, 0644); err != nil {
		return fmt.Errorf("failed to write prometheus state file: %w", err)
	}
	return nil
}

func ReadPrometheusPort() (int, error) {
	data, err := os.ReadFile(prometheusStateFile())
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("prometheus is not installed; run \"skupper system prometheus\" first")
		}
		return 0, fmt.Errorf("failed to read prometheus state file: %w", err)
	}
	var state PrometheusState
	if err := json.Unmarshal(data, &state); err != nil {
		return 0, fmt.Errorf("failed to parse prometheus state file: %w", err)
	}
	return state.Port, nil
}

func IsPrometheusInstalled() bool {
	_, err := ReadPrometheusPort()
	return err == nil
}

func WriteTargetFile(namespace string, metricsPort int) error {
	type target struct {
		Targets []string          `json:"targets"`
		Labels  map[string]string `json:"labels"`
	}
	targets := []target{
		{
			Targets: []string{fmt.Sprintf("localhost:%d", metricsPort)},
			Labels:  map[string]string{"namespace": namespace},
		},
	}
	data, err := json.MarshalIndent(targets, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal target file: %w", err)
	}
	targetsDir := api.GetPrometheusTargetsDir()
	if err := os.MkdirAll(targetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create targets directory: %w", err)
	}
	targetFile := filepath.Join(targetsDir, namespace+".json")
	if err := os.WriteFile(targetFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write target file %s: %w", targetFile, err)
	}
	return nil
}

func RemoveTargetFile(namespace string) error {
	targetFile := filepath.Join(api.GetPrometheusTargetsDir(), namespace+".json")
	if err := os.Remove(targetFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove target file %s: %w", targetFile, err)
	}
	return nil
}

func claimedMetricsPorts() map[int]bool {
	claimed := map[int]bool{}
	targetsDir := api.GetPrometheusTargetsDir()
	entries, err := os.ReadDir(targetsDir)
	if err != nil {
		return claimed
	}
	type targetEntry struct {
		Targets []string `json:"targets"`
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(targetsDir, e.Name()))
		if err != nil {
			continue
		}
		var targets []targetEntry
		if err := json.Unmarshal(data, &targets); err != nil {
			continue
		}
		for _, t := range targets {
			for _, addr := range t.Targets {
				if _, portStr, err := net.SplitHostPort(addr); err == nil {
					if p, err := strconv.Atoi(portStr); err == nil {
						claimed[p] = true
					}
				}
			}
		}
	}
	return claimed
}

func installedNetworkObservers() []string {
	targetsDir := api.GetPrometheusTargetsDir()
	entries, err := os.ReadDir(targetsDir)
	if err != nil {
		return nil
	}
	var namespaces []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			namespaces = append(namespaces, e.Name()[:len(e.Name())-len(".json")])
		}
	}
	return namespaces
}

func joinStrings(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

func NextFreeMetricsPort(start int) (int, error) {
	claimed := claimedMetricsPorts()
	for port := start; port <= 65535; port++ {
		if claimed[port] {
			continue
		}
		if !utils.TcpPortInUse("", port) {
			return port, nil
		}
	}
	return 0, fmt.Errorf("no available metrics port found")
}
