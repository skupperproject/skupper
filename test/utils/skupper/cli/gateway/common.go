package gateway

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

func execCommand(command string, args ...string) (string, string, error) {
	cmd := exec.Command(command, args...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Running
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func systemctlCommand(args ...string) (string, string, error) {
	return execCommand("systemctl", append([]string{"--user"}, args...)...)
}

func SystemdUnitAvailable(gatewayName string) bool {
	s := gatewayName + ".service"
	stdout, _, err := systemctlCommand("list-unit-files", s)
	if err != nil {
		log.Printf("systemd user unit not found: %s - %s", s, err)
		log.Printf(stdout)
	} else if strings.Contains(stdout, "0 unit") {
		log.Printf(stdout)
		return false
	}
	return err == nil
}

func SystemdUnitEnabled(gatewayName string) bool {
	s := gatewayName + ".service"
	_, _, err := systemctlCommand("is-enabled", s)
	if err != nil {
		return false
	}
	return true
}

func GetSkupperDataHome() string {
	dataHome, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.local/share/skupper"
	} else {
		return dataHome + "/skupper"
	}
}

func GetSystemdUserHome() string {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.config/systemd/user"
	} else {
		return configHome + "/systemd/user"
	}
}

func IsDockerContainerRunning(containerName string) (bool, error) {
	return isContainerRunning("docker", containerName)
}

func IsPodmanContainerRunning(containerName string) (bool, error) {
	return isContainerRunning("podman", containerName)
}

func isContainerRunning(command, containerName string) (bool, error) {
	stdout, stderr, err := execCommand(command, "inspect", containerName)
	if err != nil {
		log.Printf("error inspecting %s container %s: %s - stderr: %s", command, containerName, err, stderr)
		return false, err
	}
	var containerInfo []map[string]interface{}
	_ = json.Unmarshal([]byte(stdout), &containerInfo)
	if len(containerInfo) != 1 || containerInfo[0] == nil {
		return false, fmt.Errorf("container not found: %s", containerName)
	}
	stateRaw, ok := containerInfo[0]["State"]
	if !ok {
		return false, nil
	}
	state := stateRaw.(map[string]interface{})
	return state["Running"].(bool), nil
}
