#!/bin/bash

BASE_COLLECTION_PATH="/must-gather"

# make sure we honor --since and --since-time args passed to oc must-gather
get_log_collection_args() {
  # validation of MUST_GATHER_SINCE and MUST_GATHER_SINCE_TIME is done by the
  # caller (oc adm must-gather) so it's safe to use the values as they are.
  log_collection_args=""

  if [ -n "${MUST_GATHER_SINCE:-}" ]; then
    log_collection_args=--since="${MUST_GATHER_SINCE}"
  fi
  if [ -n "${MUST_GATHER_SINCE_TIME:-}" ]; then
    log_collection_args=--since-time="${MUST_GATHER_SINCE_TIME}"
  fi
}

# Collects cgroup v2 memory/CPU metrics as "===<filename>===" delimited sections.
# CONTAINER_ID set: run on the node (oc debug), find the container's cgroup on the host.
# CONTAINER_ID empty: run in the container, read /sys/fs/cgroup directly.
read -r -d '' CGROUP_COLLECT_SCRIPT <<'COLLECT_EOF'
if [ -n "${CONTAINER_ID}" ]; then
  d=$(find /sys/fs/cgroup -type d -name "*${CONTAINER_ID}*" 2>/dev/null | head -1)
  if [ -z "$d" ]; then
    echo "===memory.info==="
    echo "cgroup directory not found on node for container ${CONTAINER_ID}"
    exit 0
  fi
else
  d=/sys/fs/cgroup
fi

echo "===memory.info==="
curr=$(cat "$d/memory.current" 2>/dev/null)
max=$(cat "$d/memory.max" 2>/dev/null)

if [ -z "$curr" ]; then
  usage_str="unknown (memory.current missing - cgroup v1?)"
else
  curr_kb=$((curr / 1024))
  curr_mb=$((curr / 1024 / 1024))
  usage_str="$curr bytes ($curr_kb KiB / $curr_mb MiB)"
fi

if [ -z "$max" ]; then
  limit_str="unknown (memory.max missing - cgroup v1?)"
elif [ "$max" = "max" ]; then
  limit_str="max (uncapped)"
else
  max_kb=$((max / 1024))
  max_mb=$((max / 1024 / 1024))
  limit_str="$max bytes ($max_kb KiB / $max_mb MiB)"
fi

echo "Usage: $usage_str - Limit: $limit_str"

echo "===memory.pressure==="
if [ -f "$d/memory.pressure" ]; then
  cat "$d/memory.pressure"
else
  echo "PSI not available on this node (kernel psi=1 not enabled)"
fi

echo "===cpu.stat==="
while read -r key val; do
  case "$key" in
    usage_usec) desc='Total CPU time consumed (user + system)' ;;
    user_usec) desc='Time spent in user-space (app logic)' ;;
    system_usec) desc='Time spent in kernel-space (syscalls, I/O)' ;;
    core_sched.force_idle_usec) desc='Time CPU forced idle for security (cross-HT side-channel protection)' ;;
    nr_periods) desc='Number of CPU quota enforcement periods (0 = no quota set)' ;;
    nr_throttled) desc='Times the container was throttled for exceeding CPU quota' ;;
    throttled_usec) desc='Total time spent throttled/waiting for CPU quota' ;;
    nr_bursts) desc='Times container used burst CPU capacity beyond quota' ;;
    burst_usec) desc='Total time spent using burst CPU capacity' ;;
    *) desc='' ;;
  esac
  if [[ $key == *_usec ]] && [ "$val" -ge 1000000 ]; then
    sec=$(awk "BEGIN {printf \"%.2f\", $val/1000000}")
    echo "$key $val ($sec s) | $desc"
  else
    echo "$key $val | $desc"
  fi
done < "$d/cpu.stat"

echo "===cpu.max==="
read -r quota period < "$d/cpu.max" 2>/dev/null
if [ -z "$quota" ] || [ -z "$period" ]; then
  echo "CPU Limit: unknown (cpu.max missing - cgroup v1?)"
elif [ "$quota" = "max" ]; then
  echo "CPU Limit: uncapped (no quota set)"
else
  cores=$(awk "BEGIN {printf \"%.2f\", $quota/$period}")
  echo "CPU Limit: $quota/$period us = $cores cores"
fi

echo "===cpu.pressure==="
if [ -f "$d/cpu.pressure" ]; then
  cat "$d/cpu.pressure"
else
  echo "PSI not available on this node (kernel psi=1 not enabled)"
fi
COLLECT_EOF

function getPodResources() {
  local ns="${1}"
  local podName="${2}"
  local container="${3}"
  local podDirName="${podName##"pod/"}"

  local resourcePath=${BASE_COLLECTION_PATH}/namespaces/${ns}/pods/${podDirName}/${container}/${container}/resources
  mkdir -p "${resourcePath}"

  echo "Collecting resource usage for pod ${podName} in ${ns}"

  # Get previous container termination info
  oc get pod "${podDirName}" -n "${ns}" -o jsonpath="{.status.containerStatuses[?(@.name=='${container}')].lastState}" > "${resourcePath}/last.terminated" 2>/dev/null
  # jsonpath emits "{}" (not an empty file) when the container has no previous
  # termination, so treat that as "nothing recorded" too.
  if [ ! -s "${resourcePath}/last.terminated" ] || [ "$(cat "${resourcePath}/last.terminated")" = "{}" ]; then
    echo "No previous termination recorded" > "${resourcePath}/last.terminated"
  fi

  # Collect cgroup metrics: exec into the container when it has bash,
  # otherwise (scratch-based images) read the container's cgroup from the
  # node via oc debug.
  local output
  if oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c 'true' >/dev/null 2>&1; then
    output=$(oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c "CONTAINER_ID='' ; ${CGROUP_COLLECT_SCRIPT}" 2>&1)
  else
    echo "  bash not available in container ${container} (scratch image), using oc debug node"
    local nodeName containerID
    nodeName=$(oc get pod "${podDirName}" -n "${ns}" -o jsonpath='{.spec.nodeName}' 2>/dev/null)
    containerID=$(oc get pod "${podDirName}" -n "${ns}" -o jsonpath="{.status.containerStatuses[?(@.name=='${container}')].containerID}" 2>/dev/null)
    containerID="${containerID##*://}"
    if [ -z "${nodeName}" ] || [ -z "${containerID}" ]; then
      echo "Unable to determine node or container ID for ${podName}/${container}" > "${resourcePath}/memory.info"
      return
    fi
    output=$(oc debug "node/${nodeName}" -q -- chroot /host bash -c "CONTAINER_ID='${containerID}' ; ${CGROUP_COLLECT_SCRIPT}" 2>&1)
  fi

  # Split the delimited sections into individual files
  echo "${output}" | awk -v dir="${resourcePath}" '
    /^===[a-z.]+===$/ { f = dir "/" substr($0, 4, length($0) - 6); next }
    f { print > f }
  '
}

function getSkupperRouterSkstatInNamespace() {
  local routerNamespace="${1}"
  local flags=("g" "c" "l" "n" "e" "a" "m" "p")
  prefix="pod/"

  for routerName in $(oc get pods -n "${sitens}" -l app.kubernetes.io/name=skupper-router -oname); do
    echo "Collecting skstats for pod ${routerName}.${routerNamespace}"
    local logPath=${BASE_COLLECTION_PATH}/namespaces/${sitens}/pods/${routerName##"$prefix"}/router/router/skstat
    mkdir -p "${logPath}"
    for flag in "${flags[@]}"; do
      oc exec -n "${sitens}" "${routerName}" -c router -- skstat -"${flag}" > "${logPath}/skstat.${flag}" 2>&1
    done
  done
}

function getSkupperResourcesInNamespace() {
  local ns="${1}"
  local labelSelector="${2:-app.kubernetes.io/part-of=skupper}"

  for podName in $(oc get pods -n "${ns}" -l "${labelSelector}" -oname); do
    for container in $(oc get "${podName}" -n "${ns}" -o jsonpath='{.spec.containers[*].name}'); do
      getPodResources "${ns}" "${podName}" "${container}"
    done
  done
}


function getCRDs() {
  local result=()
  local output

  output=$(oc get crds -o custom-columns=NAME:metadata.name --no-headers | grep -e '\.skupper\.io')

  for crd in ${output}; do
    result+=("${crd}")
  done

  echo "${result[@]}"
}

function inspect() {
  local resource ns
  resource=$1
  ns=$2

  echo
  if [ -n "$ns" ]; then
    echo "Inspecting resource ${resource} in namespace ${ns}"
    # it's here just to make the linter happy (we have to use double quotes around the variable)
    if [ -n "${log_collection_args}" ]
    then
      oc adm inspect "${log_collection_args}" "--dest-dir=${BASE_COLLECTION_PATH}" "${resource}" -n "${ns}"
    else
      oc adm inspect "--dest-dir=${BASE_COLLECTION_PATH}" "${resource}" -n "${ns}"
    fi
  else
    echo "Inspecting resource ${resource}"
    # it's here just to make the linter happy
    if [ -n "${log_collection_args}" ]
    then
      oc adm inspect "${log_collection_args}" "--dest-dir=${BASE_COLLECTION_PATH}" "${resource}"
    else
      oc adm inspect "--dest-dir=${BASE_COLLECTION_PATH}" "${resource}"
    fi
  fi
}

function inspectNamespace() {
  local ns
  ns=$1

  inspect "ns/$ns"
  for crd in $crds; do
    inspect "$crd" "$ns"
  done
  inspect roles,rolebindings "$ns"
}

function main() {
  local crds
  echo
  echo "Executing Skupper gather script"
  echo

  # set global variable which is used when calling 'oc adm inspect'
  get_log_collection_args

  # note: does not account for ns scoped controller
  controllerNamespace=$(oc get pods --all-namespaces -l app.kubernetes.io/name=skupper-controller -o jsonpath="{.items[0].metadata.namespace}")
  # this gets also logs for all pods in that namespace
  inspect "ns/$controllerNamespace"
  # collect resource utilization for the skupper-controller pod itself
  getSkupperResourcesInNamespace "${controllerNamespace}" "app.kubernetes.io/name=skupper-controller"

  inspect nodes

  for r in $(oc get clusterroles,clusterrolebindings -l application=skupper-controller -oname); do
    inspect "$r"
  done

  crds="$(getCRDs)"
  for crd in ${crds}; do
    inspect "crd/${crd}"
  done

  # iterate over all namespaces which have Sites for the skupper-controller
  for sitens in $(oc get site -A -o=jsonpath="{.items[*].metadata.namespace}"); do
    echo "Inspecting Skupper Site in ${sitens} namespace"
    inspectNamespace "${sitens}"

    getSkupperRouterSkstatInNamespace "${sitens}"
    getSkupperResourcesInNamespace "${sitens}"
  done

  # iterate over all namespaces which have AttachedConnectors
  for acns in $(oc get attachedconnector -A -o=jsonpath="{.items[*].metadata.namespace}"); do
    echo "Inspecting AttachedConnector in ${acns} namespace"
    inspectNamespace "${acns}"
    getSkupperResourcesInNamespace "${acns}"
  done

  echo
  echo
  echo "Done"
  echo
}

main "$@"