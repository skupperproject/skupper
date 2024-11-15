package bootstrap

import (
	"fmt"
	"github.com/skupperproject/skupper/pkg/config"
	"github.com/skupperproject/skupper/pkg/nonkube/api"
	"os"
	"os/exec"
	"path/filepath"
)

type LocalData struct {
	namespacesPath string
	servicePath    string
	userSvcFlag    bool
	service        string
}

func Teardown(namespace string) error {

	localData, err := getLocalData(namespace)
	if err != nil {
		return err
	}

	if err := removeRouter(namespace); err != nil {
		return err
	}

	if err := removeDefinition(localData.namespacesPath, namespace); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(localData.servicePath, localData.service)); err == nil {
		if err := removeService(localData.userSvcFlag, localData.servicePath, namespace); err != nil {
			return err
		}
	}

	fmt.Printf("Namespace \"%s\" has been removed\n", namespace)
	return nil

}

func Stop(namespace string) error {

	localData, err := getLocalData(namespace)
	if err != nil {
		return err
	}

	if err := removeRouter(namespace); err != nil {
		return err
	}

	if err := removeService(localData.userSvcFlag, localData.servicePath, namespace); err != nil {
		return err
	}

	return nil

}

func removeDefinition(namespacesPath string, namespace string) error {

	return os.RemoveAll(filepath.Join(namespacesPath, namespace))
}

func removeRouter(namespace string) error {
	skupperPlatform := config.GetPlatform()

	switch skupperPlatform {
	case "podman", "docker":
		err := exec.Command(string(skupperPlatform), "rm", "-f", namespace+"-skupper-router").Run()
		if err != nil {
			return err
		}
	}
	return nil
}

func removeService(userSvcFlag bool, servicepath string, namespace string) error {

	service := "skupper-" + namespace + ".service"

	cmdStopArgs := []string{"stop", service}
	cmdDisableArgs := []string{"disable", service}
	cmdReloadArgs := []string{"daemon-reload"}
	cmdResetArgs := []string{"reset-failed"}

	cmdStop := execCommand("systemctl", cmdStopArgs, userSvcFlag)
	cmdDisable := execCommand("systemctl", cmdDisableArgs, userSvcFlag)
	cmdReload := execCommand("systemctl", cmdReloadArgs, userSvcFlag)
	cmdReset := execCommand("systemctl", cmdResetArgs, userSvcFlag)

	err := cmdStop.Run()
	if err != nil {
		return fmt.Errorf("Failed to stop service %s: %s", service, err)
	}

	err = cmdDisable.Run()
	if err != nil {
		return fmt.Errorf("Failed to disable service %s: %s", service, err)
	}
	err = cmdReload.Run()
	if err != nil {
		return fmt.Errorf("Failed to reload systemd: %s", err)
	}
	err = cmdReset.Run()
	if err != nil {
		return fmt.Errorf("Failed to reset systemd: %s", err)
	}

	return os.Remove(filepath.Join(servicepath, service))

}

func execCommand(name string, args []string, userSvcFlag bool) *exec.Cmd {

	if userSvcFlag {
		args = append(args, "--user")
	}

	return exec.Command(name, args...)
}

func getLocalData(namespace string) (*LocalData, error) {

	uid := os.Geteuid()
	var namespacesPath string
	var servicePath string
	userSvcFlag := true

	if uid == 0 {
		namespacesPath = "/usr/local/share/skupper/namespaces"
		servicePath = "/etc/systemd/system"
		userSvcFlag = false
	} else {
		if xdgDataHome := api.GetDataHome(); xdgDataHome != "" {
			namespacesPath = filepath.Join(xdgDataHome, "namespaces")
		} else {
			namespacesPath = filepath.Join(os.Getenv("HOME"), ".local/share/skupper/namespaces")
		}

		if xdgConfigHome := api.GetConfigHome(); xdgConfigHome != "" {
			servicePath = filepath.Join(xdgConfigHome, "systemd/user")
		} else {
			servicePath = filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
		}
	}

	if _, err := os.Stat(filepath.Join(namespacesPath, namespace)); os.IsNotExist(err) {
		return nil, fmt.Errorf("Namespace \"%s\" does not exist\n", namespace)
	}

	service := "skupper-" + namespace + ".service"

	return &LocalData{
		namespacesPath: namespacesPath,
		servicePath:    servicePath,
		userSvcFlag:    userSvcFlag,
		service:        service,
	}, nil
}
