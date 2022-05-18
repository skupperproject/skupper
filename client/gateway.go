package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"gopkg.in/yaml.v3"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/google/uuid"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
	"github.com/skupperproject/skupper/pkg/utils"
)

const (
	GatewayServiceType         string = "service"
	GatewayDockerType          string = "docker"
	GatewayPodmanType          string = "podman"
	GatewayMockType            string = "mock"
	gatewayPrefix              string = "skupper-gateway-"
	gatewayIngress             string = "-ingress-"
	gatewayEgress              string = "-egress-"
	gatewayClusterDir          string = "/skupper/cluster/"
	gatewayBundleDir           string = "/skupper/bundle/"
	gatewayContainerWorkingDir string = "/opt/skupper"
)

type GatewayConfig struct {
	GatewayName  string                  `yaml:"name,omitempty"`
	QdrListeners []qdr.Listener          `yaml:"qdr-listeners,omitempty"`
	Bindings     []types.GatewayEndpoint `yaml:"bindings,omitempty"`
	Forwards     []types.GatewayEndpoint `yaml:"forwards,omitempty"`
}

type UnitInfo struct {
	IsSystemService bool
	Binary          string
	Image           string
	ConfigPath      string
	GatewayName     string
}

type GatewayInstance struct {
	WorkingDir string
	Hostname   string
	RouterID   string
}

func serviceForQdr(info UnitInfo) string {
	service := `
[Unit]
Description=Qpid Dispatch router daemon
{{- if .IsSystemService }}
Requires=network.target
After=network.target
{{- end }}

[Service]
Type=simple
ExecStart={{.Binary}} -c {{.ConfigPath}}/config/skrouterd.json

[Install]
{{- if .IsSystemService }}
WantedBy=multi-user.target
{{- else}}
WantedBy=default.target
{{- end}}
`
	var buf bytes.Buffer
	qdrService := template.Must(template.New("qdrService").Parse(service))
	qdrService.Execute(&buf, info)

	return buf.String()
}

func expandVars() string {
	expand := `
from __future__ import print_function
import sys
import os

try:
	filename = sys.argv[1]
	is_file = os.path.isfile(filename)
	if not is_file:
		raise Exception()
except Exception as e:
	print ("Usage: python3 expandvars.py <absolute_file_path>. Example - python3 expandvars.py /tmp/skrouterd.conf")
	## Unix programs generally use 2 for command line syntax errors
	sys.exit(2)

out_list = []
with open(filename) as f:
	for line in f:
		if line.startswith("#") or not '$' in line:
			out_list.append(line)
		else:
			out_list.append(os.path.expandvars(line))

with open(filename, 'w') as f:
	for out in out_list:
		f.write(out)
`
	return expand
}

func launchScript(info UnitInfo) string {
	launch := `
#!/bin/sh

while getopts t: flag
do
    case "${flag}" in
        t) type=${OPTARG};;
    esac
done

if [ -z "$type" ]; then
	type="service"
fi

if [ "$type" != "service" ] && [ "$type" != "docker" ] && [ "$type" != "podman" ]; then
    echo "gateway type must be one of service, docker or podman"
    exit
fi

gateway_name={{.GatewayName}}
gateway_image=${QDROUTERD_IMAGE:-{{.Image}}}

share_dir=${XDG_DATA_HOME:-~/.local/share}
config_dir=${XDG_CONFIG_HOME:-~/.config}

local_dir=$share_dir/skupper/bundle/$gateway_name
certs_dir=$share_dir/skupper/bundle/$gateway_name/skupper-router-certs
qdrcfg_dir=$share_dir/skupper/bundle/$gateway_name/config

if [[ -z "$(command -v python3 2>&1)" ]]; then
    echo "python3 could not be found. Please 'install python3'"
    exit
fi

if [ "$type" == "service" ]; then
    if result=$(command -v skrouterd 2>&1); then
        qdr_bin=$result
    else
        echo "skrouterd could not be found. Please 'install skrouterd'"
        exit
    fi    
    export QDR_CONF_DIR=$share_dir/skupper/bundle/$gateway_name
    export QDR_CONF_DIR=$share_dir/skupper/bundle/$gateway_name
    export QDR_BIN_PATH=${QDROUTERD_HOME:-$qdr_bin}  
else
	if [ "$type" == "docker" ]; then    
        if result=$(command -v docker 2>&1); then
            docker_bin=$result
        else
            echo "docker could not be found. Please install first"
            exit
        fi
	elif [ "$type" == "podman" ]; then
	    if result=$(command -v podman 2>&1); then
	        podman_bin=$result
        else
	        echo "podman could not be found. Please install first"
	        exit
        fi	
	fi
    export ROUTER_ID=$(uuidgen)
    export QDR_CONF_DIR=/opt/skupper
fi

mkdir -p $qdrcfg_dir
mkdir -p $certs_dir

cp -R ./skupper-router-certs/* $certs_dir
cp ./config/skrouterd.json $qdrcfg_dir
    
python3 ./expandvars.py $qdrcfg_dir/skrouterd.json

if [ "$type" == "service" ]; then
    mkdir -p $config_dir/systemd/user
    cp ./service/$gateway_name.service $config_dir/systemd/user/
    
    python3 ./expandvars.py $config_dir/systemd/user/$gateway_name.service
    
    systemctl --user enable $gateway_name.service
    systemctl --user daemon-reload
    systemctl --user start $gateway_name.service
    exit
elif [ "$type" == "docker" ] || [ "$type" == "podman" ]; then
    ${type} run --restart always -d --name ${gateway_name} --network host \
	   -e QDROUTERD_CONF_TYPE=json \
	   -e QDROUTERD_CONF=/opt/skupper/config/skrouterd.json \
	   -v ${local_dir}:${QDR_CONF_DIR}:Z \
	   ${gateway_image} 
    exit    
fi

`
	var buf bytes.Buffer
	launchScript := template.Must(template.New("launchScript").Parse(launch))
	launchScript.Execute(&buf, info)

	return buf.String()
}

func removeScript(info UnitInfo) string {
	remove := `
#!/bin/sh

gateway_name={{.GatewayName}}

share_dir=${XDG_DATA_HOME:-~/.local/share}
config_dir=${XDG_CONFIG_HOME:-~/.config}

if [ -f $config_dir/systemd/user/$gateway_name.service ]; then
    systemctl --user stop $gateway_name.service
    systemctl --user disable $gateway_name.service
    systemctl --user daemon-reload

    rm $config_dir/systemd/user/$gateway_name.service
elif [ $( docker ps -a | grep $gateway_name | wc -l ) -gt 0 ]; then
    docker rm -f $gateway_name
elif [ $( podman ps -a | grep $gateway_name | wc -l ) -gt 0 ]; then
    podman rm -f $gateway_name
fi

if [ -d $share_dir/skupper/bundle/$gateway_name ]; then
    rm -rf $share_dir/skupper/bundle/$gateway_name
fi

`
	var buf bytes.Buffer
	removeScript := template.Must(template.New("removeScript").Parse(remove))
	removeScript.Execute(&buf, info)

	return buf.String()
}

func getDataHome() string {
	dataHome, ok := os.LookupEnv("XDG_DATA_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.local/share"
	} else {
		return dataHome
	}
}

func getConfigHome() string {
	configHome, ok := os.LookupEnv("XDG_CONFIG_HOME")
	if !ok {
		homeDir, _ := os.UserHomeDir()
		return homeDir + "/.config"
	} else {
		return configHome
	}
}

func getGatewaySiteId(gatewayDir string) (string, error) {
	siteId, err := ioutil.ReadFile(gatewayDir + "/config/siteid.txt")
	if err != nil {
		return "", fmt.Errorf("Failed to read site id: %w", err)
	}
	return string(siteId), nil
}

func getRouterId(gatewayDir string) (string, error) {
	routerId, err := ioutil.ReadFile(gatewayDir + "/config/routerid.txt")
	if err != nil {
		return "", fmt.Errorf("Failed to read router id: %w", err)
	}
	return string(routerId), nil
}

func getRouterUrl(gatewayDir string) (string, error) {
	url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
	if err != nil {
		return "", fmt.Errorf("Failed to read router url: %w", err)
	}
	return string(url), nil
}

func getRouterVersion(gatewayName string, gatewayType string) (string, error) {
	var err error
	routerVersion := []byte{}

	if gatewayType == GatewayServiceType {
		routerVersion, err = exec.Command("skrouterd", "-v").Output()
	} else if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		if isActive(gatewayName, gatewayType) {
			routerVersion, err = exec.Command(gatewayType, "exec", gatewayName, "skrouterd", "-v").Output()
		} else {
			routerVersion = []byte("Unknown (container not running)")
		}
	} else if gatewayType == GatewayMockType {
		routerVersion = []byte("1.2.3")
	}

	if err != nil {
		return "", err
	} else {
		return strings.Trim(string(routerVersion), "\n"), nil
	}
}

func getMachineID() (string, error) {
	id, err := ioutil.ReadFile("/var/lib/dbus/machine-id")
	if err != nil {
		id, err = ioutil.ReadFile("/etc/machine-id")
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Trim(string(id), "\n")), nil
}

func clusterGatewayName(gatewayName string) string {
	machineId, _ := getMachineID()
	mac := hmac.New(sha1.New, []byte(machineId))
	mac.Write([]byte(gatewayName))
	return gatewayPrefix + hex.EncodeToString(mac.Sum(nil))
}

func newUUID() string {
	return uuid.New().String()
}

func checkPortFree(protocol, port string) bool {
	l, err := net.Listen(protocol, ":"+port)
	if err != nil {
		return false
	} else {
		defer l.Close()
		return true
	}
}

func GetFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return 0, err
}

func (cli *VanClient) getGatewayType(gatewayName string) (string, error) {
	configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return "", err
	}
	gatewayType, ok := configmap.ObjectMeta.Annotations["skupper.io/gateway-type"]
	if !ok {
		return "", fmt.Errorf("Unable to get gateway type")
	}
	return gatewayType, nil
}

func isValidGatewayType(gatewayType string) bool {
	if gatewayType == GatewayServiceType || gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType || gatewayType == GatewayMockType {
		return true
	} else {
		return false
	}
}

func isActive(gatewayName string, gatewayType string) bool {
	if gatewayType == GatewayServiceType {
		out, err := exec.Command("systemctl", "--user", "check", gatewayName).Output()
		if err == nil {
			if strings.Trim(string(out), "\n") == "active" {
				return true
			} else {
				return false
			}
		} else {
			return false
		}
	} else if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		running, err := exec.Command(gatewayType, "inspect", "--format", "'{{json .State.Running}}'", gatewayName).Output()
		if err == nil {
			bv, _ := strconv.ParseBool(strings.Trim(string(running), "'\n"))
			return bv
		} else {
			return false
		}
	} else if gatewayType == GatewayMockType {
		return true
	} else {
		return false
	}
}

func waitForGatewayActive(gatewayName string, gatewayType string, timeout, interval time.Duration) error {
	var err error

	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err = utils.RetryWithContext(ctx, interval, func() (bool, error) {
		isActive := isActive(gatewayName, gatewayType)
		return isActive, nil
	})

	return err
}

func getUserDefaultGatewayName() (string, error) {
	hostname, _ := os.Hostname()

	u, err := user.Current()
	if err != nil {
		return "", err
	}
	name := strings.Join(strings.Fields(u.Username), "")
	return hostname + "-" + strings.ToLower(name), nil
}

func setupLocalDir(localDir string) error {
	_ = os.RemoveAll(localDir)

	if err := os.MkdirAll(localDir+"/config", 0755); err != nil {
		return fmt.Errorf("Unable to create config directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/user", 0755); err != nil {
		return fmt.Errorf("Unable to create user directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/system", 0755); err != nil {
		return fmt.Errorf("Unable to create system directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/skupper-router-certs/conn1-profile", 0755); err != nil {
		return fmt.Errorf("Unable to create certs directory: %w", err)
	}

	return nil
}

func startGatewayUserService(gatewayName, unitDir, localDir string) error {

	unitFile, err := ioutil.ReadFile(localDir + "/user/" + gatewayName + ".service")
	if err != nil {
		return fmt.Errorf("Unable to read service file: %w", err)
	}

	err = ioutil.WriteFile(unitDir+"/"+gatewayName+".service", unitFile, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write user unit file: %w", err)
	}

	cmd := exec.Command("systemctl", "--user", "enable", gatewayName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "start", gatewayName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}
	return nil
}

func stopGatewayUserService(unitDir, gatewayName string) error {
	systemctlCommands := [][]string{
		{"--user", "stop", gatewayName + ".service"},
		{"--user", "disable", gatewayName + ".service"},
		{"--user", "daemon-reload"},
	}

	// Note: will not error out stopping service
	for _, args := range systemctlCommands {
		cmd := exec.Command("systemctl", args...)
		cmd.Run()
	}

	err := os.Remove(unitDir + "/" + gatewayName + ".service")
	if err != nil {
		return fmt.Errorf("Unable to remove user service file: %w", err)
	}

	return nil
}

func updateLocalGatewayConfig(gatewayDir string, gatewayType string, gatewayConfig qdr.RouterConfig) error {
	mc, err := qdr.MarshalRouterConfig(gatewayConfig)
	if err != nil {
		return fmt.Errorf("Failed to marshall router config: %w", err)
	}

	hostname, _ := os.Hostname()

	routerId, err := getRouterId(gatewayDir)
	if err != nil {
		return err
	}

	workingDir := gatewayDir
	if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		workingDir = gatewayContainerWorkingDir
	}
	instance := GatewayInstance{
		WorkingDir: workingDir,
		RouterID:   routerId,
		Hostname:   hostname,
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	err = ioutil.WriteFile(gatewayDir+"/config/skrouterd.json", buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}
	return nil
}

func (cli *VanClient) setupGatewayConfig(ctx context.Context, gatewayName string, gatewayType string) error {
	gatewayResourceName := clusterGatewayName(gatewayName)
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	if gatewayType == "" {
		gatewayType = "service"
	}

	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	err := setupLocalDir(gatewayDir)
	if err != nil {
		return err
	}

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayResourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to retreive external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = ioutil.WriteFile(gatewayDir+"/skupper-router-certs/conn1-profile/"+cert, secret.Data[cert], 0644)
		if err != nil {
			return fmt.Errorf("Failed to write cert file: %w", err)
		}
	}

	configmap, err := kube.GetConfigMap(gatewayResourceName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	amqpPort, err := GetFreePort()
	if err != nil {
		return fmt.Errorf("Could not aquire free port: %w", err)
	}
	gatewayConfig.Listeners["amqp"] = qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: int32(amqpPort),
	}

	// store the url for instance queries
	url := fmt.Sprintf("amqp://127.0.0.1:%s", strconv.Itoa(amqpPort))
	err = ioutil.WriteFile(gatewayDir+"/config/url.txt", []byte(url), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write instance url file: %w", err)
	}

	// generate a router id and store it for subsequent template updates
	routerId := newUUID()
	err = ioutil.WriteFile(gatewayDir+"/config/routerid.txt", []byte(routerId), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write instance id file: %w", err)
	}

	// store the site id to prevent second gateway to different site
	siteConfig, err := cli.SiteConfigInspect(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to retrieve site id: %w", err)
	}
	if siteConfig != nil {
		err = ioutil.WriteFile(gatewayDir+"/config/siteid.txt", []byte(siteConfig.Reference.UID), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write site id file: %w", err)
		}
	}

	// Iterate through the config and check free ports, get port if in use
	for name, tcpListener := range gatewayConfig.Bridges.TcpListeners {
		if !checkPortFree("tcp", tcpListener.Port) {
			portToUse, err := GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			}
			tcpListener.Port = strconv.Itoa(portToUse)
			gatewayConfig.Bridges.TcpListeners[name] = tcpListener
		}
	}
	for name, httpListener := range gatewayConfig.Bridges.HttpListeners {
		if !checkPortFree("tcp", httpListener.Port) {
			portToUse, err := GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			}
			httpListener.Port = strconv.Itoa(portToUse)
			gatewayConfig.Bridges.HttpListeners[name] = httpListener
		}
	}

	mc, _ := qdr.MarshalRouterConfig(*gatewayConfig)

	hostname, _ := os.Hostname()
	workingDir := gatewayDir
	if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		workingDir = gatewayContainerWorkingDir
	}
	instance := GatewayInstance{
		WorkingDir: workingDir,
		RouterID:   routerId,
		Hostname:   hostname,
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	err = ioutil.WriteFile(gatewayDir+"/config/skrouterd.json", buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}

	return nil
}

func (cli *VanClient) gatewayStartService(ctx context.Context, gatewayName string, routerPath string) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	svcDir := getConfigHome() + "/systemd/user"

	err := cli.setupGatewayConfig(context.Background(), gatewayName, GatewayServiceType)
	if err != nil {
		return fmt.Errorf("Failed to setup gateway local directories: %w", err)
	}

	err = os.MkdirAll(svcDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create gateway service directory: %w", err)
	}

	qdrUserUnit := serviceForQdr(UnitInfo{
		IsSystemService: false,
		Binary:          routerPath,
		Image:           "",
		ConfigPath:      gatewayDir,
		GatewayName:     gatewayName,
	})
	err = ioutil.WriteFile(gatewayDir+"/user/"+gatewayName+".service", []byte(qdrUserUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write unit file: %w", err)
	}

	err = startGatewayUserService(gatewayName, svcDir, gatewayDir)
	if err != nil {
		return fmt.Errorf("Failed to create user service: %w", err)
	}

	return nil
}

func (cli *VanClient) gatewayStopService(ctx context.Context, gatewayName string) error {
	svcDir := getConfigHome() + "/systemd/user"

	if gatewayName == "" {
		return fmt.Errorf("Unable to delete gateway definition, need gateway name")
	}

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	}

	if isActive(gatewayName, GatewayServiceType) {
		stopGatewayUserService(svcDir, gatewayName)
	}

	return nil
}

func (cli *VanClient) gatewayStartContainer(ctx context.Context, gatewayName string, gatewayType string) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	containerDir := "/opt/skupper"

	err := cli.setupGatewayConfig(context.Background(), gatewayName, gatewayType)
	if err != nil {
		return fmt.Errorf("Failed to setup gateway local directories: %w", err)
	}

	containerCmd := gatewayType
	containerCmdArgs := []string{
		"run",
		"--restart",
		"always",
		"-d",
		"--name",
		gatewayName,
		"--network",
		"host",
		"-e",
		"QDROUTERD_CONF_TYPE=json",
		"-e",
		"QDROUTERD_CONF=" + containerDir + "/config/skrouterd.json",
		"-v",
		gatewayDir + ":" + containerDir + ":Z",
		GetRouterImageName(),
	}

	cmd := exec.Command(containerCmd, containerCmdArgs...)
	result, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to start gateway as container: %s", result)
	}

	return nil
}

func (cli *VanClient) gatewayStopContainer(ctx context.Context, gatewayName string, gatewayType string) []string {
	errs := []string{}

	if gatewayName == "" {
		errs = append(errs, fmt.Sprintf("Unable to delete gateway definition, need gateway name"))
		return errs
	}

	_, err := getRootObject(cli)
	if err != nil {
		errs = append(errs, fmt.Sprintf("Skupper is not enabled in namespace '%s'", cli.Namespace))
	}

	containerCmd := gatewayType
	containerCmdArgs := []string{
		"rm",
		"-f",
		gatewayName,
	}
	cmd := exec.Command(containerCmd, containerCmdArgs...)
	result, err := cmd.CombinedOutput()
	if err != nil {
		errs = append(errs, fmt.Sprintf("Unable to remove gateway container: %s", result))
	}

	return errs
}

func (a *GatewayConfig) getBridgeConfig(gatewayName string, routerId string) (*qdr.BridgeConfig, error) {
	bc := qdr.NewBridgeConfig()

	for _, binding := range a.Bindings {
		for i, _ := range binding.TargetPorts {
			name := gatewayName + gatewayEgress + binding.Service.Address
			switch binding.Service.Protocol {
			case "tcp":
				bc.AddTcpConnector(qdr.TcpEndpoint{
					Name:    name,
					Host:    binding.Host,
					Port:    strconv.Itoa(binding.TargetPorts[i]),
					Address: binding.Service.Address,
					SiteId:  routerId,
				})
			case "http":
				bc.AddHttpConnector(qdr.HttpEndpoint{
					Name:            name,
					Host:            binding.Host,
					Port:            strconv.Itoa(binding.TargetPorts[i]),
					Address:         binding.Service.Address,
					ProtocolVersion: qdr.HttpVersion1,
					Aggregation:     binding.Service.Aggregate,
					EventChannel:    binding.Service.EventChannel,
					SiteId:          routerId,
				})
			case "http2":
				bc.AddHttpConnector(qdr.HttpEndpoint{
					Name:            name,
					Host:            binding.Host,
					Port:            strconv.Itoa(binding.TargetPorts[i]),
					Address:         binding.Service.Address,
					ProtocolVersion: qdr.HttpVersion2,
					Aggregation:     binding.Service.Aggregate,
					EventChannel:    binding.Service.EventChannel,
					SiteId:          routerId,
				})
			default:
			}
		}
	}
	for _, forward := range a.Forwards {
		for i, _ := range forward.TargetPorts {
			name := gatewayName + gatewayIngress + forward.Service.Address
			switch forward.Service.Protocol {
			case "tcp":
				bc.AddTcpListener(qdr.TcpEndpoint{
					Name:    name,
					Host:    forward.Host,
					Port:    strconv.Itoa(forward.Service.Ports[i]),
					Address: forward.Service.Address,
					SiteId:  routerId,
				})
			case "http":
				bc.AddHttpListener(qdr.HttpEndpoint{
					Name:            name,
					Host:            forward.Host,
					Port:            strconv.Itoa(forward.Service.Ports[i]),
					Address:         forward.Service.Address,
					ProtocolVersion: qdr.HttpVersion1,
					Aggregation:     forward.Service.Aggregate,
					EventChannel:    forward.Service.EventChannel,
					SiteId:          routerId,
				})
			case "http2":
				bc.AddHttpListener(qdr.HttpEndpoint{
					Name:            name,
					Host:            forward.Host,
					Port:            strconv.Itoa(forward.Service.Ports[i]),
					Address:         forward.Service.Address,
					ProtocolVersion: qdr.HttpVersion2,
					Aggregation:     forward.Service.Aggregate,
					EventChannel:    forward.Service.EventChannel,
					SiteId:          routerId,
				})
			default:
			}
		}
	}
	return &bc, nil
}

func (cli *VanClient) newGateway(ctx context.Context, gatewayName string, gatewayType string, configFile string, owner *metav1.OwnerReference) (string, error) {
	var err error
	var routerPath string = ""
	gatewayResourceName := clusterGatewayName(gatewayName)

	if gatewayType == GatewayServiceType {
		routerPath, err = exec.LookPath("skrouterd")
		if err != nil {
			return "", fmt.Errorf("skrouterd not found, please 'dnf install skupper-router' first or use --type [docker|podman]")
		}
	}

	secret, _, err := cli.ConnectorTokenCreate(context.Background(), gatewayResourceName, "")
	if err != nil {
		return "", fmt.Errorf("Failed to create gateway token: %w", err)
	}
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
	if secret.Labels == nil {
		secret.Labels = map[string]string{}
	}
	secret.Labels[types.SkupperTypeQualifier] = types.TypeGatewayToken
	_, err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Create(secret)
	if err != nil {
		return "", fmt.Errorf("Failed to create gateway secret: %w", err)
	}

	routerConfig := qdr.InitialConfig(clusterGatewayName(gatewayName), "{{.RouterID}}", Version, true, 3)

	// NOTE: at instantiation time detect amqp port in use and allocate port if needed
	routerConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})

	routerConfig.AddSslProfileWithPath("{{.WorkingDir}}/skupper-router-certs", qdr.SslProfile{
		Name: "conn1-profile",
	})
	connector := qdr.Connector{
		Name:             "conn1",
		Cost:             1,
		SslProfile:       "conn1-profile",
		MaxFrameSize:     16384,
		MaxSessionFrames: 640,
	}
	connector.Host = secret.ObjectMeta.Annotations["edge-host"]
	connector.Port = secret.ObjectMeta.Annotations["edge-port"]
	connector.Role = qdr.RoleEdge

	routerConfig.AddConnector(connector)

	if configFile != "" {
		// grab the bindings and forwards from the config file
		yamlFile, err := ioutil.ReadFile(configFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read gateway config file: %w", err)
		}
		gatewayConfig := GatewayConfig{}
		err = yaml.Unmarshal(yamlFile, &gatewayConfig)
		if err != nil {
			return "", fmt.Errorf("Failed to unmarshal gateway config file: %w", err)
		}

		// TODO: how to deal with service dependencies (e.g. how to know that we should create them)
		for _, binding := range gatewayConfig.Bindings {
			for i, _ := range binding.TargetPorts {
				name := gatewayName + gatewayEgress + binding.Service.Address
				switch binding.Service.Protocol {
				case "tcp":
					routerConfig.AddTcpConnector(qdr.TcpEndpoint{
						Name:    name,
						Host:    binding.Host,
						Port:    strconv.Itoa(binding.TargetPorts[i]),
						Address: binding.Service.Address,
						SiteId:  "{{.RouterID}}",
					})
				case "http":
					routerConfig.AddHttpConnector(qdr.HttpEndpoint{
						Name:            name,
						Host:            binding.Host,
						Port:            strconv.Itoa(binding.TargetPorts[i]),
						Address:         binding.Service.Address,
						ProtocolVersion: qdr.HttpVersion1,
						Aggregation:     binding.Service.Aggregate,
						EventChannel:    binding.Service.EventChannel,
						SiteId:          "{{.RouterID}}",
					})
				case "http2":
					routerConfig.AddHttpConnector(qdr.HttpEndpoint{
						Name:            name,
						Host:            binding.Host,
						Port:            strconv.Itoa(binding.TargetPorts[i]),
						Address:         binding.Service.Address,
						ProtocolVersion: qdr.HttpVersion2,
						Aggregation:     binding.Service.Aggregate,
						EventChannel:    binding.Service.EventChannel,
						SiteId:          "{{.RouterID}}",
					})
				default:
				}
			}
		}

		for _, forward := range gatewayConfig.Forwards {
			for i, _ := range forward.TargetPorts {
				name := gatewayName + gatewayIngress + forward.Service.Address
				switch forward.Service.Protocol {
				case "tcp":
					routerConfig.AddTcpListener(qdr.TcpEndpoint{
						Name:    name,
						Host:    forward.Host,
						Port:    strconv.Itoa(forward.Service.Ports[i]),
						Address: forward.Service.Address,
						SiteId:  "{{.RouterID}}",
					})
				case "http":
					routerConfig.AddHttpListener(qdr.HttpEndpoint{
						Name:            name,
						Host:            forward.Host,
						Port:            strconv.Itoa(forward.Service.Ports[i]),
						Address:         forward.Service.Address,
						ProtocolVersion: qdr.HttpVersion1,
						Aggregation:     forward.Service.Aggregate,
						EventChannel:    forward.Service.EventChannel,
						SiteId:          "{{.RouterID}}",
					})
				case "http2":
					routerConfig.AddHttpListener(qdr.HttpEndpoint{
						Name:            name,
						Host:            forward.Host,
						Port:            strconv.Itoa(forward.Service.Ports[i]),
						Address:         forward.Service.Address,
						ProtocolVersion: qdr.HttpVersion2,
						Aggregation:     forward.Service.Aggregate,
						EventChannel:    forward.Service.EventChannel,
						SiteId:          "{{.RouterID}}",
					})
				default:
				}
			}
		}
	}

	mapData, err := routerConfig.AsConfigMapData()
	labels := map[string]string{
		"skupper.io/type": "gateway-definition",
	}
	annotations := map[string]string{
		"skupper.io/gateway-type": gatewayType,
		"skupper.io/gateway-name": gatewayName,
	}
	_, err = kube.NewConfigMap(gatewayResourceName, &mapData, &labels, &annotations, owner, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return "", fmt.Errorf("Failed to create gateway config map: %w", err)
	}

	if gatewayType == GatewayServiceType {
		err = cli.gatewayStartService(ctx, gatewayName, routerPath)
	} else if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		err = cli.gatewayStartContainer(ctx, gatewayName, gatewayType)
	}
	if err != nil {
		return gatewayName, err
	}

	return gatewayName, nil
}

func GatewayDetectTypeIfPresent() (string, error) {
	gatewayName, err := getUserDefaultGatewayName()
	if err != nil {
		return "", fmt.Errorf("Unable to generate gateway name: %w", err)
	}

	gatewayType := ""
	out, err := exec.Command("systemctl", "--user", "check", gatewayName).Output()
	if err == nil {
		if strings.Trim(string(out), "\n") == "active" {
			gatewayType = GatewayServiceType
		}
	}
	if gatewayType == "" {
		// check for podman container
		_, err := exec.LookPath("podman")
		if err == nil {
			_, err := exec.Command("podman", "inspect", "--format", "'{{json .State}}'", gatewayName).Output()
			if err == nil {
				gatewayType = GatewayPodmanType
			}
		}
	}
	if gatewayType == "" {
		// check for docker container
		_, err := exec.LookPath("docker")
		if err == nil {
			_, err := exec.Command("docker", "inspect", "--format", "'{{json .State}}'", gatewayName).Output()
			if err == nil {
				gatewayType = GatewayDockerType
			}
		}
	}
	return gatewayType, nil
}

func (cli *VanClient) GatewayInit(ctx context.Context, gatewayName string, gatewayType string, configFile string) (string, error) {
	var err error

	policy := NewPolicyValidatorAPI(cli)
	policyRes, err := policy.IncomingLink()
	if err != nil {
		return "", err
	}
	if !policyRes.Allowed {
		return "", policyRes.Err()
	}

	if gatewayType == "" {
		gatewayType = GatewayServiceType
	}
	if !isValidGatewayType(gatewayType) {
		return "", fmt.Errorf("Invalid gateway type %s must be one of 'service', 'docker', 'podman', or 'export'", gatewayType)
	}

	if gatewayName != "" {
		nameRegex := regexp.MustCompile(`^[a-z]([a-z0-9\.-]*[a-z0-9])*$`)
		if !nameRegex.MatchString(gatewayName) {
			return "", fmt.Errorf("Gateway name must consist of lower case letters, numerals and '-'. Must start with a letter.")
		}
	} else {
		gatewayName, err = getUserDefaultGatewayName()
		if err != nil {
			return "", fmt.Errorf("Unable to generate gateway name: %w", err)
		}
	}

	owner, err := getRootObject(cli)
	if err != nil {
		return "", fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	// check if gw to different sites exists on host, only single host/gw/site allowed
	existing, err := getGatewaySiteId(gatewayDir)
	if err == nil {
		siteConfig, err := cli.SiteConfigInspect(ctx, nil)
		if err != nil {
			return "", fmt.Errorf("Failed to retrieve site id: %w", err)
		}
		if siteConfig != nil {
			if siteConfig.Reference.UID != existing {
				return "", fmt.Errorf("gateway not created as existing gateway detected for site: %s", existing)
			}
		}
	}

	configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
	if err == nil {
		gwType, ok := configmap.ObjectMeta.Annotations["skupper.io/gateway-type"]
		if ok && gwType != gatewayType {
			return "", fmt.Errorf("gateway previously created as %s type, delete current gateway to change to %s type", gwType, gatewayType)
		}
	} else {
		return cli.newGateway(ctx, gatewayName, gatewayType, configFile, owner)
	}

	if configFile != "" && gatewayType != GatewayMockType {
		yamlFile, err := ioutil.ReadFile(configFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read gateway config file: %w", err)
		}

		gatewayConfigFromFile := GatewayConfig{}
		err = yaml.Unmarshal(yamlFile, &gatewayConfigFromFile)
		if err != nil {
			return "", fmt.Errorf("Failed to unmarshal gateway config file: %w", err)
		}

		routerId, _ := getRouterId(gatewayDir)
		bridgeConfigFromFile, _ := gatewayConfigFromFile.getBridgeConfig(gatewayName, routerId)

		var unretryable error = nil
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)

			routerConfigCurrent, err := qdr.GetRouterConfigFromConfigMap(configmap)
			bcDiff := routerConfigCurrent.Bridges.Difference(bridgeConfigFromFile)

			if bcDiff != (&qdr.BridgeConfigDifference{}) {
				routerConfigCurrent.Bridges = *bridgeConfigFromFile
				routerConfigCurrent.UpdateConfigMap(configmap)

				_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
				if err != nil {
					return err
				}

				err = applyBridgeDifferences(bcDiff, gatewayDir)
				if err != nil {
					unretryable = err
					return nil
				}

				_, err = os.Stat(gatewayDir + "/config/skrouterd.json")
				if err == nil {
					err = updateLocalGatewayConfig(gatewayDir, gatewayType, *routerConfigCurrent)
					if err != nil {
						unretryable = err
						return nil
					}
				}
			}
			return nil
		})
		if unretryable != nil {
			return gatewayName, unretryable
		}
	}
	return gatewayName, nil
}

func (cli *VanClient) GatewayDownload(ctx context.Context, gatewayName string, downloadPath string) (string, error) {
	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}
	gatewayResourceName := clusterGatewayName(gatewayName)

	tarFile, err := os.Create(downloadPath + "/" + gatewayName + ".tar.gz")
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Unable to create download file: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayResourceName, metav1.GetOptions{})
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = writeTar("skupper-router-certs/conn1-profile/"+cert, secret.Data[cert], time.Now(), tw)
		if err != nil {
			return tarFile.Name(), err
		}
	}

	configmap, err := kube.GetConfigMap(gatewayResourceName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	mc, _ := qdr.MarshalRouterConfig(*gatewayConfig)

	instance := GatewayInstance{
		WorkingDir: "${QDR_CONF_DIR}",
		RouterID:   "${ROUTER_ID}",
		Hostname:   "${HOSTNAME}",
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	err = writeTar("config/skrouterd.json", buf.Bytes(), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	gatewayInfo := UnitInfo{
		IsSystemService: false,
		Binary:          "${QDR_BIN_PATH}",
		Image:           GetRouterImageName(),
		ConfigPath:      "${QDR_CONF_DIR}",
		GatewayName:     gatewayName,
	}

	qdrUserUnit := serviceForQdr(gatewayInfo)
	err = writeTar("service/"+gatewayName+".service", []byte(qdrUserUnit), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	launch := launchScript(gatewayInfo)
	err = writeTar("launch.sh", []byte(launch), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	remove := removeScript(gatewayInfo)
	err = writeTar("remove.sh", []byte(remove), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	expand := expandVars()
	err = writeTar("expandvars.py", []byte(expand), time.Now(), tw)
	return tarFile.Name(), nil
}

func (cli *VanClient) GatewayRemove(ctx context.Context, gatewayName string) error {
	errs := []string{}
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}
	gatewayResourceName := clusterGatewayName(gatewayName)

	_, err := kube.GetConfigMap(gatewayResourceName, cli.GetNamespace(), cli.KubeClient)
	if errors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("Config map for gateway %s not found", gatewayResourceName))
	}

	gatewayType, err := cli.getGatewayType(gatewayName)
	if err != nil {
		// this is exceptional so inspect host to detect gatewayType
		gatewayType, err = GatewayDetectTypeIfPresent()
		if err == nil && gatewayType != "" {
			errs = append(errs, fmt.Sprintf("Unable to retrieve gateway type from config map: detected type %s", gatewayType))
		}
	}

	if gatewayType == GatewayServiceType {
		err = cli.gatewayStopService(ctx, gatewayName)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Unable to stop gateway %s", err))
		}
	} else if gatewayType == GatewayDockerType || gatewayType == GatewayPodmanType {
		containerErrs := cli.gatewayStopContainer(ctx, gatewayName, gatewayType)
		if len(containerErrs) > 0 {
			errs = append(errs, containerErrs...)
		}
	}

	svcList, err := cli.KubeClient.CoreV1().Services(cli.GetNamespace()).List(metav1.ListOptions{LabelSelector: types.GatewayQualifier + "=" + gatewayName})
	if err == nil {
		for _, service := range svcList.Items {
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				si, err := cli.ServiceInterfaceInspect(ctx, service.Name)
				if err != nil {
					errs = append(errs, fmt.Sprintf("Failed to retrieve service %s: %s", service.Name, err))
					return nil
				}
				if si != nil && len(si.Targets) == 0 && si.Origin == "" {
					err := cli.ServiceInterfaceRemove(ctx, service.Name)
					if err != nil {
						errs = append(errs, fmt.Sprintf("Failed to remove %s service: %s", service.Name, err))
					}
					return nil
				} else {
					delete(service.ObjectMeta.Labels, types.GatewayQualifier)
					_, err = cli.KubeClient.CoreV1().Services(cli.GetNamespace()).Update(&service)
					return err
				}
			})
		}
	}

	err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Delete(gatewayResourceName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("Unable to remove gateway secret: %s", err))
	}

	err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Delete(gatewayResourceName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Sprintf("Unable to remove gateway config map: %s", err))
	}

	// finally, remove local files
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	err = os.RemoveAll(gatewayDir)
	if err != nil && !os.IsNotExist(err) {
		errs = append(errs, fmt.Sprintf("Unable to remove gateway local directory %s: %s", gatewayDir, err.Error()))
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, ","))
	} else {
		return nil
	}
}

func convert(from interface{}, to interface{}) error {
	data, err := json.Marshal(from)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, to)
	if err != nil {
		return err
	}
	return nil
}

func getEntity(protocol string, endpointType string) string {
	if protocol == "tcp" {
		if endpointType == gatewayIngress {
			return "io.skupper.router.tcpListener"
		} else {
			return "io.skupper.router.tcpConnector"
		}
	} else if protocol == "http" || protocol == "http2" {
		if endpointType == gatewayIngress {
			return "io.skupper.router.httpListener"
		} else {
			return "io.skupper.router.httpConnector"
		}
	}
	return ""
}

func applyBridgeDifferences(bcDiff *qdr.BridgeConfigDifference, gatewayDir string) error {
	routerId, err := getRouterId(gatewayDir)
	if err != nil {
		return err
	}
	url, err := getRouterUrl(gatewayDir)
	if err != nil {
		return err
	}

	agent, err := qdr.Connect(url, nil)
	if err != nil {
		return fmt.Errorf("qdr agent error: %w", err)
	}
	defer agent.Close()

	for _, deleted := range bcDiff.TcpConnectors.Deleted {
		if err = agent.Delete(getEntity("tcp", gatewayEgress), deleted); err != nil {
			return fmt.Errorf("Error removing entity connector: %w", err)
		}
	}
	for _, added := range bcDiff.TcpConnectors.Added {
		record := map[string]interface{}{}
		added.SiteId = routerId
		if err = convert(added, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("tcp", gatewayEgress), added.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp entity: %w", err)
		}
	}

	for _, deleted := range bcDiff.TcpListeners.Deleted {
		if err = agent.Delete(getEntity("tcp", gatewayIngress), deleted); err != nil {
			return fmt.Errorf("Error removing entity listener: %w", err)
		}
	}
	for _, added := range bcDiff.TcpListeners.Added {
		record := map[string]interface{}{}
		added.SiteId = routerId
		if err = convert(added, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("tcp", gatewayIngress), added.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp entity: %w", err)
		}
	}

	for _, deleted := range bcDiff.HttpConnectors.Deleted {
		if err = agent.Delete(getEntity("http", gatewayEgress), deleted.Name); err != nil {
			return fmt.Errorf("Error removing entity connector: %w", err)
		}
	}
	for _, added := range bcDiff.HttpConnectors.Added {
		record := map[string]interface{}{}
		added.SiteId = routerId
		if err = convert(added, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("http", gatewayEgress), added.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp entity: %w", err)
		}
	}

	for _, deleted := range bcDiff.HttpListeners.Deleted {
		if err = agent.Delete(getEntity("http", gatewayIngress), deleted.Name); err != nil {
			return fmt.Errorf("Error removing entity listener: %w", err)
		}
	}
	for _, added := range bcDiff.HttpListeners.Added {
		record := map[string]interface{}{}
		added.SiteId = routerId
		if err = convert(added, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("http", gatewayIngress), added.Name, record); err != nil {
			return fmt.Errorf("Error adding http entity: %w", err)
		}
	}
	return nil
}

func (cli *VanClient) gatewayBridgeEndpointUpdate(ctx context.Context, gatewayName string, gatewayDirection string, addEndpoint bool, endpoint types.GatewayEndpoint) error {
	service := endpoint.Service

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	}

	si, err := cli.ServiceInterfaceInspect(ctx, service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}
	if si == nil {
		return fmt.Errorf("Unable to update gateway endpoint, service not found for %s", service.Address)
	}
	if addEndpoint && gatewayDirection == gatewayEgress && len(si.Ports) != len(service.Ports) {
		return fmt.Errorf("Unable to update gateway endpoint, the given service provides %d ports, but only %d provided", len(si.Ports), len(service.Ports))
	}

	gatewayType, err := cli.getGatewayType(gatewayName)
	if err != nil {
		return err
	}

	routerId := "314159"
	if gatewayType != GatewayMockType {
		routerId, err = getRouterId(gatewayDir)
		if err != nil {
			return err
		}
	}

	ifc := ""
	if endpoint.Loopback {
		ifc = "127.0.0.1"
	}

	var unretryable error = nil
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
		if err != nil {
			unretryable = fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
			return nil
		}
		currentGatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
		if err != nil {
			unretryable = fmt.Errorf("Failed to parse gateway configmap: %w", err)
			return nil
		}

		newBridges := qdr.NewBridgeConfigCopy(currentGatewayConfig.Bridges)

		if addEndpoint {
			for i, _ := range service.Ports {
				if gatewayDirection == gatewayEgress {
					name := fmt.Sprintf("%s:%d", gatewayName+gatewayEgress+service.Address, si.Ports[i])
					switch si.Protocol {
					case "tcp":
						newBridges.AddTcpConnector(qdr.TcpEndpoint{
							Name:    name,
							Host:    endpoint.Host,
							Port:    strconv.Itoa(service.Ports[i]),
							Address: fmt.Sprintf("%s:%d", service.Address, si.Ports[i]),
							SiteId:  routerId,
						})
					case "http", "http2":
						pv := qdr.HttpVersion1
						if si.Protocol == "http2" {
							pv = qdr.HttpVersion2
						}
						newBridges.AddHttpConnector(qdr.HttpEndpoint{
							Name:            name,
							Host:            endpoint.Host,
							Port:            strconv.Itoa(endpoint.Service.Ports[i]),
							Address:         fmt.Sprintf("%s:%d", endpoint.Service.Address, si.Ports[i]),
							ProtocolVersion: pv,
							SiteId:          routerId,
						})
					default:
						unretryable = fmt.Errorf("Unsupported gateway endpoint protocol: %s", si.Protocol)
						return nil
					}
				} else {
					name := fmt.Sprintf("%s:%d", gatewayName+gatewayIngress+service.Address, si.Ports[i])
					portToUse := strconv.Itoa(endpoint.Service.Ports[i])
					if !checkPortFree("tcp", portToUse) {
						freePort, err := GetFreePort()
						if err != nil {
							unretryable = fmt.Errorf("Unable to get free port for listener: %w", err)
							return nil
						} else {
							portToUse = strconv.Itoa(freePort)
						}
					}
					switch si.Protocol {
					case "tcp":
						newBridges.AddTcpListener(qdr.TcpEndpoint{
							Name:    name,
							Host:    ifc,
							Port:    portToUse,
							Address: fmt.Sprintf("%s:%d", service.Address, si.Ports[i]),
							SiteId:  routerId,
						})
					case "http", "http2":
						pv := qdr.HttpVersion1
						if si.Protocol == "http2" {
							pv = qdr.HttpVersion2
						}
						newBridges.AddHttpListener(qdr.HttpEndpoint{
							Name:            name,
							Host:            ifc,
							Port:            portToUse,
							Address:         fmt.Sprintf("%s:%d", endpoint.Service.Address, si.Ports[i]),
							ProtocolVersion: pv,
							Aggregation:     si.Aggregate,
							EventChannel:    si.EventChannel,
							SiteId:          routerId,
						})
					default:
						unretryable = fmt.Errorf("Unsupported gateway endpoint protocol: %s", si.Protocol)
						return nil
					}
				}
			}
		} else {
			for i, _ := range si.Ports {
				if gatewayDirection == gatewayEgress {
					name := fmt.Sprintf("%s:%d", gatewayName+gatewayEgress+endpoint.Service.Address, si.Ports[i])
					switch si.Protocol {
					case "tcp":
						newBridges.RemoveTcpConnector(name)
					case "http", "http2":
						newBridges.RemoveHttpConnector(name)
					default:
						unretryable = fmt.Errorf("Unsupported gateway endpoint protocol: %s", si.Protocol)
						return nil
					}
				} else {
					name := fmt.Sprintf("%s:%d", gatewayName+gatewayIngress+endpoint.Service.Address, si.Ports[i])
					switch si.Protocol {
					case "tcp":
						newBridges.RemoveTcpListener(name)
					case "http", "http2":
						newBridges.RemoveHttpListener(name)
					default:
						unretryable = fmt.Errorf("Unsupported gateway endpoint protocol: %s", si.Protocol)
						return nil
					}
				}
			}
		}

		bcDiff := currentGatewayConfig.Bridges.Difference(&newBridges)

		if bcDiff != (&qdr.BridgeConfigDifference{}) && gatewayType != GatewayMockType {
			currentGatewayConfig.Bridges = newBridges
			currentGatewayConfig.UpdateConfigMap(configmap)

			_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
			if err != nil {
				return err
			}

			err = applyBridgeDifferences(bcDiff, gatewayDir)
			if err != nil {
				unretryable = err
				return nil
			}

			_, err = os.Stat(gatewayDir + "/config/skrouterd.json")
			if err == nil {
				err = updateLocalGatewayConfig(gatewayDir, gatewayType, *currentGatewayConfig)
				if err != nil {
					unretryable = err
					return nil
				}
			}
		}
		return nil
	})
	if unretryable != nil {
		return unretryable
	}
	return err
}

func (cli *VanClient) GatewayBind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return cli.gatewayBridgeEndpointUpdate(ctx, gatewayName, gatewayEgress, true, endpoint)
}

func (cli *VanClient) GatewayUnbind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return cli.gatewayBridgeEndpointUpdate(ctx, gatewayName, gatewayEgress, false, endpoint)
}

func (cli *VanClient) GatewayForward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return cli.gatewayBridgeEndpointUpdate(ctx, gatewayName, gatewayIngress, true, endpoint)
}

func (cli *VanClient) GatewayUnforward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	return cli.gatewayBridgeEndpointUpdate(ctx, gatewayName, gatewayIngress, false, endpoint)
}

func (cli *VanClient) GatewayExpose(ctx context.Context, gatewayName string, gatewayType string, endpoint types.GatewayEndpoint) (string, error) {
	if gatewayType == "" {
		gatewayType = "service"
	}
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	// check if existing and type change
	currentType, err := cli.getGatewayType(gatewayName)
	if err == nil && currentType != gatewayType {
		return "", fmt.Errorf("gateway previously created as %s type, delete current gateway to change to %s type", currentType, gatewayType)
	}

	// Create the cluster service if it does not exist
	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve service definition: %w", err)
	}
	if si == nil {
		err = cli.ServiceInterfaceCreate(context.Background(), &types.ServiceInterface{
			Address:        endpoint.Service.Address,
			Protocol:       endpoint.Service.Protocol,
			Ports:          endpoint.Service.Ports,
			EnableTls:      endpoint.Service.EnableTls,
			TlsCredentials: endpoint.Service.TlsCredentials,
		})
		if err != nil {
			return "", fmt.Errorf("Unable to create service: %w", err)
		}

		_, err = kube.WaitServiceExists(endpoint.Service.Address, cli.GetNamespace(), cli.KubeClient, time.Second*60, time.Second*5)
		if err != nil {
			return "", err
		}

		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			svc, err := kube.GetService(endpoint.Service.Address, cli.GetNamespace(), cli.KubeClient)
			if err == nil {
				if svc.ObjectMeta.Labels == nil {
					svc.ObjectMeta.Labels = map[string]string{}
				}
				svc.ObjectMeta.Labels[types.GatewayQualifier] = gatewayName
				_, err = cli.KubeClient.CoreV1().Services(cli.GetNamespace()).Update(svc)
				return err
			} else {
				return err
			}
		})
		if err != nil {
			return "", err
		}

	}

	_, err = kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := cli.GatewayInit(ctx, gatewayName, gatewayType, "")
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	err = waitForGatewayActive(gatewayName, gatewayType, time.Second*180, time.Second*2)
	if err != nil {
		return gatewayName, err
	}

	// endpoint.Service.Ports was initially defined with service ports
	// now we need to update it to use the target ports before calling GatewayBind
	endpoint.Service.Ports = endpoint.TargetPorts
	err = cli.GatewayBind(ctx, gatewayName, endpoint)
	if err != nil {
		return gatewayName, err
	}

	return gatewayName, nil
}

func (cli *VanClient) GatewayUnexpose(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint, deleteLast bool) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Unable to retrieve gateay definition: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	if deleteLast && len(gatewayConfig.Bridges.TcpConnectors) == 1 && len(gatewayConfig.Bridges.TcpListeners) == 0 {
		err = cli.GatewayRemove(ctx, gatewayName)
		if err != nil {
			return err
		}
	} else {
		// Note: unexpose implicitly removes the cluster service
		si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
		if err != nil {
			return fmt.Errorf("Failed to retrieve service: %w", err)
		}

		err = cli.GatewayUnbind(ctx, gatewayName, endpoint)
		if err != nil {
			return err
		}

		if si != nil && len(si.Targets) == 0 && si.Origin == "" {
			err := cli.ServiceInterfaceRemove(ctx, endpoint.Service.Address)
			if err != nil {
				return fmt.Errorf("Failed to remove service: %w", err)
			}
		}
	}

	return nil
}

func (cli *VanClient) GatewayList(ctx context.Context) ([]*types.GatewayInspectResponse, error) {
	var list []*types.GatewayInspectResponse
	gateways, err := cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).List(metav1.ListOptions{LabelSelector: "skupper.io/type=gateway-definition"})
	if err != nil {
		return nil, err
	}

	for _, gateway := range gateways.Items {
		backoff := retry.DefaultRetry
		for i := 0; i < 20; i++ {
			if i > 0 {
				time.Sleep(backoff.Step())
			}
			gatewayName, ok := gateway.ObjectMeta.Annotations["skupper.io/gateway-name"]
			if ok {
				inspect, inspectErr := cli.GatewayInspect(ctx, gatewayName)
				if inspectErr == nil {
					list = append(list, inspect)
					break
				}
			}
		}
	}

	return list, nil
}

func (cli *VanClient) GatewayInspect(ctx context.Context, gatewayName string) (*types.GatewayInspectResponse, error) {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return nil, fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(clusterGatewayName(gatewayName), cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}

	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	gatewayType, err := cli.getGatewayType(gatewayName)
	if err != nil {
		return nil, err
	}

	gatewayVersion, _ := getRouterVersion(gatewayName, gatewayType)

	url := "amqp://127.0.0.1:5672"
	bc := &qdr.BridgeConfig{}

	inspect := types.GatewayInspectResponse{
		Name:       gatewayName,
		Type:       gatewayType,
		Url:        url,
		Version:    gatewayVersion,
		Connectors: map[string]types.GatewayEndpoint{},
		Listeners:  map[string]types.GatewayEndpoint{},
	}

	if gatewayType != GatewayMockType {
		inspect.Url, err = getRouterUrl(gatewayDir)
		if err != nil {
			return &inspect, err
		}
		if isActive(gatewayName, gatewayType) {
			agent, err := qdr.Connect(inspect.Url, nil)
			if err != nil {
				return &inspect, err
			}
			defer agent.Close()
			bc, err = agent.GetLocalBridgeConfig()
			if err != nil {
				return &inspect, err
			}
		} else {
			return &inspect, nil
		}
	}

	for name, connector := range gatewayConfig.Bridges.TcpConnectors {
		port, _ := strconv.Atoi(connector.Port)
		inspect.Connectors[name] = types.GatewayEndpoint{
			Name: connector.Name,
			Host: connector.Host,
			Service: types.ServiceInterface{
				Ports:    []int{port},
				Address:  connector.Address,
				Protocol: "tcp",
			},
		}
	}

	for name, connector := range gatewayConfig.Bridges.HttpConnectors {
		port, _ := strconv.Atoi(connector.Port)
		protocol := "http"
		if connector.ProtocolVersion == qdr.HttpVersion2 {
			protocol = "http2"
		}
		inspect.Connectors[name] = types.GatewayEndpoint{
			Name: connector.Name,
			Host: connector.Host,
			Service: types.ServiceInterface{
				Ports:        []int{port},
				Address:      connector.Address,
				Protocol:     protocol,
				Aggregate:    connector.Aggregation,
				EventChannel: connector.EventChannel,
			},
		}
	}

	for name, listener := range gatewayConfig.Bridges.TcpListeners {
		port, _ := strconv.Atoi(listener.Port)
		inspect.Listeners[name] = types.GatewayEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			LocalPort: bc.TcpListeners[listener.Name].Port,
			Service: types.ServiceInterface{
				Ports:    []int{port},
				Address:  listener.Address,
				Protocol: "tcp",
			},
		}
	}

	for name, listener := range gatewayConfig.Bridges.HttpListeners {
		port, _ := strconv.Atoi(listener.Port)
		protocol := "http"
		if listener.ProtocolVersion == qdr.HttpVersion2 {
			protocol = "http2"
		}
		inspect.Listeners[name] = types.GatewayEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			LocalPort: bc.HttpListeners[listener.Name].Port,
			Service: types.ServiceInterface{
				Ports:        []int{port},
				Address:      listener.Address,
				Protocol:     protocol,
				Aggregate:    listener.Aggregation,
				EventChannel: listener.EventChannel,
			},
		}
	}
	return &inspect, nil
}

func (cli *VanClient) GatewayExportConfig(ctx context.Context, targetGatewayName string, exportGatewayName string, exportPath string) (string, error) {
	if targetGatewayName == "" {
		targetGatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayResourceName := clusterGatewayName(targetGatewayName)

	exportFile := exportPath + "/" + exportGatewayName + ".yaml"

	configmap, err := kube.GetConfigMap(gatewayResourceName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return exportFile, fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)
	if err != nil {
		return exportFile, fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	gatewayConfig := GatewayConfig{
		GatewayName:  exportGatewayName,
		QdrListeners: []qdr.Listener{},
		Bindings:     []types.GatewayEndpoint{},
		Forwards:     []types.GatewayEndpoint{},
	}

	for _, listener := range routerConfig.Listeners {
		gatewayConfig.QdrListeners = append(gatewayConfig.QdrListeners, listener)
	}

	for _, connector := range routerConfig.Bridges.TcpConnectors {
		port, _ := strconv.Atoi(connector.Port)
		gatewayConfig.Bindings = append(gatewayConfig.Bindings, types.GatewayEndpoint{
			Name: strings.TrimPrefix(connector.Name, targetGatewayName+"-"),
			Host: connector.Host,
			Service: types.ServiceInterface{
				Address:  connector.Address,
				Protocol: "tcp",
				Ports:    []int{port},
			},
			TargetPorts: []int{port},
		})
	}
	for _, listener := range routerConfig.Bridges.TcpListeners {
		port, _ := strconv.Atoi(listener.Port)
		gatewayConfig.Forwards = append(gatewayConfig.Forwards, types.GatewayEndpoint{
			Name: strings.TrimPrefix(listener.Name, targetGatewayName+"-"),
			Host: listener.Host,
			Service: types.ServiceInterface{
				Address:  listener.Address,
				Protocol: "tcp",
				Ports:    []int{port},
			},
			TargetPorts: []int{port},
		})
	}
	for _, connector := range routerConfig.Bridges.HttpConnectors {
		port, _ := strconv.Atoi(connector.Port)
		protocol := "http"
		if connector.ProtocolVersion == qdr.HttpVersion2 {
			protocol = "http2"
		}
		gatewayConfig.Bindings = append(gatewayConfig.Bindings, types.GatewayEndpoint{
			Name: strings.TrimPrefix(connector.Name, targetGatewayName+"-"),
			Host: connector.Host,
			Service: types.ServiceInterface{
				Address:      connector.Address,
				Protocol:     protocol,
				Ports:        []int{port},
				Aggregate:    connector.Aggregation,
				EventChannel: connector.EventChannel,
			},
			TargetPorts: []int{port},
		})
	}
	for _, listener := range routerConfig.Bridges.HttpListeners {
		port, _ := strconv.Atoi(listener.Port)
		protocol := "http"
		if listener.ProtocolVersion == qdr.HttpVersion2 {
			protocol = "http2"
		}
		gatewayConfig.Forwards = append(gatewayConfig.Forwards, types.GatewayEndpoint{
			Name: strings.TrimPrefix(listener.Name, targetGatewayName+"-"),
			Host: listener.Host,
			Service: types.ServiceInterface{
				Address:      listener.Address,
				Protocol:     protocol,
				Ports:        []int{port},
				Aggregate:    listener.Aggregation,
				EventChannel: listener.EventChannel,
			},
			TargetPorts: []int{port},
		})
	}

	mcData, err := yaml.Marshal(&gatewayConfig)
	if err != nil {
		return exportFile, fmt.Errorf("Failed to marshal export config: %w", err)
	}

	err = ioutil.WriteFile(exportFile, mcData, 0644)
	if err != nil {
		return exportFile, fmt.Errorf("Failed to write export config file: %w", err)
	}

	return exportFile, nil
}

func (cli *VanClient) GatewayGenerateBundle(ctx context.Context, configFile string, bundlePath string) (string, error) {
	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	owner, err := getRootObject(cli)
	if err != nil {
		return "", fmt.Errorf("Skupper is not enabled in namespace '%s'", cli.Namespace)
	}

	yamlFile, err := ioutil.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("Failed to read gateway config file: %w", err)
	}

	gatewayConfig := GatewayConfig{}
	err = yaml.Unmarshal(yamlFile, &gatewayConfig)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal gateway config file: %w", err)
	}

	gatewayName := gatewayConfig.GatewayName
	tarFile, err := os.Create(bundlePath + "/" + gatewayName + ".tar.gz")
	if err != nil {
		return "", fmt.Errorf("Unable to create download file: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	gatewayResourceName := clusterGatewayName(gatewayName)
	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayResourceName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		secret, _, err = cli.ConnectorTokenCreate(context.Background(), gatewayResourceName, "")
		if err != nil {
			return tarFile.Name(), fmt.Errorf("Failed to create gateway token: %w", err)
		}
		secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
		secret.Labels[types.SkupperTypeQualifier] = types.TypeGatewayToken
		_, err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Create(secret)
		if err != nil {
			return tarFile.Name(), fmt.Errorf("Failed to create gateway secret: %w", err)
		}
	} else if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = writeTar("skupper-router-certs/conn1-profile/"+cert, secret.Data[cert], time.Now(), tw)
		if err != nil {
			return tarFile.Name(), err
		}
	}

	routerConfig := qdr.InitialConfig(gatewayPrefix+"{{.Hostname}}", "{{.RouterID}}", Version, true, 3)

	if len(gatewayConfig.QdrListeners) == 0 {
		routerConfig.AddListener(qdr.Listener{
			Name: "amqp",
			Host: "localhost",
			Port: types.AmqpDefaultPort,
		})
	} else {
		for _, listener := range gatewayConfig.QdrListeners {
			routerConfig.AddListener(qdr.Listener{
				Name: listener.Name,
				Host: listener.Host,
				Port: listener.Port,
			})
		}
	}

	routerConfig.AddSslProfileWithPath("{{.WorkingDir}}/skupper-router-certs", qdr.SslProfile{
		Name: "conn1-profile",
	})
	connector := qdr.Connector{
		Name:             "conn1",
		Cost:             1,
		SslProfile:       "conn1-profile",
		MaxFrameSize:     16384,
		MaxSessionFrames: 640,
	}
	connector.Host = secret.ObjectMeta.Annotations["edge-host"]
	connector.Port = secret.ObjectMeta.Annotations["edge-port"]
	connector.Role = qdr.RoleEdge

	routerConfig.AddConnector(connector)

	for _, binding := range gatewayConfig.Bindings {
		switch binding.Service.Protocol {
		case "tcp":
			routerConfig.AddTcpConnector(qdr.TcpEndpoint{
				Name:    binding.Name,
				Host:    binding.Host,
				Port:    strconv.Itoa(binding.Service.Ports[0]),
				Address: binding.Service.Address,
				SiteId:  "{{.RouterID}}",
			})
		case "http":
			routerConfig.AddHttpConnector(qdr.HttpEndpoint{
				Name:            binding.Name,
				Host:            binding.Host,
				Port:            strconv.Itoa(binding.Service.Ports[0]),
				Address:         binding.Service.Address,
				ProtocolVersion: qdr.HttpVersion1,
				Aggregation:     binding.Service.Aggregate,
				EventChannel:    binding.Service.EventChannel,
				SiteId:          "{{.RouterID}}",
			})
		case "http2":
			routerConfig.AddHttpConnector(qdr.HttpEndpoint{
				Name:            binding.Name,
				Host:            binding.Host,
				Port:            strconv.Itoa(binding.Service.Ports[0]),
				Address:         binding.Service.Address,
				ProtocolVersion: qdr.HttpVersion2,
				Aggregation:     binding.Service.Aggregate,
				EventChannel:    binding.Service.EventChannel,
				SiteId:          "{{.RouterID}}",
			})
		default:
		}
	}

	for _, forward := range gatewayConfig.Forwards {
		switch forward.Service.Protocol {
		case "tcp":
			routerConfig.AddTcpListener(qdr.TcpEndpoint{
				Name:    forward.Name,
				Host:    forward.Host,
				Port:    strconv.Itoa(forward.Service.Ports[0]),
				Address: forward.Service.Address,
				SiteId:  "{{.RouterID}}",
			})
		case "http":
			routerConfig.AddHttpListener(qdr.HttpEndpoint{
				Name:            forward.Name,
				Host:            forward.Host,
				Port:            strconv.Itoa(forward.Service.Ports[0]),
				Address:         forward.Service.Address,
				ProtocolVersion: qdr.HttpVersion1,
				Aggregation:     forward.Service.Aggregate,
				EventChannel:    forward.Service.EventChannel,
				SiteId:          "{{.RouterID}}",
			})
		case "http2":
			routerConfig.AddHttpListener(qdr.HttpEndpoint{
				Name:            forward.Name,
				Host:            forward.Host,
				Port:            strconv.Itoa(forward.Service.Ports[0]),
				Address:         forward.Service.Address,
				ProtocolVersion: qdr.HttpVersion2,
				Aggregation:     forward.Service.Aggregate,
				EventChannel:    forward.Service.EventChannel,
				SiteId:          "{{.RouterID}}",
			})
		default:
		}
	}

	mc, _ := qdr.MarshalRouterConfig(routerConfig)

	instance := GatewayInstance{
		WorkingDir: "${QDR_CONF_DIR}",
		RouterID:   "${ROUTER_ID}",
		Hostname:   "${HOSTNAME}",
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	err = writeTar("config/skrouterd.json", buf.Bytes(), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	gatewayInfo := UnitInfo{
		IsSystemService: false,
		Binary:          "${QDR_BIN_PATH}",
		Image:           GetRouterImageName(),
		ConfigPath:      "${QDR_CONF_DIR}",
		GatewayName:     gatewayName,
	}

	qdrUserUnit := serviceForQdr(gatewayInfo)
	err = writeTar("service/"+gatewayName+".service", []byte(qdrUserUnit), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	launch := launchScript(gatewayInfo)
	err = writeTar("launch.sh", []byte(launch), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	remove := removeScript(gatewayInfo)
	err = writeTar("remove.sh", []byte(remove), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	expand := expandVars()
	err = writeTar("expandvars.py", []byte(expand), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	return tarFile.Name(), nil
}
