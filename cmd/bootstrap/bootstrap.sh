#!/bin/sh

set -Ceu

IMAGE="quay.io/skupper/bootstrap"
INPUT_PATH="${1:-${PWD}}"
SKUPPER_OUTPUT_PATH="${XDG_DATA_HOME:-${HOME}/.local/share}/skupper"
SERVICE_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/systemd/user"
if [ -z "${UID:-}" ]; then
    UID="$(id -u)"
    export UID
fi
LOG_FILE="$(mktemp /tmp/skupper-bootstrap.XXXXX.log)"

exit_error() {
    echo "$*"
    exit 1
}

is_sock_endpoint() {
    case "${CONTAINER_ENDPOINT}" in
        "/"*)
            return 0
            ;;
        "unix://"*)
            return 0
            ;;
    esac
    return 1
}

is_container_platform() {
    if [ "${SKUPPER_PLATFORM}" = "podman" ] || [ "${SKUPPER_PLATFORM}" = "docker" ]; then
        return 0
    fi
    return 1
}

is_bundle_platform() {
    if [ "${SKUPPER_PLATFORM}" = "bundle" ] || [ "${SKUPPER_PLATFORM}" = "tarball" ]; then
        return 0
    fi
    return 1
}

container_env() {
    export SKUPPER_PLATFORM="${SKUPPER_PLATFORM:-podman}"
    export CONTAINER_ENGINE="${CONTAINER_ENGINE:-podman}"
    # Determining if the respective binaries are available
    case "${SKUPPER_PLATFORM}" in
        systemd)
            command -v skrouterd > /dev/null || exit_error "SystemD platform cannot be used: skrouterd not found"
            command -v "${CONTAINER_ENGINE}" > /dev/null || exit_error "SystemD platform cannot be used: ${CONTAINER_ENGINE} (container engine) not found"
            ;;
        docker)
            command -v docker > /dev/null || exit_error "Docker platform cannot be used: docker not found"
            export CONTAINER_ENGINE=docker
            ;;
        *)
            command -v podman > /dev/null || exit_error "Podman platform cannot be used: podman not found"
            export CONTAINER_ENGINE=podman
            ;;
    esac
    export CONTAINER_ENDPOINT_DEFAULT="unix://${XDG_RUNTIME_DIR:-/run/user/${UID}}/podman/podman.sock"
    GID=$(id -g "${UID}")
    export RUNAS="${UID}:${GID}"
    export USERNS="keep-id"
    if [ "${CONTAINER_ENGINE}" = "docker" ]; then
        export CONTAINER_ENDPOINT_DEFAULT="unix:///run/docker.sock"
        export USERNS="host"
        DOCKER_GID=$(getent group docker | cut -d: -f3)
        export RUNAS="${UID}:${DOCKER_GID}"
    fi
    if [ "${UID}" -eq 0 ]; then
        if [ "${CONTAINER_ENGINE}" = "podman" ]; then
            export CONTAINER_ENDPOINT_DEFAULT="unix:///run/podman/podman.sock"
        fi
        export USERNS=""
        export SKUPPER_OUTPUT_PATH="/usr/local/share/skupper"
        export SERVICE_DIR="/etc/systemd/system"
    fi
    mkdir -p "${SKUPPER_OUTPUT_PATH}"
    export CONTAINER_ENDPOINT="${CONTAINER_ENDPOINT:-${CONTAINER_ENDPOINT_DEFAULT}}"
}

create_service() {
    # do not create service when preparing a site bundle
    if is_bundle_platform; then
        return
    fi

    # if systemd is not available, skip it
    if [ "${UID}" -eq 0 ]; then
        systemctl list-units > /dev/null 2>&1 || return
    else
        systemctl --user list-units > /dev/null 2>&1 || return
    fi

    # generated service file
    site_name="$(grep -E 'Site ".*" has been created' "${LOG_FILE}" | awk -F'"' '{print $2}')"
    if [ -z "${site_name}" ]; then
        echo "Unable to create SystemD service (site name could not be identified)"
        return
    fi
    service_name="skupper-site-${site_name}.service"
    service_file="${SKUPPER_OUTPUT_PATH}/sites/${site_name}/runtime/scripts/${service_name}"
    if [ ! -f "${service_file}" ]; then
        echo "SystemD service has not been defined"
        return
    fi

    # Moving it to the appropriate location
    if [ "${UID}" -eq 0 ]; then
        cp -f "${service_file}" /etc/systemd/system/
        systemctl enable --now "${service_name}"
        systemctl daemon-reload
    else
        if [ ! -d "${SERVICE_DIR}" ]; then
            echo "Unable to define path to SystemD service"
            return
        fi
        cp -f "${service_file}" "${SERVICE_DIR}"
        systemctl --user enable --now "${service_name}"
        systemctl --user daemon-reload
    fi
}

main() {
    if [ -z "${INPUT_PATH}" ] || [ ! -d "${INPUT_PATH}" ]; then
        exit_error "Use: bootstrap.sh <local path to CRs>"
    fi

    # Parse Skupper Platform and Container Engine settings
    container_env

    # Must be mounted into the container
    MOUNTS=""
    ENV_VARS=""
    
    # Mounts
    if is_sock_endpoint && is_container_platform; then
        file_container_endpoint=$(echo "${CONTAINER_ENDPOINT}" | sed -e "s#unix://##g")
        MOUNTS="${MOUNTS} -v '${file_container_endpoint}:/${CONTAINER_ENGINE}.sock:z'"
    fi
    MOUNTS="${MOUNTS} -v '${INPUT_PATH}:/input:z'"
    MOUNTS="${MOUNTS} -v '${SKUPPER_OUTPUT_PATH}:/output:z'"
    
    # Env vars
    if is_container_platform; then
        if is_sock_endpoint; then
            ENV_VARS="${ENV_VARS} -e 'CONTAINER_ENDPOINT=/${CONTAINER_ENGINE}.sock'"
        else
            ENV_VARS="${ENV_VARS} -e 'CONTAINER_ENDPOINT=${CONTAINER_ENDPOINT}'"
        fi
    fi
    ENV_VARS="${ENV_VARS} -e 'SKUPPER_OUTPUT_PATH=${SKUPPER_OUTPUT_PATH}'"
    ENV_VARS="${ENV_VARS} -e 'SKUPPER_PLATFORM=${SKUPPER_PLATFORM}'"

    # Running the bootstrap
    ${CONTAINER_ENGINE} pull ${IMAGE}
    if eval "${CONTAINER_ENGINE}" run --rm --name skupper-bootstrap \
        --network host --security-opt label=disable -u \""${RUNAS}"\" --userns=\""${USERNS}"\" \
        "${MOUNTS}" \
        "${ENV_VARS}" \
        "${IMAGE}" 2>&1 | tee "${LOG_FILE}"; then
        create_service
    fi
}

main "$@"
