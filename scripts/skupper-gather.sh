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

function getPodResources() {
  local ns="${1}"
  local podName="${2}"
  local container="${3}"
  local podDirName="${podName##"pod/"}"

  local resourcePath=${BASE_COLLECTION_PATH}/namespaces/${ns}/pods/${podDirName}/${container}/${container}/resources
  mkdir -p "${resourcePath}"

  echo "Collecting resource usage for pod ${podName} in ${ns}"

  # Memory Info
  oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c '
    curr=$(cat /sys/fs/cgroup/memory.current)
    max=$(cat /sys/fs/cgroup/memory.max)
    curr_kb=$((curr / 1024))
    curr_mb=$((curr / 1024 / 1024))

    if [ "$max" = "max" ]; then
      limit_str="max (uncapped)"
    else
      max_kb=$((max / 1024))
      max_mb=$((max / 1024 / 1024))
      limit_str="$max bytes ($max_kb KiB / $max_mb MiB)"
    fi

    echo "Usage: $curr bytes ($curr_kb KiB / $curr_mb MiB) - Limit: $limit_str" > /tmp/memory.info
  ' 2>/dev/null
  oc exec -n "${ns}" "${podName}" -c "${container}" -- cat /tmp/memory.info > "${resourcePath}/memory.info" 2>&1

# Get previous container termination info
oc get pod "${podName##"pod/"}" -n "${ns}" -o jsonpath="{.status.containerStatuses[?(@.name=='${container}')].lastState}" > "${resourcePath}/last.terminated" 2>/dev/null
if [ ! -s "${resourcePath}/last.terminated" ]; then
  echo "No previous termination recorded" > "${resourcePath}/last.terminated"
fi

  # Memory pressure
oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c '
    if [ -f /sys/fs/cgroup/memory.pressure ]; then
      cat /sys/fs/cgroup/memory.pressure
    else
      echo "PSI not available on this node (kernel psi=1 not enabled)"
    fi
' > "${resourcePath}/memory.pressure" 2>&1

# CPU usage
oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c "
  declare -A cpu_descriptions
  cpu_descriptions=(
    [usage_usec]='Total CPU time consumed (user + system)'
    [user_usec]='Time spent in user-space (app logic)'
    [system_usec]='Time spent in kernel-space (syscalls, I/O)'
    [core_sched.force_idle_usec]='Time CPU forced idle for security (cross-HT side-channel protection)'
    [nr_periods]='Number of CPU quota enforcement periods (0 = no quota set)'
    [nr_throttled]='Times the container was throttled for exceeding CPU quota'
    [throttled_usec]='Total time spent throttled/waiting for CPU quota'
    [nr_bursts]='Times container used burst CPU capacity beyond quota'
    [burst_usec]='Total time spent using burst CPU capacity'
  )

  cat /sys/fs/cgroup/cpu.stat | while read line; do
    val=\$(echo \$line | awk '{print \$2}')
    key=\$(echo \$line | awk '{print \$1}')
    desc=\${cpu_descriptions[\$key]}
    if [[ \$key == *_usec ]] && [ \$val -ge 1000000 ]; then
      sec=\$(awk \"BEGIN {printf \\\"%.2f\\\", \$val/1000000}\")
      echo \"\$line (\$sec s) | \$desc\"
    else
      echo \"\$line | \$desc\"
    fi
  done
" > "${resourcePath}/cpu.stat" 2>&1

  # CPU limit
  oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c '
    cpu_max=$(cat /sys/fs/cgroup/cpu.max)
    quota=$(echo $cpu_max | awk "{print \$1}")
    period=$(echo $cpu_max | awk "{print \$2}")
    if [ "$quota" = "max" ]; then
      echo "CPU Limit: uncapped (no quota set)"
    else
      cores=$(awk "BEGIN {printf \"%.2f\", $quota/$period}")
      echo "CPU Limit: $quota/$period us = $cores cores"
    fi
  ' > "${resourcePath}/cpu.max" 2>&1

  # CPU pressure
oc exec -n "${ns}" "${podName}" -c "${container}" -- bash -c '
  if [ -f /sys/fs/cgroup/cpu.pressure ]; then
    cat /sys/fs/cgroup/cpu.pressure
  else
    echo "PSI not available on this node (kernel psi=1 not enabled)"
  fi
' > "${resourcePath}/cpu.pressure" 2>&1
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

  for podName in $(oc get pods -n "${ns}" -l app.kubernetes.io/part-of=skupper -oname); do
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