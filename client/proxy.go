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
	"regexp"
	"strconv"
	"text/template"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/google/uuid"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/kube"
	"github.com/skupperproject/skupper/pkg/qdr"
)

const (
	proxyPrefix string = "skupper-proxy-"
)

type UnitInfo struct {
	IsSystemService bool
	Binary          string
	ConfigPath      string
	ProxyName       string
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
	// TODO: include license header?
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
    echo "qdrouterd could not be found. Please 'dnf install qpid-dispatch-router'"
    exit
fi

proxy_name={{.ProxyName}}

share_dir=${XDG_DATA_HOME:-~/.local/share}
config_dir=${XDG_CONFIG_HOME:-~/.config}

certs_dir=$share_dir/skupper/$proxy_name/qpid-dispatch-certs
qdrcfg_dir=$share_dir/skupper/$proxy_name/config

export ROUTER_ID=$(uuidgen)
export QDR_CONF_DIR=$share_dir/skupper/$proxy_name
export QDR_BIN_PATH=${QDROUTERD_HOME:-$qdr_bin}

mkdir -p $config_dir/systemd/user
mkdir -p $qdrcfg_dir
mkdir -p $certs_dir

cp -R ./qpid-dispatch-certs/* $certs_dir
cp ./service/$proxy_name.service $config_dir/systemd/user/
cp ./config/qdrouterd.json $qdrcfg_dir

python3 ./expandvars.py $config_dir/systemd/user/$proxy_name.service
python3 ./expandvars.py $qdrcfg_dir/qdrouterd.json

systemctl --user enable $proxy_name.service
systemctl --user daemon-reload
systemctl --user start $proxy_name.service

`
	var buf bytes.Buffer
	launchScript := template.Must(template.New("launchScript").Parse(launch))
	launchScript.Execute(&buf, info)

	return buf.String()
}

func removeScript(info UnitInfo) string {
	remove := `
#!/bin/sh

proxy_name={{.ProxyName}}

share_dir=${XDG_DATA_HOME:-~/.local/share}
config_dir=${XDG_CONFIG_HOME:-~/.config}

systemctl --user stop $proxy_name.service
systemctl --user disable $proxy_name.service
systemctl --user daemon-reload

rm -rf $share_dir/skupper/$proxy_name
rm $config_dir/systemd/user/$proxy_name.service
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

func isActive(proxyName string) bool {
	cmd := exec.Command("systemctl", "--user", "check", proxyName)
	err := cmd.Run()
	if err == nil {
		return true
	} else {
		return false
	}
}

func generateProxyName(namespace string, cli kubernetes.Interface) (string, error) {
	proxies, err := cli.CoreV1().ConfigMaps(namespace).List(metav1.ListOptions{LabelSelector: "skupper.io/type=proxy-definition"})
	max := 1
	if err == nil {
		proxy_name_pattern := regexp.MustCompile("proxy([0-9])+")
		for _, s := range proxies.Items {
			count := proxy_name_pattern.FindStringSubmatch(s.ObjectMeta.Name)
			if len(count) > 1 {
				v, _ := strconv.Atoi(count[1])
				if v >= max {
					max = v + 1
				}
			}

		}
	} else {
		return "", fmt.Errorf("Could not retrieve proxy config maps: %w", err)
	}
	return "proxy" + strconv.Itoa(max), nil
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

func startProxyUserService(proxyName, unitDir, localDir string) error {

	unitFile, err := ioutil.ReadFile(localDir + "/user/" + proxyName + ".service")
	if err != nil {
		return fmt.Errorf("Unable to read service file: %w", err)
	}

	err = ioutil.WriteFile(unitDir+"/"+proxyName+".service", unitFile, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write user unit file: %w", err)
	}

	cmd := exec.Command("systemctl", "--user", "enable", proxyName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "start", proxyName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}
	return nil
}

func stopProxyUserService(unitDir, proxyName string) error {

	//TODP: if service is proxyName, will have to pass param
	cmd := exec.Command("systemctl", "--user", "stop", proxyName+".service")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "disable", proxyName+".service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}

	err = os.Remove(unitDir + "/" + proxyName + ".service")
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

type ProxyInstance struct {
	DataDir  string
	Hostname string
	RouterID string
}

func (cli *VanClient) setupProxyDataDirs(ctx context.Context, proxyName string) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	err := setupLocalDir(proxyDir)
	if err != nil {
		return err
	}

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(proxyPrefix+proxyName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to retreive external proxy secret: %w", err)
	}

	for _, cert := range certs {
		err = ioutil.WriteFile(proxyDir+"/qpid-dispatch-certs/conn1-profile/"+cert, secret.Data[cert], 0644)
		if err != nil {
			return fmt.Errorf("Failed to write cert file: %w", err)
		}
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	// for qdr listener, check for 5672 in use, if it is get a free port
	var amqpPort int
	listener, ok := proxyConfig.Listeners["amqp"]
	if !ok {
		return fmt.Errorf("Unable to get amqp listener from proxy definition")
	}
	amqpPort = int(listener.Port)
	if !checkPortFree("tcp", strconv.Itoa(amqpPort)) {
		amqpPort, err = GetFreePort()
		if err != nil {
			return fmt.Errorf("Could not acquire free port: %w", err)
		}
		proxyConfig.Listeners["amqp"] = qdr.Listener{
			Name: "amqp",
			Host: "localhost",
			Port: int32(amqpPort),
		}
	}
	// store the url for instance queries
	url := fmt.Sprintf("amqp://127.0.0.1:%s", strconv.Itoa(amqpPort))
	err = ioutil.WriteFile(proxyDir+"/config/url.txt", []byte(url), 0644)

	// Iterate through the config and check free ports, get port if in use
	for name, tcpListener := range proxyConfig.Bridges.TcpListeners {
		if !checkPortFree("tcp", tcpListener.Port) {
			portToUse, err := GetFreePort()
			if err != nil {
				return fmt.Errorf("Unable to get free port for listener: %w", err)
			}
			proxyConfig.Bridges.TcpListeners[name] = qdr.TcpEndpoint{
				Name:    tcpListener.Name,
				Host:    tcpListener.Host,
				Port:    strconv.Itoa(portToUse),
				Address: tcpListener.Address,
			}
		}
	}

	mc, _ := qdr.MarshalRouterConfig(*proxyConfig)

	hostname, _ := os.Hostname()

	instance := ProxyInstance{
		DataDir:  proxyDir,
		RouterID: newUUID(),
		Hostname: hostname,
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return fmt.Errorf("Failed to parse proxy configmap: %w", err)
	}

	err = ioutil.WriteFile(proxyDir+"/config/qdrouterd.json", buf.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}

	return nil
}

func (cli *VanClient) ProxyInit(ctx context.Context, options types.ProxyInitOptions) (string, error) {
	var err error
	proxyName := options.Name

	if proxyName == "" {
		proxyName, err = generateProxyName(cli.GetNamespace(), cli.KubeClient)
		if err != nil {
			return "", fmt.Errorf("Unable to generate proxy name: %w", err)
		}
	}

	owner, err := getRootObject(cli)
	if err != nil {
		return "", fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	secret, _, err := cli.ConnectorTokenCreate(context.Background(), proxyPrefix+proxyName, "")
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*owner}
	_, err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Create(secret)

	proxyConfig := qdr.InitialConfig("proxy-"+proxyName+"-{{.Hostname}}", "{{.RouterID}}", Version, true, 3)

	// NOTE: at instantiation time detect amqp port in use and allocate port if needed
	proxyConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})

	proxyConfig.AddSslProfileWithPath("{{.DataDir}}/qpid-dispatch-certs", qdr.SslProfile{
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

	proxyConfig.AddConnector(connector)

	mapData, err := proxyConfig.AsConfigMapData()
	labels := map[string]string{
		"skupper.io/type": "proxy-definition",
	}
	_, err = kube.NewConfigMap(proxyPrefix+proxyName, &mapData, &labels, owner, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return "", fmt.Errorf("Failed to create proxy config map: %w", err)
	}

	if options.StartProxy {
		err = cli.proxyStart(ctx, proxyName)
		if err != nil {
			return proxyName, err
		}
	}
	return proxyName, nil
}

func (cli *VanClient) ProxyDownload(ctx context.Context, proxyName string, downloadPath string) error {
	certs := []string{"tls.crt", "tls.key", "ca.crt"}

	tarFile, err := os.Create(downloadPath + "/" + proxyName + ".tar.gz")
	if err != nil {
		return fmt.Errorf("Unable to create download file: %w", err)
	}

	// compress tar
	gz := gzip.NewWriter(tarFile)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	secret, err := cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Get(proxyPrefix+proxyName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("Failed to retreive external proxy secret: %w", err)
	}

	for _, cert := range certs {
		err = writeTar("qpid-dispatch-certs/conn1-profile/"+cert, secret.Data[cert], time.Now(), tw)
		if err != nil {
			return err
		}
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	mc, _ := qdr.MarshalRouterConfig(*proxyConfig)

	instance := ProxyInstance{
		DataDir:  "${QDR_CONF_DIR}",
		RouterID: "${ROUTER_ID}",
		Hostname: "${HOSTNAME}",
	}
	var buf bytes.Buffer
	qdrConfig := template.Must(template.New("qdrConfig").Parse(mc))
	qdrConfig.Execute(&buf, instance)

	if err != nil {
		return fmt.Errorf("Failed to parse proxy configmap: %w", err)
	}

	err = writeTar("config/qdrouterd.json", buf.Bytes(), time.Now(), tw)
	if err != nil {
		return err
	}

	proxyInfo := UnitInfo{
		IsSystemService: false,
		Binary:          "${QDR_BIN_PATH}",
		ConfigPath:      "${QDR_CONF_DIR}",
		ProxyName:       proxyName,
	}

	qdrUserUnit := serviceForQdr(proxyInfo)
	err = writeTar("service/"+proxyName+".service", []byte(qdrUserUnit), time.Now(), tw)
	if err != nil {
		return err
	}

	launch := launchScript(proxyInfo)
	err = writeTar("launch.sh", []byte(launch), time.Now(), tw)
	if err != nil {
		return err
	}

	remove := removeScript(proxyInfo)
	err = writeTar("remove.sh", []byte(remove), time.Now(), tw)
	if err != nil {
		return err
	}

	expand := expandVars()
	err = writeTar("expandvars.py", []byte(expand), time.Now(), tw)
	return nil
}

func (cli *VanClient) proxyStart(ctx context.Context, proxyName string) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName
	svcDir := getConfigHome() + "/systemd/user"

	qdrBinaryPath, err := exec.LookPath("qdrouterd")
	if err != nil {
		return fmt.Errorf("qdrouterd not available, please 'dnf install qpid-dispatch-router' first")
	}

	err = cli.setupProxyDataDirs(context.Background(), proxyName)
	if err != nil {
		return fmt.Errorf("Failed to create user service: %w", err)
	}

	qdrUserUnit := serviceForQdr(UnitInfo{
		IsSystemService: false,
		Binary:          qdrBinaryPath,
		ConfigPath:      proxyDir,
		ProxyName:       proxyName,
	})
	err = ioutil.WriteFile(proxyDir+"/user/"+proxyName+".service", []byte(qdrUserUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write unit file: %w", err)
	}

	err = startProxyUserService(proxyName, svcDir, proxyDir)
	if err != nil {
		return fmt.Errorf("Failed to create start service: %w", err)
	}

	return nil
}

func (cli *VanClient) proxyStop(ctx context.Context, proxyName string) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName
	svcDir := getConfigHome() + "/systemd/user"

	// TODO: this should return accumulated errors but get throught the whole thing

	if proxyName == "" {
		return fmt.Errorf("Unable to delete proxy definition, need proxy name")
	}

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	if isActive(proxyName) {
		stopProxyUserService(svcDir, proxyName)
	}

	err = os.RemoveAll(proxyDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Unable to remove proxy local directory: %w", err)
	}

	return nil
}

func (cli *VanClient) ProxyRemove(ctx context.Context, proxyName string) error {
	// TODO: this should return accumulated errors but get throught the whole thing

	if proxyName == "" {
		return fmt.Errorf("Unable to delete proxy definition, need proxy name")
	}

	err := cli.proxyStop(ctx, proxyName)
	if err != nil {
		return fmt.Errorf("Not able to stop proxy %w", err)
	}

	err = cli.KubeClient.CoreV1().Secrets(cli.GetNamespace()).Delete(proxyPrefix+proxyName, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Unbable to remove proxy secret: %w", err)
	}

	err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Delete(proxyPrefix+proxyName, &metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Unbable to remove proxy config map: %w", err)
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

func (cli *VanClient) ProxyBind(ctx context.Context, proxyName string, egress types.ProxyBindOptions) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	si, err := cli.ServiceInterfaceInspect(ctx, egress.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}

	if si == nil {
		return fmt.Errorf("Unable to proxy bind, service not found for %s", egress.Address)
	}

	// TODO switch on egress.Protocol
	endpoint := qdr.TcpEndpoint{
		Name:    proxyName + "-egress-" + egress.Address,
		Host:    egress.Host,
		Port:    egress.Port,
		Address: egress.Address,
	}
	proxyConfig.AddTcpConnector(endpoint)

	// should this be update or write to config map
	proxyConfig.UpdateConfigMap(configmap)
	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update proxy configmap: %w", err)
	}

	if isActive(proxyName) {
		mc, err := qdr.MarshalRouterConfig(*proxyConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(proxyDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}

		url, err := ioutil.ReadFile(proxyDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("Agent error adding tcp connector: %w", err)
		}
		record := map[string]interface{}{}
		if err = convert(endpoint, &record); err != nil {
			return fmt.Errorf("Failed to convert record: %w", err)
		}
		if err = agent.Create("org.apache.qpid.dispatch.tcpConnector", endpoint.Name, record); err != nil {
			return fmt.Errorf("Error adding tcp connector : %w", err)
		}
		agent.Close()
	}
	return nil
}

func (cli *VanClient) ProxyUnbind(ctx context.Context, proxyName string, address string) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	deleted, _ := proxyConfig.RemoveTcpConnector(proxyName + "-egress-" + address)

	// should this be update or write to config map
	proxyConfig.UpdateConfigMap(configmap)
	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update proxy configmap: %w", err)
	}

	if isActive(proxyName) && deleted {
		mc, err := qdr.MarshalRouterConfig(*proxyConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(proxyDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}

		url, err := ioutil.ReadFile(proxyDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		if err = agent.Delete("org.apache.qpid.dispatch.tcpConnector", proxyName+"-egress-"+address); err != nil {
			return fmt.Errorf("Error removing tcp connector : %w", err)
		}
		agent.Close()
	}

	return nil
}

func (cli *VanClient) ProxyExpose(ctx context.Context, options types.ProxyExposeOptions) (string, error) {

	proxyName, err := cli.ProxyInit(ctx, types.ProxyInitOptions{
		Name:       options.ProxyName,
		StartProxy: false,
	})
	if err != nil {
		return "", err
	}

	// Note: expose implicitly creates the service
	si, err := cli.ServiceInterfaceInspect(ctx, options.Egress.Address)
	if err != nil {
		return "", fmt.Errorf("Failed to retrieve service: %w", err)
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
			return "", fmt.Errorf("Unabled to create service: %w", err)
		}
	}

	err = cli.ProxyBind(ctx, proxyName, options.Egress)
	if err != nil {
		return proxyName, err
	}

	err = cli.proxyStart(ctx, proxyName)
	if err != nil {
		return proxyName, err
	}

	return proxyName, nil
}

func (cli *VanClient) ProxyUnexpose(ctx context.Context, proxyName string, address string) error {
	// Note: no need to unbind as proxy is being deleted
	// Note: unexpose implicitly removes the cluster service
	si, err := cli.ServiceInterfaceInspect(ctx, address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	}

	// todo: only remove service if not used, is this necessary
	if si != nil && len(si.Targets) == 0 && si.Origin == "" {
		err := cli.ServiceInterfaceRemove(ctx, address)
		if err != nil {
			return fmt.Errorf("Failed to removes service: %w", err)
		}
	}

	// Note: unexpose will stop and remove proxy independent of bridge configuration
	err = cli.proxyStop(ctx, proxyName)
	if err != nil {
		return err
	}

	err = cli.ProxyRemove(ctx, proxyName)
	if err != nil {
		return err
	}

	return nil
}

func (cli *VanClient) ProxyInspect(ctx context.Context, proxyName string) (*types.ProxyInspectResponse, error) {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	_, err := getRootObject(cli)
	if err != nil {
		return nil, fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	proxyVersion, err := exec.Command("qdrouterd", "-v").Output()

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	var url []byte
	var bc *qdr.BridgeConfig
	isActive := isActive(proxyName)
	if isActive {
		url, err = ioutil.ReadFile(proxyDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return &types.ProxyInspectResponse{}, err
		}
		bc, err = agent.GetLocalBridgeConfig()
		if err != nil {
			return &types.ProxyInspectResponse{}, err
		}
		agent.Close()
	} else {
		url = []byte("not active")
	}

	// for consideration is to have endpoints are api types or leave qdr alone
	inspect := types.ProxyInspectResponse{
		ProxyName:     proxyName,
		ProxyUrl:      string(url),
		ProxyVersion:  string(proxyVersion),
		TcpConnectors: map[string]types.ProxyEndpoint{},
		TcpListeners:  map[string]types.ProxyEndpoint{},
	}

	// this is definition, not instance
	for name, connector := range proxyConfig.Bridges.TcpConnectors {
		inspect.TcpConnectors[name] = types.ProxyEndpoint{
			Name:    connector.Name,
			Host:    connector.Host,
			Port:    connector.Port,
			Address: connector.Address,
		}
	}

	for name, listener := range proxyConfig.Bridges.TcpListeners {
		localPort := ""
		if isActive {
			localPort = bc.TcpListeners[listener.Name].Port
		}
		inspect.TcpListeners[name] = types.ProxyEndpoint{
			Name:      listener.Name,
			Host:      listener.Host,
			Port:      listener.Port,
			Address:   listener.Address,
			LocalPort: localPort,
		}
	}

	return &inspect, nil
}

func (cli *VanClient) ProxyForward(ctx context.Context, proxyName string, loopback bool, service *types.ServiceInterface) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	si, err := cli.ServiceInterfaceInspect(ctx, service.Address)
	if err != nil {
		return fmt.Errorf("Failed to retrieve service: %w", err)
	} else if si == nil {
		return fmt.Errorf("Unable to proxy forward, service not found for %s", service.Address)
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	ifc := "0.0.0.0"
	if loopback {
		ifc = "127.0.0.1"
	}
	endpoint := qdr.TcpEndpoint{
		Name:    proxyName + "-ingress-" + service.Address,
		Host:    ifc,
		Port:    strconv.Itoa(service.Port),
		Address: service.Address,
	}
	proxyConfig.AddTcpListener(endpoint)

	proxyConfig.WriteToConfigMap(configmap)

	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update external proxy config map: %s", err)
	}

	if isActive(proxyName) {
		var freePort int

		// TODO: port collision on subsequent startup?
		mc, err := qdr.MarshalRouterConfig(*proxyConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(proxyDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}

		url, err := ioutil.ReadFile(proxyDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}

		//check if service port is free otherwise get a free port
		if checkPortFree("tcp", strconv.Itoa(service.Port)) {
			endpoint.Port = strconv.Itoa(service.Port)
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
		agent.Close()
	}
	return nil
}

func (cli *VanClient) ProxyUnforward(ctx context.Context, proxyName string, address string) error {
	proxyDir := getDataHome() + "/skupper/" + proxyName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	configmap, err := kube.GetConfigMap(proxyPrefix+proxyName, cli.GetNamespace(), cli.KubeClient)
	if err != nil {
		return fmt.Errorf("Failed to retrieve proxy configmap: %w", err)
	}
	proxyConfig, err := qdr.GetRouterConfigFromConfigMap(configmap)

	deleted, _ := proxyConfig.RemoveTcpListener(proxyName + "-ingress-" + address)

	proxyConfig.WriteToConfigMap(configmap)

	_, err = cli.KubeClient.CoreV1().ConfigMaps(cli.GetNamespace()).Update(configmap)
	if err != nil {
		return fmt.Errorf("Failed to update external proxy config map: %s", err)
	}

	if isActive(proxyName) && deleted {
		mc, err := qdr.MarshalRouterConfig(*proxyConfig)
		if err != nil {
			return fmt.Errorf("Failed to marshall router config: %w", err)
		}

		err = ioutil.WriteFile(proxyDir+"/config/qdrouterd.json", []byte(mc), 0644)
		if err != nil {
			return fmt.Errorf("Failed to write config file: %w", err)
		}

		url, err := ioutil.ReadFile(proxyDir + "/config/url.txt")
		agent, err := qdr.Connect(string(url), nil)
		if err != nil {
			return fmt.Errorf("qdr agent error: %w", err)
		}
		if err = agent.Delete("org.apache.qpid.dispatch.tcpListener", proxyName+"-ingress-"+address); err != nil {
			return fmt.Errorf("Error removing tcp listener : %w", err)
		}
		agent.Close()
	}

	return nil
}
