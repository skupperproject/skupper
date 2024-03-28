#!/bin/sh

remove=false
while getopts xt: flag
do
    case "${flag}" in
        t) type=${OPTARG};;
        x) remove=true;;
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

TAR_CONTENT_START=$(awk '/^__TARBALL_CONTENT__$/ {print NR+1; exit 0;}' $0)
TMP_DIR=$(mktemp -d /tmp/skupper-bundle.XXXXX)
CUR_DIR=$(pwd)

cleanup() {
  [[ -d "${TMP_DIR}" ]] && rm -rf "${TMP_DIR}"
  cd ${CUR_DIR}
}

trap cleanup EXIT
tail -n+${TAR_CONTENT_START} $0 | tar zxvf - -C ${TMP_DIR}
cd ${TMP_DIR}

# if remove requested
if $remove; then
  echo
  echo "Removing '${type:-systemd}' gateway"
  sh ./remove.sh
  exit 0
fi

cp -R ./skupper-router-certs/* $certs_dir
cp ./config/skrouterd.json $qdrcfg_dir

chmod -R 0755 $local_dir

python3 ./expandvars.py $qdrcfg_dir/skrouterd.json

if [ "$type" == "service" ]; then
    mkdir -p $config_dir/systemd/user
    cp ./service/$gateway_name.service $config_dir/systemd/user/

    python3 ./expandvars.py $config_dir/systemd/user/$gateway_name.service

    systemctl --user enable $gateway_name.service
    systemctl --user daemon-reload
    systemctl --user start $gateway_name.service
elif [ "$type" == "docker" ] || [ "$type" == "podman" ]; then
    ${type} run --restart always -d --name ${gateway_name} --network host \
	   -e QDROUTERD_CONF_TYPE=json \
	   -e QDROUTERD_CONF=/opt/skupper/config/skrouterd.json \
	   -e SKUPPER_SITE_ID=gateway_${gateway_name}_$(uuidgen) \
	   -v ${local_dir}:${QDR_CONF_DIR}:Z \
	   ${gateway_image}
fi

exit 0

__TARBALL_CONTENT__
