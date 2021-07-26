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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"

	"github.com/google/uuid"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

const (
	gatewayPrefix string = "skupper-gateway-"
)

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

certs_dir=$share_dir/skupper/$gateway_name/qpid-dispatch-certs
qdrcfg_dir=$share_dir/skupper/$gateway_name/config

export ROUTER_ID=$(cat /proc/sys/kernel/random/uuid)
export QDR_CONF_DIR=$share_dir/skupper/$gateway_name
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

rm -rf $share_dir/skupper/$gateway_name
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

func (cli *VanClient) setupGatewayDataDirs(ctx context.Context, gatewayName string) error {
	gatewayDir := getDataHome() + "/skupper/" + gatewayName

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
		RouterID: newUUID(),
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

func (cli *VanClient) GatewayInit(ctx context.Context, options types.GatewayInitOptions) (string, error) {
	var err error
	gatewayName := options.Name

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

	gatewayConfig := qdr.InitialConfig("gateway-"+gatewayName+"-{{.Hostname}}", "{{.RouterID}}", Version, true, 3)

	// NOTE: at instantiation time detect amqp port in use and allocate port if needed
	gatewayConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})

	gatewayConfig.AddSslProfileWithPath("{{.DataDir}}/qpid-dispatch-certs", qdr.SslProfile{
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

	gatewayConfig.AddConnector(connector)

	mapData, err := gatewayConfig.AsConfigMapData()
	labels := map[string]string{
		"skupper.io/type": "gateway-definition",
	}
	_, err = kube.NewConfigMap(gatewayPrefix+gatewayName, &mapData, &labels, owner, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return "", fmt.Errorf("Failed to create gateway config map: %w", err)
	}

	if !options.DownloadOnly {
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
	gatewayDir := getDataHome() + "/skupper/" + gatewayName
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
	gatewayDir := getDataHome() + "/skupper/" + gatewayName
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

func (cli *VanClient) GatewayBind(ctx context.Context, options types.GatewayBindOptions) error {

	gatewayName := options.GatewayName

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + "/skupper/" + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	si, err := cli.ServiceInterfaceInspect(ctx, options.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}
	if si == nil {
		return fmt.Errorf("Unable to gateway bind, service not found for %s", options.Address)
	}

	endpointName := gatewayName + "-egress-" + options.Address
	endpoint := qdr.TcpEndpoint{
		Name:    endpointName,
		Host:    options.Host,
		Port:    options.Port,
		Address: options.Address,
	}

	current, ok := gatewayConfig.Bridges.TcpConnectors[endpointName]
	if ok {
		if reflect.DeepEqual(current, endpoint) {
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
			if err = agent.Delete("org.apache.qpid.dispatch.tcpConnector", endpointName); err != nil {
				return fmt.Errorf("Error removing tcp connector : %w", err)
			}
		}
	}
	gatewayConfig.AddTcpConnector(endpoint)

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
		mc, err := qdr.MarshalRouterConfig(*gatewayConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}
	}

	if isActive(gatewayName) {
		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		if err != nil {
			return fmt.Errorf("Failed to read instance url file: %w", err)
		}

		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()
		record := map[string]interface{}{}
		if err = convert(endpoint, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create("org.apache.qpid.dispatch.tcpConnector", endpointName, record); err != nil {
			return fmt.Errorf("Error adding tcp connector : %w", err)
		}
	}
	return nil
}

func (cli *VanClient) GatewayUnbind(ctx context.Context, options types.GatewayUnbindOptions) error {

	gatewayName := options.GatewayName

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + "/skupper/" + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	if _, ok := gatewayConfig.Bridges.TcpConnectors[gatewayName+"-egress-"+options.Address]; !ok {
		return nil
	}

	deleted, _ := gatewayConfig.RemoveTcpConnector(gatewayName + "-egress-" + options.Address)

	gatewayConfig.UpdateConfigMap(configmap)
	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update gateway configmap: %w", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		mc, err := qdr.MarshalRouterConfig(*gatewayConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
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
		if err = agent.Delete("org.apache.qpid.dispatch.tcpConnector", gatewayName+"-egress-"+options.Address); err != nil {
			return fmt.Errorf("Error removing tcp connector : %w", err)
		}
	}

	return nil
}

func (cli *VanClient) GatewayExpose(ctx context.Context, options types.GatewayExposeOptions) (string, error) {

	gatewayName := options.GatewayName

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	_, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := cli.GatewayInit(ctx, types.GatewayInitOptions{
				Name:         options.GatewayName,
				DownloadOnly: true,
			})
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	// Create the cluster service if it does not exist
	si, err := cli.ServiceInterfaceInspect(ctx, options.Egress.Address)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve service definition: %w", err)
	}
	if si == nil {
		servicePort, err := strconv.Atoi(options.Egress.Port)
		if err != nil {
			return "", fmt.Errorf("%s is not a valid port", options.Egress.Port)
		}
		err = cli.ServiceInterfaceCreate(context.Background(), &types.ServiceInterface{
			Address:  options.Egress.Address,
			Protocol: options.Egress.Protocol,
			Port:     servicePort,
		})
		if err != nil {
			return "", fmt.Errorf("Unable to create service: %w", err)
		}

		svc, err := kube.WaitServiceExists(options.Egress.Address, cli.GetNamespace(), cli.KubeClient, time.Second*60, time.Second*5)
		if svc.ObjectMeta.Labels == nil {
			svc.ObjectMeta.Labels = map[string]string{}
		}
		svc.ObjectMeta.Labels[types.GatewayQualifier] = gatewayName
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			_, err = cli.KubeClient.CoreV1().Services(cli.GetNamespace()).Update(svc)
			return err
		})

	}

	err = cli.GatewayBind(ctx, types.GatewayBindOptions{
		GatewayName: gatewayName,
		Protocol:    options.Egress.Protocol,
		Address:     options.Egress.Address,
		Host:        options.Egress.Host,
		Port:        options.Egress.Port,
		ErrIfNoSvc:  options.Egress.ErrIfNoSvc,
	})
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

func (cli *VanClient) GatewayUnexpose(ctx context.Context, options types.GatewayUnexposeOptions) error {
	gatewayName := options.GatewayName

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Unable to retrieve gateay definition: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	// Note: unexpose implicitly removes the cluster service
	si, err := cli.ServiceInterfaceInspect(ctx, options.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}

	if si != nil && len(si.Targets) == 0 && si.Origin == "" {
		err := cli.ServiceInterfaceRemove(ctx, options.Address)
		if err != nil {
			return fmt.Errorf("Failed to removes service: %w", err)
		}
	}

	if options.DeleteIfLast && len(gatewayConfig.Bridges.TcpConnectors) == 1 && len(gatewayConfig.Bridges.TcpListeners) == 0 {
		err = cli.gatewayStop(ctx, gatewayName)
		if err != nil {
			return err
		}
		err = cli.GatewayRemove(ctx, gatewayName)
		if err != nil {
			return err
		}
	} else {
		return cli.GatewayUnbind(ctx, types.GatewayUnbindOptions{
			GatewayName: options.GatewayName,
			Protocol:    "tcp",
			Address:     options.Address,
		})
	}

	return nil
}

func (cli *VanClient) GatewayForward(ctx context.Context, options types.GatewayForwardOptions) error {

	gatewayName := options.GatewayName

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + "/skupper/" + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	si, err := cli.ServiceInterfaceInspect(ctx, options.Service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	} else if si == nil {
		return fmt.Errorf("Unable to gateway forward, service not found for %s", options.Service.Address)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	ifc := "0.0.0.0"
	if options.Loopback {
		ifc = "127.0.0.1"
	}

	endpointName := gatewayName + "-ingress-" + options.Service.Address
	endpoint := qdr.TcpEndpoint{
		Name:    endpointName,
		Host:    ifc,
		Port:    strconv.Itoa(options.Service.Port),
		Address: options.Service.Address,
	}

	current, ok := gatewayConfig.Bridges.TcpListeners[endpointName]
	if ok {
		if reflect.DeepEqual(current, endpoint) {
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
			if err = agent.Delete("org.apache.qpid.dispatch.tcpListener", endpointName); err != nil {
				return fmt.Errorf("Error removing tcp listener : %w", err)
			}
		}
	}
	gatewayConfig.AddTcpListener(endpoint)

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
		mc, err := qdr.MarshalRouterConfig(*gatewayConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}
	}

	if isActive(gatewayName) {
		var freePort int

		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()

		//check if service port is free otherwise get a free port
		if checkPortFree("tcp", strconv.Itoa(options.Service.Port)) {
			endpoint.Port = strconv.Itoa(options.Service.Port)
		} else {
			freePort, err = GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			} else {
				endpoint.Port = strconv.Itoa(freePort)
			}
		}

		record := map[string]interface{}{}
		if err = convert(endpoint, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create("org.apache.qpid.dispatch.tcpListener", endpoint.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp listener : %w", err)
		}
	}
	return nil
}

func (cli *VanClient) GatewayUnforward(ctx context.Context, gatewayName string, address string) error {

	if gatewayName == "" {
		gatewayName, _ = getUserDefaultGatewayName()
	}

	gatewayDir := getDataHome() + "/skupper/" + gatewayName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(gatewayPrefix+gatewayName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve gateway configmap: %w", err)
	}
	gatewayConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	deleted, _ := gatewayConfig.RemoveTcpListener(gatewayName + "-ingress-" + address)

	gatewayConfig.WriteToConfigMap(configmap)

	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update external gateway config map: %s", err)
	}

	_, err = os.Stat(gatewayDir + "/config/qdrouterd.json")
	if err == nil {
		mc, err := qdr.MarshalRouterConfig(*gatewayConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(gatewayDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}
	}

	if isActive(gatewayName) && deleted {
		url, err := ioutil.ReadFile(gatewayDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		defer agent.Close()
		if err = agent.Delete("org.apache.qpid.dispatch.tcpListener", gatewayName+"-ingress-"+address); err != nil {
			return fmt.Errorf("Error removing tcp listener : %w", err)
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
		inspect, _ := cli.GatewayInspect(ctx, strings.TrimPrefix(gateway.ObjectMeta.Name, gatewayPrefix))
		list = append(list, inspect)
	}

	return list, nil
}

func (cli *VanClient) GatewayInspect(ctx context.Context, gatewayName string) (*types.GatewayInspectResponse, error) {
	gatewayDir := getDataHome() + "/skupper/" + gatewayName

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
		GatewayName:    gatewayName,
		GatewayUrl:     string(url),
		GatewayVersion: string(gatewayVersion),
		TcpConnectors:  map[string]types.GatewayEndpoint{},
		TcpListeners:   map[string]types.GatewayEndpoint{},
	}

	for name, connector := range gatewayConfig.Bridges.TcpConnectors {
		inspect.TcpConnectors[name] = types.GatewayEndpoint{
			Name:    connector.Name,
			Host:    connector.Host,
			Port:    connector.Port,
			Address: connector.Address,
		}
	}

	for name, listener := range gatewayConfig.Bridges.TcpListeners {
		localPort := ""
		if isActive {
			localPort = bc.TcpListeners[listener.Name].Port
		}
		inspect.TcpListeners[name] = types.GatewayEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			Port:      listener.Port,
			Address:   listener.Address,
			LocalPort: localPort,
		}
	}

	return &inspect, nil
}
