package gateway

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"strings"
)

func systemctlCommand(args ...string) (string, string, error) {
	cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	// Running
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
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
