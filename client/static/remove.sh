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

