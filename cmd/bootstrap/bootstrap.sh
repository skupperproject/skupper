#!/bin/sh

set -Ceu

IMAGE="${SKUPPER_CLI_IMAGE:-quay.io/skupper/cli:2.1.1}"
ROUTER_IMAGE="${SKUPPER_ROUTER_IMAGE:-quay.io/skupper/skupper-router:3.4.0}"
export INPUT_PATH=""
export NAMESPACE=""
export FORCE_FLAG=""
export BUNDLE_STRATEGY=""
SKUPPER_OUTPUT_PATH="${XDG_DATA_HOME:-${HOME}/.local/share}/skupper"
SERVICE_DIR="${XDG_CONFIG_HOME:-${HOME}/.config}/systemd/user"
if [ -z "${UID:-}" ]; then
    UID="$(id -u)"
    export UID
fi
BOOTSTRAP_OUT="$(mktemp /tmp/skupper-bootstrap.XXXXX.out)"

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
    if [ -n "${BUNDLE_STRATEGY}" ]; then
        return 0
    fi
    return 1
}

container_env() {
    export SKUPPER_PLATFORM="${SKUPPER_PLATFORM:-podman}"
    export CONTAINER_ENGINE="${CONTAINER_ENGINE:-podman}"
    # Determining if the respective binaries are available
    case "${SKUPPER_PLATFORM}" in
        linux)
            command -v skrouterd > /dev/null || exit_error "Linux platform cannot be used: skrouterd not found"
            command -v "${CONTAINER_ENGINE}" > /dev/null || exit_error "Linux platform cannot be used: ${CONTAINER_ENGINE} (container engine) not found"
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
        export SKUPPER_OUTPUT_PATH="/var/lib/skupper"
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
    namespace="$(cat "${BOOTSTRAP_OUT}")"
    if [ -z "${namespace}" ]; then
        # unable to create SystemD service (namespace could not be identified)
        # possibly due to bootstrap failure
        return
    fi
    service_name="skupper-${namespace}.service"
    service_file="${SKUPPER_OUTPUT_PATH}/namespaces/${namespace}/internal/scripts/${service_name}"
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
            mkdir -p "${SERVICE_DIR}"
        fi
        cp -f "${service_file}" "${SERVICE_DIR}"
        systemctl --user enable --now "${service_name}"
        systemctl --user daemon-reload
    fi
}

usage() {
    echo "Use: bootstrap.sh [-p <path>] [-n <namespace>] [-b strategy] [-f]"
    echo "     -p Custom resources location on the file system for the bundle"
    echo "     -n The target namespace used for installation"
    echo "     -b The bundle strategy to be produced: bundle or tarball"
    exit 1
}

parse_opts() {
    while getopts "p:n:b:f" opt; do
        case "${opt}" in
            p)
                export INPUT_PATH="${OPTARG}"
                if [ -z "${INPUT_PATH}" ] || [ ! -d "${INPUT_PATH}" ] || [ ! -x "${INPUT_PATH}" ]; then
                    echo "Invalid custom resources path (it must be an accessible directory)"
                    usage
                fi
                INPUT_PATH="$(cd "${INPUT_PATH}" > /dev/null 2>&1 && echo "${PWD}" || echo "")"
                export INPUT_PATH
                if [ -z "${INPUT_PATH}" ]; then
                    echo "Unable to determine custom resources fully qualified path"
                    usage
                fi
                ;;
            b)
                bundle="${OPTARG}"
                case "${bundle}" in
                "bundle")
                    export BUNDLE_STRATEGY="--type=shell-script"
                    ;;
                "tarball")
                    export BUNDLE_STRATEGY="--type=tarball"
                    ;;
                esac
                ;;
            n)
                export NAMESPACE="${OPTARG}"
                if ! echo "${NAMESPACE:?}" | grep -qE '^[a-z0-9]([-a-z0-9]*[a-z0-9])?$'; then
                    echo "Invalid namespace"
                    usage
                fi
                ;;
            f)
                export FORCE_FLAG="-f"
                ;;
            *)
                usage
                ;;
        esac
    done

    # Build the namespace argument only if NAMESPACE is not empty
    NAMESPACE_ARG=""
    if [ -n "${NAMESPACE:-}" ]; then
      NAMESPACE_ARG="-n=${NAMESPACE}"
    fi
}

main() {
    parse_opts "$@"

    # Parse Skupper Platform and Container Engine settings
    container_env

    # Must be mounted into the container
    MOUNTS=""
    ENV_VARS=""

    CONTAINER_COMMAND="system start"
    
    # Mounts
    if is_sock_endpoint && is_container_platform; then
        file_container_endpoint=$(echo "${CONTAINER_ENDPOINT}" | sed -e "s#unix://##g")
        MOUNTS="${MOUNTS} -v '${file_container_endpoint}:/${CONTAINER_ENGINE}.sock:z'"
    fi
    INPUT_PATH_ARG="/input"
    if [ -n "${INPUT_PATH}" ]; then
        MOUNTS="${MOUNTS} -v '${INPUT_PATH}:/input:z'"
    else
        MOUNTS="${MOUNTS} -v '${SKUPPER_OUTPUT_PATH}/namespaces/${NAMESPACE:-default}/input/resources:/input:z'"
    fi
    MOUNTS="${MOUNTS} -v '${SKUPPER_OUTPUT_PATH}:/output:z'"
    MOUNTS="${MOUNTS} -v '${BOOTSTRAP_OUT}:/bootstrap.out:z'"

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
    ENV_VARS="${ENV_VARS} -e 'SKUPPER_ROUTER_IMAGE=${ROUTER_IMAGE}'"

    # Running the bootstrap
    ${CONTAINER_ENGINE} pull "${IMAGE}"

    if [ -n "${BUNDLE_STRATEGY}" ]; then
      CONTAINER_COMMAND="system generate-bundle skupper-install --input=\"${INPUT_PATH_ARG}\" ${BUNDLE_STRATEGY}"
    fi

    eval "${CONTAINER_ENGINE}" run --rm --name skupper-bootstrap \
            --network host --security-opt label=disable -u \""${RUNAS}"\" --userns=\""${USERNS}"\" \
            "${MOUNTS}" \
            "${ENV_VARS}" \
            --entrypoint /app/skupper \
            "${IMAGE}" \
            "${CONTAINER_COMMAND}" "${NAMESPACE_ARG}" 2>&1

    create_service
}

main "$@"
