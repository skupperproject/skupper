#!/bin/sh

set -Ceu

if [ $# -eq 0 ] || [ -z "${1}" ]; then
    echo "Use: ${0##*/} <site-name>"
    exit 1
fi

if [ -z "${UID:-}" ]; then
    UID="$(id -u)"
    export UID
fi
site=$1
sites_path="${HOME}/.local/share/skupper/sites"
service_path="${HOME}/.config/systemd/user"
systemctl="systemctl --user"
if [ "${UID}" -eq 0 ]; then
    sites_path="/usr/local/share/skupper/sites"
    service_path="/etc/systemd/system"
    systemctl="systemctl"
fi

remove_definition() {
    platform_file="${sites_path}/${site}/runtime/state/platform.yaml"
    SKUPPER_PLATFORM=$(grep '^platform: ' "${platform_file}" | sed -e 's/.*: //g')
    if [ "${SKUPPER_PLATFORM}" != "systemd" ]; then
        ${SKUPPER_PLATFORM} rm -f "${site}-skupper-router"
    fi
    rm -rf "${sites_path:?}/${site:?}/"
}

remove_service() {
    service="skupper-site-${site}.service"
    ${systemctl} stop "${service}"
    ${systemctl} disable "${service}"
    rm -f "${service_path:?}/${service:?}"
    ${systemctl} daemon-reload
    ${systemctl} reset-failed
}

main () {
    if ! echo "${site:?}" | grep -E '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'; then
        echo "Invalid site name"
        exit 0
    fi
    if [ ! -d "${sites_path:?}/${site:?}" ]; then
        echo "Site does not exist"
        exit 0
    fi
    remove_definition
    remove_service
}

main "$@"