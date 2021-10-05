package client

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/user"
	"reflect"
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
)

const (
	gatewayPrefix     string = "skupper-gateway-"
	gatewayIngress    string = "-ingress-"
	gatewayEgress     string = "-egress-"
	gatewayClusterDir string = "/skupper/cluster/"
	gatewayBundleDir  string = "/skupper/bundle/"
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
	ConfigPath      string
	GatewayName     string
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
ExecStart={{.Binary}} -c {{.ConfigPath}}/config/qdrouterd.json

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
	print ("Usage: python3 expandvars.py <absolute_file_path>. Example - python3 expandvars.py /tmp/qdrouterd.conf")
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

if result=$(command -v qdrouterd 2>&1); then
    qdr_bin=$result
else
    echo "qdrouterd could not be found. Please 'install qdrouterd'"
    exit
fi

if [[ -z "$(command -v python3 2>&1)" ]]; then
    echo "python3 could not be found. Please 'install python3'"
    exit
fi

gateway_name={{.GatewayName}}

share_dir=${XDG_DATA_HOME:-~/.local/share}
config_dir=${XDG_CONFIG_HOME:-~/.config}

certs_dir=$share_dir/skupper/bundle/$gateway_name/qpid-dispatch-certs
qdrcfg_dir=$share_dir/skupper/bundle/$gateway_name/config

export ROUTER_ID=$(cat /proc/sys/kernel/random/uuid)
export QDR_CONF_DIR=$share_dir/skupper/bundle/$gateway_name
export QDR_BIN_PATH=${QDROUTERD_HOME:-$qdr_bin}

mkdir -p $config_dir/systemd/user
mkdir -p $qdrcfg_dir
mkdir -p $certs_dir

cp -R ./qpid-dispatch-certs/* $certs_dir
cp ./service/$gateway_name.service $config_dir/systemd/user/
cp ./config/qdrouterd.json $qdrcfg_dir

python3 ./expandvars.py $config_dir/systemd/user/$gateway_name.service
python3 ./expandvars.py $qdrcfg_dir/qdrouterd.json

systemctl --user enable $gateway_name.service
systemctl --user daemon-reload
systemctl --user start $gateway_name.service

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

systemctl --user stop $gateway_name.service
systemctl --user disable $gateway_name.service
systemctl --user daemon-reload

rm -rf $share_dir/skupper/bundle/$gateway_name
rm $config_dir/systemd/user/$gateway_name.service
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

func newUUID() string {
	return uuid.New().String()
}

func isActive(gatewayName string) bool {
	cmd := exec.Command("systemctl", "--user", "check", gatewayName)
	err := cmd.Run()
	if err == nil {
		return true
	} else {
		return false
	}
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

	if err := os.MkdirAll(localDir+"/config", 0744); err != nil {
		return fmt.Errorf("Unable to create config directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/user", 0744); err != nil {
		return fmt.Errorf("Unable to create user directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/system", 0744); err != nil {
		return fmt.Errorf("Unable to create system directory: %w", err)
	}

	if err := os.MkdirAll(localDir+"/qpid-dispatch-certs/conn1-profile", 0744); err != nil {
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

	cmd := exec.Command("systemctl", "--user", "stop", gatewayName+".service")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "disable", gatewayName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}

	err = os.Remove(unitDir + "/" + gatewayName + ".service")
	if err != nil {
		return fmt.Errorf("Unable to remove user service file: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	return nil
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

type GatewayInstance struct {
	DataDir  string
	Hostname string
	RouterID string
}

func updateLocalGatewayConfig(gatewayDir string, gatewayConfig qdr.RouterConfig) error {
	mc, err := qdr.MarshalRouterConfig(gatewayConfig)
	if err != nil {
		return fmt.Errorf("Failed to marshall router config: %w", err)
	}

	hostname, _ := os.Hostname()

	routerId, err := ioutil.ReadFile(gatewayDir + "/config/routerid.txt")
	if err != nil {
		return fmt.Errorf("Failed to read instance url file: %w", err)
	}

	instance := GatewayInstance{
		DataDir:  gatewayDir,
		RouterID: string(routerId),
		Hostname: hostname,
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}
	return nil
}

func (cli *VanClient) setupGatewayDataDirs(ctx context.Context, gatewayName string) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	err := setupLocalDir(gatewayDir)
	if err != nil {
		return err
	}

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayPrefix+gatewayName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to retreive external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = ioutil.WriteFile(gatewayDir+"/qpid-dispatch-certs/conn1-profile/"+cert, secret.Data[cert], 0644)
		if err != nil {
			return fmt.Errorf("Failed to write cert file: %w", err)
		}
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	// for qdr listener, check for 5672 in use, if it is get a free port
	var amqpPort int
	listener, ok := gatewayConfig.Listeners["amqp"]
	if !ok {
		return fmt.Errorf("Unable to get amqp listener from gateway definition")
	}
	amqpPort = int(listener.Port)
	if !checkPortFree("tcp", strconv.Itoa(amqpPort)) {
		amqpPort, err = GetFreePort()
		if err != nil {
			return fmt.Errorf("Could not acquire free port: %w", err)
		}
		gatewayConfig.Listeners["amqp"] = qdr.Listener{
			Name: "amqp",
			Host: "localhost",
			Port: int32(amqpPort),
		}
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

	// Iterate through the config and check free ports, get port if in use
	for name, tcpListener := range gatewayConfig.Bridges.TcpListeners {
		if !checkPortFree("tcp", tcpListener.Port) {
			portToUse, err := GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			}
			gatewayConfig.Bridges.TcpListeners[name] = qdr.TcpEndpoint{
				Name:    tcpListener.Name,
				Host:    tcpListener.Host,
				Port:    strconv.Itoa(portToUse),
				Address: tcpListener.Address,
			}
		}
	}

	mc, _ := qdr.MarshalRouterConfig(*gatewayConfig)

	hostname, _ := os.Hostname()

	instance := GatewayInstance{
		DataDir:  gatewayDir,
		RouterID: routerId,
		Hostname: hostname,
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}

	return nil
}

func (cli *VanClient) GatewayInit(ctx context.Context, gatewayName string, configFile string, exportOnly bool) (string, error) {
	var err error

	if gatewayName != "" {
		nameRegex := regexp.MustCompile(`^[a-z]([a-z0-9-]*[a-z0-9])*$`)
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
		return "", fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	_, err = kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err == nil {
		return "", fmt.Errorf("Gateway name already exists: %s", gatewayName)
	}

	secret, _, err := cli.ConnectorTokenCreate(context.Background(), gatewayPrefix+gatewayName, "")
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
	_, err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Create(secret)

	routerConfig := qdr.InitialConfig("gateway-"+gatewayName+"-{{.Hostname}}", "{{.RouterID}}", Version, true, 3)

	// NOTE: at instantiation time detect amqp port in use and allocate port if needed
	routerConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})

	routerConfig.AddSslProfileWithPath("{{.DataDir}}/qpid-dispatch-certs", qdr.SslProfile{
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
				name := gatewayName + gatewayEgress + binding.Service.Address + ":" + strconv.Itoa(binding.Service.Ports[i])
				addr := fmt.Sprintf("%s:%d", binding.Service.Address, binding.Service.Ports[i])
				switch binding.Service.Protocol {
				case "tcp":
					routerConfig.AddTcpConnector(qdr.TcpEndpoint{
						Name:    name,
						Host:    binding.Host,
						Port:    strconv.Itoa(binding.TargetPorts[i]),
						Address: addr,
					})
				case "http":
					routerConfig.AddHttpConnector(qdr.HttpEndpoint{
						Name:            name,
						Host:            binding.Host,
						Port:            strconv.Itoa(binding.TargetPorts[i]),
						Address:         addr,
						ProtocolVersion: qdr.HttpVersion1,
						Aggregation:     binding.Service.Aggregate,
						EventChannel:    binding.Service.EventChannel,
					})
				case "http2":
					routerConfig.AddHttpConnector(qdr.HttpEndpoint{
						Name:            name,
						Host:            binding.Host,
						Port:            strconv.Itoa(binding.TargetPorts[i]),
						Address:         addr,
						ProtocolVersion: qdr.HttpVersion2,
						Aggregation:     binding.Service.Aggregate,
						EventChannel:    binding.Service.EventChannel,
					})
				default:
				}
			}
		}

		for _, forward := range gatewayConfig.Forwards {
			for i, _ := range forward.TargetPorts {
				name := gatewayName + gatewayIngress + forward.Service.Address + ":" + strconv.Itoa(forward.Service.Ports[i])
				addr := fmt.Sprintf("%s:%d", forward.Service.Address, forward.Service.Ports[i])
				switch forward.Service.Protocol {
				case "tcp":
					routerConfig.AddTcpListener(qdr.TcpEndpoint{
						Name:    name,
						Host:    forward.Host,
						Port:    strconv.Itoa(forward.Service.Ports[i]),
						Address: addr,
					})
				case "http":
					routerConfig.AddHttpListener(qdr.HttpEndpoint{
						Name:            name,
						Host:            forward.Host,
						Port:            strconv.Itoa(forward.Service.Ports[i]),
						Address:         addr,
						ProtocolVersion: qdr.HttpVersion1,
						Aggregation:     forward.Service.Aggregate,
						EventChannel:    forward.Service.EventChannel,
					})
				case "http2":
					routerConfig.AddHttpListener(qdr.HttpEndpoint{
						Name:            name,
						Host:            forward.Host,
						Port:            strconv.Itoa(forward.Service.Ports[i]),
						Address:         addr,
						ProtocolVersion: qdr.HttpVersion2,
						Aggregation:     forward.Service.Aggregate,
						EventChannel:    forward.Service.EventChannel,
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
	_, err = kube.NewConfigMap(gatewayPrefix+gatewayName, &mapData, &labels, owner, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return "", fmt.Errorf("Failed to create gateway config map: %w", err)
	}

	if !exportOnly {
		err = cli.gatewayStart(ctx, gatewayName)
		if err != nil {
			return gatewayName, err
		}
	}
	return gatewayName, nil
}

func (cli *VanClient) GatewayDownload(ctx context.Context, gatewayName string, downloadPath string) (string, error) {
	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	tarFile, err := os.Create(downloadPath + "/" + gatewayName + ".tar.gz")
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Unable to create download file: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayPrefix+gatewayName, metav1.GetOptions{})
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = writeTar("qpid-dispatch-certs/conn1-profile/"+cert, secret.Data[cert], time.Now(), tw)
		if err != nil {
			return tarFile.Name(), err
		}
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	mc, _ := qdr.MarshalRouterConfig(*gatewayConfig)

	instance := GatewayInstance{
		DataDir:  "${QDR_CONF_DIR}",
		RouterID: "${ROUTER_ID}",
		Hostname: "${HOSTNAME}",
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	err = writeTar("config/qdrouterd.json", buf.Bytes(), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	gatewayInfo := UnitInfo{
		IsSystemService: false,
		Binary:          "${QDR_BIN_PATH}",
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

func (cli *VanClient) gatewayStart(ctx context.Context, gatewayName string) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	svcDir := getConfigHome() + "/systemd/user"

	qdrBinaryPath, err := exec.LookPath("qdrouterd")
	if err != nil {
		return fmt.Errorf("qdrouterd not available, please 'dnf install qpid-dispatch-router' first")
	}

	err = cli.setupGatewayDataDirs(context.Background(), gatewayName)
	if err != nil {
		return fmt.Errorf("Failed to setup gateway local directories: %w", err)
	}

	err = os.MkdirAll(svcDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create gateway service directory: %w", err)
	}

	qdrUserUnit := serviceForQdr(UnitInfo{
		IsSystemService: false,
		Binary:          qdrBinaryPath,
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

func (cli *VanClient) gatewayStop(ctx context.Context, gatewayName string) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName
	svcDir := getConfigHome() + "/systemd/user"

	if gatewayName == "" {
		return fmt.Errorf("Unable to delete gateway definition, need gateway name")
	}

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	if isActive(gatewayName) {
		stopGatewayUserService(svcDir, gatewayName)
	}

	err = os.RemoveAll(gatewayDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Unable to remove gateway local directory: %w", err)
	}

	return nil
}

func (cli *VanClient) GatewayRemove(ctx context.Context, gatewayName string) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	err := cli.gatewayStop(ctx, gatewayName)
	if err != nil {
		return fmt.Errorf("Not able to stop gateway %w", err)
	}

	svcList, err := cli.KubeClient.CoreV1().Services(cli.GetNamespace()).List(metav1.ListOptions{LabelSelector: types.GatewayQualifier + "=" + gatewayName})
	for _, service := range svcList.Items {
		si, err := cli.ServiceInterfaceInspect(ctx, service.Name)
		if err != nil {
			return fmt.Errorf("Failed to retrieve service: %w", err)
		}
		if si != nil && len(si.Targets) == 0 && si.Origin == "" {
			err := cli.ServiceInterfaceRemove(ctx, service.Name)
			if err != nil {
				return fmt.Errorf("Failed to remove service: %w", err)
			}
		} else {
			delete(service.ObjectMeta.Labels, types.GatewayQualifier)
			err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
				_, err = cli.KubeClient.CoreV1().Services(cli.GetNamespace()).Update(&service)
				return err
			})
		}
	}

	err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Delete(gatewayPrefix+gatewayName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to remove gateway secret: %w", err)
	}

	err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Delete(gatewayPrefix+gatewayName, &metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("Unable to remove gateway config map: %w", err)
	}
	return nil
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
			return "org.apache.qpid.dispatch.tcpListener"
		} else {
			return "org.apache.qpid.dispatch.tcpConnector"
		}
	} else if protocol == "http" || protocol == "http2" {
		if endpointType == gatewayIngress {
			return "org.apache.qpid.dispatch.httpListener"
		} else {
			return "org.apache.qpid.dispatch.httpConnector"
		}
	}
	return ""
}

func gatewayAddTcpEndpoint(gatewayName string, endpointType string, tcpEndpoint qdr.TcpEndpoint, gatewayConfig *qdr.RouterConfig) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	ok := false
	current := qdr.TcpEndpoint{}
	if endpointType == gatewayIngress {
		current, ok = gatewayConfig.Bridges.TcpListeners[tcpEndpoint.Name]
	} else {
		current, ok = gatewayConfig.Bridges.TcpConnectors[tcpEndpoint.Name]
	}
	if ok {
		if reflect.DeepEqual(current, tcpEndpoint) {
			return nil
		} else if isActive(gatewayName) {
			url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
			if err != nil {
				return fmt.Errorf("Failed to read instance url file: %w", err)
			}

			agent, err := qdr.Connect(string(url), nil)
			if err != nil {
				return fmt.Errorf("qdr agent error: %w", err)
			}
			defer agent.Close()
			if err = agent.Delete(getEntity("tcp", endpointType), tcpEndpoint.Name); err != nil {
				return fmt.Errorf("Error removing tcp entity : %w", err)
			}
		}
	}
	if endpointType == gatewayIngress {
		gatewayConfig.AddTcpListener(tcpEndpoint)
	} else {
		gatewayConfig.AddTcpConnector(tcpEndpoint)
	}

	if isActive(gatewayName) {
		var freePort int

		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()

		// for ingress, check if service port is free otherwise get a free port
		if endpointType == gatewayIngress && !checkPortFree("tcp", tcpEndpoint.Port) {
			freePort, err = GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			} else {
				tcpEndpoint.Port = strconv.Itoa(freePort)
			}
		}

		record := map[string]interface{}{}
		if err = convert(tcpEndpoint, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("tcp", endpointType), tcpEndpoint.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp entity : %w", err)
		}
	}

	return nil
}

func gatewayAddHttpEndpoint(gatewayName string, endpointType string, httpEndpoint qdr.HttpEndpoint, gatewayConfig *qdr.RouterConfig) error {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	ok := false
	current := qdr.HttpEndpoint{}

	if endpointType == gatewayIngress {
		current, ok = gatewayConfig.Bridges.HttpListeners[httpEndpoint.Name]
	} else {
		current, ok = gatewayConfig.Bridges.HttpConnectors[httpEndpoint.Name]
	}
	if ok {
		if reflect.DeepEqual(current, httpEndpoint) {
			return nil
		} else if isActive(gatewayName) {
			url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
			if err != nil {
				return fmt.Errorf("Failed to read instance url file: %w", err)
			}

			agent, err := qdr.Connect(string(url), nil)
			if err != nil {
				return fmt.Errorf("qdr agent error: %w", err)
			}
			defer agent.Close()
			if err = agent.Delete(getEntity("http", endpointType), httpEndpoint.Name); err != nil {
				return fmt.Errorf("Error removing http entity : %w", err)
			}
		}
	}
	if endpointType == gatewayIngress {
		gatewayConfig.AddHttpListener(httpEndpoint)
	} else {
		gatewayConfig.AddHttpConnector(httpEndpoint)
	}

	if isActive(gatewayName) {
		var freePort int

		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()

		// for ingress, check if service port is free otherwise get a free port
		if endpointType == gatewayIngress && !checkPortFree("tcp", httpEndpoint.Port) {
			freePort, err = GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			} else {
				httpEndpoint.Port = strconv.Itoa(freePort)
			}
		}

		record := map[string]interface{}{}
		if err = convert(httpEndpoint, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create(getEntity("http", endpointType), httpEndpoint.Name, record); err != nil {
			return fmt.Errorf("Error adding http endpoint : %w", err)
		}
	}

	return nil
}

func (cli *VanClient) GatewayBind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	service := endpoint.Service

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	si, err := cli.ServiceInterfaceInspect(ctx, service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}
	if si == nil {
		return fmt.Errorf("Unable to gateway bind, service not found for %s", service.Address)
	}
	if len(si.Ports) != len(service.Ports) {
		return fmt.Errorf("Unable to gateway bind, the given service provides %d ports, but only %d provided", len(si.Ports), len(service.Ports))
	}

	for i, _ := range service.Ports {
		name := fmt.Sprintf("%s:%d", gatewayName+gatewayEgress+service.Address, si.Ports[i])
		switch endpoint.Service.Protocol {
		case "tcp":
			err = gatewayAddTcpEndpoint(gatewayName,
				gatewayEgress,
				qdr.TcpEndpoint{
					Name:    name,
					Host:    endpoint.Host,
					Port:    strconv.Itoa(service.Ports[i]),
					Address: fmt.Sprintf("%s:%d", service.Address, si.Ports[i]),
				},
				gatewayConfig)
		case "http", "http2":
			pv := qdr.HttpVersion1
			if endpoint.Service.Protocol == "http2" {
				pv = qdr.HttpVersion2
			}
			err = gatewayAddHttpEndpoint(gatewayName,
				gatewayEgress,
				qdr.HttpEndpoint{
					Name:            name,
					Host:            endpoint.Host,
					Port:            strconv.Itoa(endpoint.Service.Ports[i]),
					Address:         fmt.Sprintf("%s:%d", endpoint.Service.Address, si.Ports[i]),
					ProtocolVersion: pv,
				},
				gatewayConfig)
		default:
			return fmt.Errorf("Unsupported gateway endpoint protocol: %s", endpoint.Service.Protocol)
		}
	}
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	gatewayConfig.UpdateConfigMap(configmap)
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to update gateway configmap: %w", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		err := updateLocalGatewayConfig(gatewayDir, *gatewayConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cli *VanClient) GatewayUnbind(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}

	deleted := false
	for i, _ := range si.Ports {
		name := fmt.Sprintf("%s:%d", gatewayName+gatewayEgress+endpoint.Service.Address, si.Ports[i])
		switch endpoint.Service.Protocol {
		case "tcp":
			if _, ok := gatewayConfig.Bridges.TcpConnectors[name]; !ok {
				return nil
			}
			deleted, _ = gatewayConfig.RemoveTcpConnector(name)
		case "http", "http2":
			if _, ok := gatewayConfig.Bridges.HttpConnectors[name]; !ok {
				return nil
			}
			deleted, _ = gatewayConfig.RemoveHttpConnector(name)
		default:
			return fmt.Errorf("Unsupported gateway endpoint protocol: %s", endpoint.Service.Protocol)
		}
	}

	gatewayConfig.UpdateConfigMap(configmap)
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to update gateway configmap: %w", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		err := updateLocalGatewayConfig(gatewayDir, *gatewayConfig)
		if err != nil {
			return err
		}
	}

	if isActive(gatewayName) && deleted {
		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		if err != nil {
			return fmt.Errorf("Failed to read instance url file: %w", err)
		}

		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()
		for i, _ := range si.Ports {
			name := fmt.Sprintf("%s:%d", gatewayName+gatewayEgress+endpoint.Service.Address, si.Ports[i])
			if err = agent.Delete(getEntity(endpoint.Service.Protocol, gatewayEgress), name); err != nil {
				return fmt.Errorf("Error removing entity connector : %w", err)
			}
		}
	}

	return nil
}

func (cli *VanClient) GatewayExpose(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) (string, error) {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	_, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := cli.GatewayInit(ctx, gatewayName, "", true)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	// Create the cluster service if it does not exist
	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve service definition: %w", err)
	}
	if si == nil {
		err = cli.ServiceInterfaceCreate(context.Background(), &types.ServiceInterface{
			Address:  endpoint.Service.Address,
			Protocol: endpoint.Service.Protocol,
			Ports:    endpoint.Service.Ports,
		})
		if err != nil {
			return "", fmt.Errorf("Unable to create service: %w", err)
		}

		svc, err := kube.WaitServiceExists(endpoint.Service.Address, cli.GetNamespace(), cli.KubeClient, time.Second*60, time.Second*5)
		if svc.ObjectMeta.Labels == nil {
			svc.ObjectMeta.Labels = map[string]string{}
		}
		svc.ObjectMeta.Labels[types.GatewayQualifier] = gatewayName
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, err = cli.KubeClient.CoreV1().Services(cli.GetNamespace()).Update(svc)
			return err
		})

	}

	// endpoint.Service.Ports was initially defined with service ports
	// now we need to update it to use the target ports before calling GatewayBind
	endpoint.Service.Ports = endpoint.TargetPorts
	err = cli.GatewayBind(ctx, gatewayName, endpoint)
	if err != nil {
		return gatewayName, err
	}

	// Note: if gateway was init as download only, it will get started here
	if !isActive(gatewayName) {
		err = cli.gatewayStart(ctx, gatewayName)
		if err != nil {
			return gatewayName, err
		}
	}

	return gatewayName, nil
}

func (cli *VanClient) GatewayUnexpose(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint, deleteLast bool) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Unable to retrieve gateay definition: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	if deleteLast && len(gatewayConfig.Bridges.TcpConnectors) == 1 && len(gatewayConfig.Bridges.TcpListeners) == 0 {
		err = cli.gatewayStop(ctx, gatewayName)
		if err != nil {
			return err
		}
		err = cli.GatewayRemove(ctx, gatewayName)
		if err != nil {
			return err
		}
	} else {
		err = cli.GatewayUnbind(ctx, gatewayName, endpoint)
		if err != nil {
			return err
		}
	}

	// Note: unexpose implicitly removes the cluster service
	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}
	if si != nil && len(si.Targets) == 0 && si.Origin == "" {
		err := cli.ServiceInterfaceRemove(ctx, endpoint.Service.Address)
		if err != nil {
			return fmt.Errorf("Failed to removes service: %w", err)
		}
	}

	return nil
}

func (cli *VanClient) GatewayForward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint, loopback bool) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	} else if si == nil {
		return fmt.Errorf("Unable to gateway forward, service not found for %s", endpoint.Service.Address)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	ifc := "0.0.0.0"
	if loopback {
		ifc = "127.0.0.1"
	}

	for i, _ := range endpoint.Service.Ports {
		name := fmt.Sprintf("%s:%d", gatewayName+gatewayIngress+endpoint.Service.Address, si.Ports[i])
		switch endpoint.Service.Protocol {
		case "tcp":
			err = gatewayAddTcpEndpoint(gatewayName,
				gatewayIngress,
				qdr.TcpEndpoint{
					Name:    name,
					Host:    ifc,
					Port:    strconv.Itoa(endpoint.Service.Ports[i]),
					Address: fmt.Sprintf("%s:%d", endpoint.Service.Address, si.Ports[i]),
				},
				gatewayConfig)
		case "http", "http2":
			pv := qdr.HttpVersion1
			if endpoint.Service.Protocol == "http2" {
				pv = qdr.HttpVersion2
			}
			err = gatewayAddHttpEndpoint(gatewayName,
				gatewayIngress,
				qdr.HttpEndpoint{
					Name:            name,
					Host:            ifc,
					Port:            strconv.Itoa(endpoint.Service.Ports[i]),
					Address:         fmt.Sprintf("%s:%d", endpoint.Service.Address, si.Ports[i]),
					ProtocolVersion: pv,
				},
				gatewayConfig)
		default:
			return fmt.Errorf("Unsuppored gateway endpoint protocol: %s", endpoint.Service.Protocol)
		}
	}
	if err != nil {
		return fmt.Errorf(err.Error())
	}

	gatewayConfig.UpdateConfigMap(configmap)
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to update gateway configmap: %w", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		err := updateLocalGatewayConfig(gatewayDir, *gatewayConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cli *VanClient) GatewayUnforward(ctx context.Context, gatewayName string, endpoint types.GatewayEndpoint) error {
	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	si, err := cli.ServiceInterfaceInspect(ctx, endpoint.Service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}

	deleted := false
	if si != nil {
		for i, _ := range si.Ports {
			name := fmt.Sprintf("%s:%d", gatewayName+gatewayIngress+endpoint.Service.Address, si.Ports[i])
			switch endpoint.Service.Protocol {
			case "tcp":
				if _, ok := gatewayConfig.Bridges.TcpListeners[name]; !ok {
					return nil
				}
				deleted, _ = gatewayConfig.RemoveTcpListener(name)
			case "http", "http2":
				if _, ok := gatewayConfig.Bridges.HttpListeners[name]; !ok {
					return nil
				}
				deleted, _ = gatewayConfig.RemoveHttpListener(name)
			default:
				return fmt.Errorf("Unsupported gateway endpoint protocol: %s", endpoint.Service.Protocol)
			}
		}
	}
	gatewayConfig.WriteToConfigMap(configmap)

	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update external gateway config map: %s", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		err := updateLocalGatewayConfig(gatewayDir, *gatewayConfig)
		if err != nil {
			return err
		}
	}

	if isActive(gatewayName) && deleted {
		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()

		for i, _ := range si.Ports {
			name := fmt.Sprintf("%s:%d", gatewayName+gatewayIngress+endpoint.Service.Address, si.Ports[i])
			if err = agent.Delete(getEntity(endpoint.Service.Protocol, gatewayIngress), name); err != nil {
				return fmt.Errorf("Error removing endpoint listener : %w", err)
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
			inspect, inspectErr := cli.GatewayInspect(ctx, strings.TrimPrefix(gateway.ObjectMeta.Name, gatewayPrefix))
			if inspectErr == nil {
				list = append(list, inspect)
				break
			}
		}
	}

	return list, nil
}

func (cli *VanClient) GatewayInspect(ctx context.Context, gatewayName string) (*types.GatewayInspectResponse, error) {
	gatewayDir := getDataHome() + gatewayClusterDir + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return nil, fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	gatewayVersion, err := exec.Command("qdrouterd", "-v").Output()

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	var url []byte
	var bc *qdr.BridgeConfig
	isActive := isActive(gatewayName)
	if isActive {
		url, err = ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return &types.GatewayInspectResponse{}, err
		}
		defer agent.Close()
		bc, err = agent.GetLocalBridgeConfig()
		if err != nil {
			return &types.GatewayInspectResponse{}, err
		}
	} else {
		url = []byte("not active")
	}

	inspect := types.GatewayInspectResponse{
		GatewayName:       gatewayName,
		GatewayUrl:        string(url),
		GatewayVersion:    string(gatewayVersion),
		GatewayConnectors: map[string]types.GatewayEndpoint{},
		GatewayListeners:  map[string]types.GatewayEndpoint{},
	}

	for name, connector := range gatewayConfig.Bridges.TcpConnectors {
		port, _ := strconv.Atoi(connector.Port)
		inspect.GatewayConnectors[name] = types.GatewayEndpoint{
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
		inspect.GatewayConnectors[name] = types.GatewayEndpoint{
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
		localPort := ""
		if isActive {
			localPort = bc.TcpListeners[listener.Name].Port
		}
		inspect.GatewayListeners[name] = types.GatewayEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			LocalPort: localPort,
			Service: types.ServiceInterface{
				Ports:    []int{port},
				Address:  listener.Address,
				Protocol: "tcp",
			},
		}
	}

	for name, listener := range gatewayConfig.Bridges.HttpListeners {
		port, _ := strconv.Atoi(listener.Port)
		localPort := ""
		if isActive {
			localPort = bc.HttpListeners[listener.Name].Port
		}
		protocol := "http"
		if listener.ProtocolVersion == qdr.HttpVersion2 {
			protocol = "http2"
		}
		inspect.GatewayListeners[name] = types.GatewayEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			LocalPort: localPort,
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

	exportFile := exportPath + "/" + exportGatewayName + ".yaml"

	configmap, err := kube.GetConfigMap(gatewayPrefix+targetGatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return exportFile, fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	routerConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

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
		})
	}
	mcData, err := yaml.Marshal(&gatewayConfig)

	if err != nil {
		return exportFile, fmt.Errorf("Failed to marshale export config ")
	}

	err = ioutil.WriteFile(exportFile, mcData, 0644)

	return exportFile, nil
}

func (cli *VanClient) GatewayGenerateBundle(ctx context.Context, configFile string, bundlePath string) (string, error) {
	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	owner, err := getRootObject(cli)
	if err != nil {
		return "", fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
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

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(gatewayPrefix+gatewayName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		secret, _, err = cli.ConnectorTokenCreate(context.Background(), gatewayPrefix+gatewayName, "")
		secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
		_, err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Create(secret)
	} else if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to retrieve external gateway secret: %w", err)
	}

	for _, cert := range certs {
		err = writeTar("qpid-dispatch-certs/conn1-profile/"+cert, secret.Data[cert], time.Now(), tw)
		if err != nil {
			return tarFile.Name(), err
		}
	}

	routerConfig := qdr.InitialConfig("gateway-{{.Hostname}}", "{{.RouterID}}", Version, true, 3)

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

	routerConfig.AddSslProfileWithPath("{{.DataDir}}/qpid-dispatch-certs", qdr.SslProfile{
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
			})
		default:
		}
	}

	mc, _ := qdr.MarshalRouterConfig(routerConfig)

	instance := GatewayInstance{
		DataDir:  "${QDR_CONF_DIR}",
		RouterID: "${ROUTER_ID}",
		Hostname: "${HOSTNAME}",
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return tarFile.Name(), fmt.Errorf("Failed to parse gateway configmap: %w", err)
	}

	err = writeTar("config/qdrouterd.json", buf.Bytes(), time.Now(), tw)
	if err != nil {
		return tarFile.Name(), err
	}

	gatewayInfo := UnitInfo{
		IsSystemService: false,
		Binary:          "${QDR_BIN_PATH}",
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
