package client

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"text/template"

	"github.com/skupperproject/skupper/api/types"
	"github.com/skupperproject/skupper/pkg/certs"
	"github.com/skupperproject/skupper/pkg/qdr"
)

type UnitInfo struct {
	Binary     string
	ConfigPath string
	ProxyName  string
}

func userServiceForQdr(info UnitInfo) string {
	service := `
[Unit]
Description=Qpid Dispatch router daemon
	
[Service]
Type=simple
ExecStart={{.Binary}} -c {{.ConfigPath}}/{{.ProxyName}}/config/qdrouterd.json

[Install]
WantedBy=multi-user.target
`
	var buf bytes.Buffer
	qdrService := template.Must(template.New("qdrService").Parse(service))
	qdrService.Execute(&buf, info)

	return buf.String()
}

func systemServiceForQdr(info UnitInfo) string {
	service := `
[Unit]
Description=Qpid Dispatch router daemon
Requires=network.target
After=network.target
	
[Service]
Type=simple
ExecStart={{.Binary}} -c {{.ConfigPath}}/{{.ProxyName}}/config/qdrouterd.json

[Install]
WantedBy=multi-user.target
`
	var buf bytes.Buffer
	qdrService := template.Must(template.New("qdrService").Parse(service))
	qdrService.Execute(&buf, info)

	return buf.String()
}

func specForRpm(options types.ExternalServiceCreateOptions) string {
	spec := `
###############################################################################
# Spec file for Skupper External Service
################################################################################
Summary: Skupper External Service script for RPM creation
Name: {{.PackageName}}
Version: 1
Release: 0
License: GPL
URL: http://www.skupper.io
Group: System
Packager: Salty Pug
Source0: %{name}-%{version}.tar.gz
#Requires: qdrouterd
BuildRoot: ~/rpmbuild
	
# Build with the following syntax:
# rpmbuild --target noarch -bb packagname.spec
	
%description
A package for external skupper service proxy creation

%prep
%setup -q -n {{.PackageName}}

%install
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/%{_datadir}
mkdir -p $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/config
mkdir -p $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/qpid-dispatch-certs
mkdir -p $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/qpid-dispatch-certs/conn1-profile
mkdir -p $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/system

cp qpid-dispatch-certs/conn1-profile/* $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/qpid-dispatch-certs/conn1-profile
cp config/qdrouterd.json $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/config
cp system/qdrouterd.service $RPM_BUILD_ROOT/%{_datadir}/{{.PackageName}}/system

%files
%{_datadir}/{{.PackageName}}/qpid-dispatch-certs/conn1-profile/*
%{_datadir}/{{.PackageName}}/config/qdrouterd.json
%{_datadir}/{{.PackageName}}/system/qdrouterd.service

%pre

%post
mkdir -p /etc/qpid-dispatch-certs/conn1-profile

# What files/dirs should be preserved prior to setting up links?
ln -s /usr/share/{{.PackageName}}/qpid-dispatch-certs/conn1-profile /etc/qpid-dispatch-certs/conn1-profile
ln -s /usr/share/{{.PackageName}}/config/qdrouterd.json /etc/qpid-dispatch/qdrouterd.json
ln -s /usr/share/{{.PackageName}}/system/qdrouterd.service /etc/systemd/system/qdrouterd.service

systemctl enable qdrouterd.service
systemctl daemon-reload
systemctl start qdrouterd.service

%preun
if [ $1 == 0 ]; then #uninstall
  systemctl stop qdrouterd.service
  systemctl disable qdrouterd.service
fi

%postun
# restore saved files?
rm -rf /etc/qpid-dispatch-certs
rm /etc/qpid-dispatch/qdrouterd.json

if [ $1 == 0 ]; then #uninstall
  systemctl daemon-reload
  systemctl reset-failed
fi
`
	var buf bytes.Buffer
	rpmSpec := template.Must(template.New("rpmSpec").Parse(spec))
	rpmSpec.Execute(&buf, options)

	return buf.String()
}

func setupRpmBuild(rpmDir string) error {
	_ = os.RemoveAll(rpmDir)
	if err := os.MkdirAll(rpmDir, 0744); err != nil {
		return err
	}
	if err := os.MkdirAll(rpmDir+"/RPMS", 0744); err != nil {
		return err
	}
	if err := os.MkdirAll(rpmDir+"/RPMS/noarch", 0744); err != nil {
		return err
	}
	if err := os.MkdirAll(rpmDir+"/SOURCES", 0744); err != nil {
		return err
	}
	if err := os.MkdirAll(rpmDir+"/SPECS", 0744); err != nil {
		return err
	}
	if err := os.MkdirAll(rpmDir+"/SRPMS", 0744); err != nil {
		return err
	}

	return nil
}

func setupDataDir(dataDir string) error {
	_ = os.RemoveAll(dataDir)
	if err := os.MkdirAll(dataDir+"/config", 0744); err != nil {
		return fmt.Errorf("Unable to create config directory: %w", err)
	}
	if err := os.MkdirAll(dataDir+"/user", 0744); err != nil {
		return fmt.Errorf("Unable to create user directory: %w", err)
	}
	if err := os.MkdirAll(dataDir+"/system", 0744); err != nil {
		return fmt.Errorf("Unable to create system directory: %w", err)
	}
	if err := os.MkdirAll(dataDir+"/qpid-dispatch-certs/conn1-profile", 0744); err != nil {
		return fmt.Errorf("Unable to create certs directory: %w", err)
	}
	return nil
}

func createUserService(unitDir, dataDir string) error {

	unitFile, err := ioutil.ReadFile(dataDir + "/user/qdrouterd.service")
	if err != nil {
		return fmt.Errorf("Unable to read service file: %w", err)
	}

	err = ioutil.WriteFile("/home/ansmith/.config/systemd/user/qdrouterd.service", unitFile, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write user unit file: %w", err)
	}

	cmd := exec.Command("systemctl", "--user", "enable", "qdrouterd.service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to enable user service: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to user service daemon-reload: %w", err)
	}

	cmd = exec.Command("systemctl", "--user", "start", "qdrouterd.service")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to start user service: %w", err)
	}
	return nil
}

func buildRpm(options types.ExternalServiceCreateOptions, srcDir string, buildDir string, destDir string) error {
	pkgName := options.PackageName
	current, _ := os.Getwd()

	err := setupRpmBuild(buildDir + "/rpmbuild")
	if err != nil {
		return fmt.Errorf("Unable to create rpmbuild tree: %w", err)
	}

	rpmSpec := specForRpm(options)
	if err := ioutil.WriteFile(buildDir+"/rpmbuild/SPECS/"+pkgName+".spec", []byte(rpmSpec), 0755); err != nil {
		return fmt.Errorf("Failed to write external service spec file: %w", err)
	}

	// consider go creation of tar rather than cmd
	err = os.Chdir(srcDir)
	if err != nil {
		return fmt.Errorf("Unable to change dir: %w", err)
	}

	cmd := exec.Command("tar", "zcvf", buildDir+"/rpmbuild/SOURCES/"+pkgName+"-1.tar.gz", ".")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to create rpm source: %w", err)
	}

	cmd = exec.Command("rpmbuild", "--bb", "--target", "noarch", buildDir+"/rpmbuild/SPECS/"+pkgName+".spec")
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("Unable to create rpm: %w", err)
	}

	rpm, err := ioutil.ReadFile(buildDir + "/rpmbuild/RPMS/noarch/" + pkgName + "-1-0.noarch.rpm")
	if err != nil {
		return fmt.Errorf("Unable to read rpm: %w", err)
	}

	err = ioutil.WriteFile(destDir+"/"+pkgName+"-1-0.noarch.rpm", rpm, 0644)
	if err != nil {
		return fmt.Errorf("Unable to write rpm: %w", err)
	}

	err = os.Chdir(current)
	if err != nil {
		return fmt.Errorf("Unable to change dir: %w", err)
	}

	return nil
}

func (cli *VanClient) ExternalServiceCreate(ctx context.Context, options types.ExternalServiceCreateOptions) error {
	pkgName := options.PackageName
	homeDir, _ := os.UserHomeDir()
	dataDir := homeDir + "/.local/share/skupper/" + pkgName

	_, err := getRootObject(cli)
	if err != nil {
		return fmt.Errorf("Skupper not initialized in %s", cli.Namespace)
	}

	err = setupDataDir(dataDir)
	if err != nil {
		return err
	}

	siteConfig, err := cli.SiteConfigInspect(context.Background(), nil)
	if err != nil {
		return fmt.Errorf("Failed to retrieve site config: %w", err)
	}

	// generate a connection token for the external service
	err = cli.ConnectorTokenCreateFile(context.Background(), types.DefaultVanName, dataDir+"/"+pkgName+".yaml")
	if err != nil {
		return fmt.Errorf("Failed to create connection token: %w", err)
	}

	hostString := ""
	portString := ""
	secret, err := certs.GetSecretContent(dataDir + "/" + pkgName + ".yaml")
	if err != nil {
		return fmt.Errorf("Failed to get secret content: %w", err)
	} else {
		for k, v := range secret {
			if k == "tls.crt" || k == "tls.key" || k == "ca.crt" {
				err = ioutil.WriteFile(dataDir+"/qpid-dispatch-certs/conn1-profile/"+k, v, 0777)
				if err != nil {
					return fmt.Errorf("Failed to write cert file: %w", err)
				}
			} else if k == "edge-host" {
				hostString = string(v)
			} else if k == "edge-port" {
				portString = string(v)
			}
		}
	}
	// done with secret file
	os.Remove(dataDir + "/" + pkgName + ".yaml")

	proxyConfig := qdr.InitialConfig(options.Address+"-service-${HOSTNAME}", siteConfig.Reference.UID, Version, true, 3)
	proxyConfig.AddListener(qdr.Listener{
		Name: "amqp",
		Host: "localhost",
		Port: types.AmqpDefaultPort,
	})
	proxyConfig.AddSslProfileWithPath(dataDir+"/qpid-dispatch-certs", qdr.SslProfile{
		Name: "conn1-profile",
	})
	connector := qdr.Connector{
		Name:             "conn1",
		Cost:             1,
		SslProfile:       "conn1-profile",
		MaxFrameSize:     16384,
		MaxSessionFrames: 640,
	}
	connector.Host = string(hostString)
	connector.Port = string(portString)
	connector.Role = qdr.RoleEdge

	proxyConfig.AddConnector(connector)

	endpoint := qdr.TcpEndpoint{
		Name:    options.Address,
		Host:    options.EgressHost,
		Port:    options.EgressPort,
		Address: options.Address,
	}
	proxyConfig.AddTcpConnector(endpoint)

	mc, _ := qdr.MarshalRouterConfig(proxyConfig)
	err = ioutil.WriteFile(dataDir+"/config/qdrouterd.json", []byte(mc), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write config file: %w", err)
	}

	qdrUserUnit := userServiceForQdr(UnitInfo{
		Binary:     "/usr/sbin/qdrouterd",
		ConfigPath: homeDir + "/.local/share/skupper",
		ProxyName:  options.PackageName,
	})
	err = ioutil.WriteFile(dataDir+"/user/qdrouterd.service", []byte(qdrUserUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write unit file: %w", err)
	}

	qdrSystemUnit := systemServiceForQdr(UnitInfo{
		Binary:     "/usr/sbin/qdrouterd",
		ConfigPath: "/usr/share",
		ProxyName:  options.PackageName,
	})
	err = ioutil.WriteFile(dataDir+"/system/qdrouterd.service", []byte(qdrSystemUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write unit file: %w", err)
	}

	if options.CreateUnit {
		err = createUserService("~/.config/systemd/user", dataDir)
		if err != nil {
			return fmt.Errorf("Failed to create user service: %w", err)
		}
	}

	if options.BuildPackage {
		err = buildRpm(options, homeDir+"/.local/share/skupper/", homeDir, "/home/ansmith/tmp")
		if err != nil {
			return fmt.Errorf("Failed to generate service rpm: %w", err)
		}
	}

	//	_ = os.RemoveAll(homeDir+"/rpmbuild")
	return nil
}
