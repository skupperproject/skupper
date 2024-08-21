#!/bin/sh

set -Ceu

namespace=${1:-default}
if [ -z "${namespace}" ]; then
    echo "Use: ${0##*/} <namespace>"
    exit 1
fi

if [ -z "${UID:-}" ]; then
    UID="$(id -u)"
    export UID
fi
namespaces_path="${HOME}/.local/share/skupper/namespaces"
service_path="${HOME}/.config/systemd/user"
systemctl="systemctl --user"
if [ "${UID}" -eq 0 ]; then
    namespaces_path="/usr/local/share/skupper/sites"
    service_path="/etc/systemd/system"
    systemctl="systemctl"
fi

usage() {
    echo "Use: remove.sh <namespace>"
}

remove_definition() {
    platform_file="${namespaces_path}/${namespace}/runtime/state/platform.yaml"
    SKUPPER_PLATFORM=$(grep '^platform: ' "${platform_file}" | sed -e 's/.*: //g')
    if [ "${SKUPPER_PLATFORM}" != "systemd" ]; then
        ${SKUPPER_PLATFORM} rm -f "${namespace}-skupper-router"
    fi
    rm -rf "${namespaces_path:?}/${namespace:?}/"
}

remove_service() {
    service="skupper-${namespace}.service"
    ${systemctl} stop "${service}"
    ${systemctl} disable "${service}"
    rm -f "${service_path:?}/${service:?}"
    ${systemctl} daemon-reload
    ${systemctl} reset-failed
}

main () {
    if ! echo "${namespace:?}" | grep -qE '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'; then
        echo "Invalid namespace"
        usage
        exit 0
    fi
    if [ ! -d "${namespaces_path:?}/${namespace:?}" ]; then
        echo "Namespace \"${namespace}\" does not exist"
        exit 0
    fi
    remove_definition
    remove_service
}

main "$@"