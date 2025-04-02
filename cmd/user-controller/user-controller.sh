#!/bin/sh

set -Ceu

IMAGE="${SKUPPER_USER_CONTROLLER_IMAGE:-quay.io/fgiorgetti/user-controller:v2-dev}"
SKUPPER_OUTPUT_PATH="${XDG_DATA_HOME:-${HOME}/.local/share}/skupper"
if [ -z "${UID:-}" ]; then
    UID="$(id -u)"
    export UID
fi

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

container_env() {
    export CONTAINER_ENGINE="${CONTAINER_ENGINE:-podman}"
    # Determining if the respective binaries are available
    case "${CONTAINER_ENGINE}" in
        docker)
            command -v docker > /dev/null || exit_error "docker not found"
            export CONTAINER_ENGINE=docker
            ;;
        *)
            command -v podman > /dev/null || exit_error "podman not found"
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
        export SKUPPER_OUTPUT_PATH="/var/lib/skupper"
    fi
    mkdir -p "${SKUPPER_OUTPUT_PATH}/namespaces"
    export CONTAINER_ENDPOINT="${CONTAINER_ENDPOINT:-${CONTAINER_ENDPOINT_DEFAULT}}"
}

main() {
    # Parse Container Engine settings
    container_env

    # Must be mounted into the container
    MOUNTS=""
    ENV_VARS=""
    
    # Mounts
    if is_sock_endpoint; then
        file_container_endpoint=$(echo "${CONTAINER_ENDPOINT}" | sed -e "s#unix://##g")
        MOUNTS="${MOUNTS} -v '${file_container_endpoint}:/${CONTAINER_ENGINE}.sock:z'"
    fi
    MOUNTS="${MOUNTS} -v '${SKUPPER_OUTPUT_PATH}:/output:z'"

    # Env vars
    if is_sock_endpoint; then
        ENV_VARS="${ENV_VARS} -e 'CONTAINER_ENDPOINT=/${CONTAINER_ENGINE}.sock'"
    else
        ENV_VARS="${ENV_VARS} -e 'CONTAINER_ENDPOINT=${CONTAINER_ENDPOINT}'"
    fi
    ENV_VARS="${ENV_VARS} -e 'SKUPPER_OUTPUT_PATH=${SKUPPER_OUTPUT_PATH}'"

    # Running the user-controller
    ${CONTAINER_ENGINE} pull "${IMAGE}"
    eval "${CONTAINER_ENGINE}" run --rm --name "${USER}-skupper-controller" \
        --network host --security-opt label=disable -u \""${RUNAS}"\" --userns=\""${USERNS}"\" \
        "${MOUNTS}" \
        "${ENV_VARS}" \
        "${IMAGE}" 2>&1
}

main "$@"
