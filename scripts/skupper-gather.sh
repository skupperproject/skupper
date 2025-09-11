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
  done

  # iterate over all namespaces which have AttachedConnectors
  for acns in $(oc get attachedconnector -A -o=jsonpath="{.items[*].metadata.namespace}"); do
    echo "Inspecting AttachedConnector in ${acns} namespace"
    inspectNamespace "${acns}"
  done

  echo
  echo
  echo "Done"
  echo
}

main "$@"