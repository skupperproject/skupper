#! /usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

# Check if the script is executed with four arguments
if [ "$#" -ne 3 ]; then
    echo "Usage: $0 <scope> <controller-version> <router-version>"
    exit 1
fi

readonly SCOPE="${1-cluster}"
readonly SKUPPER_IMAGE_TAG="${2-v2-dev}"
readonly SKUPPER_ROUTER_IMAGE_TAG="${3-main}"

readonly KUBECTL=${KUBECTL:-kubectl}

readonly SKUPPER_IMAGE_REGISTRY=${SKUPPER_IMAGE_REGISTRY:-quay.io/skupper}
readonly SKUPPER_ROUTER_IMAGE=${SKUPPER_ROUTER_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/skupper-router}
readonly SKUPPER_CONTROLLER_IMAGE=${SKUPPER_CONTROLLER_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/controller}
readonly SKUPPER_KUBE_ADAPTOR_IMAGE=${SKUPPER_KUBE_ADAPTOR_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/kube-adaptor}

DEBUG=${DEBUG:=false}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

main () {
  if [ ${SCOPE} != "cluster" ] && [ ${SCOPE} != "namespace" ]; then
    echo "Scope: ${SCOPE} not recognized"
    exit 1
  fi

  ktempdir=$(mktemp -d --tmpdir="${REPO_ROOT}")
  if [ "${DEBUG}" != "true" ]; then
    trap 'rm -rf $ktempdir' EXIT
  fi

  mkdir -p ${ktempdir}/manifests

  cp "${REPO_ROOT}/config/hack/deploy/patches"/*.yaml "${ktempdir}/manifests/"

  IS_CLUSTER_SCOPED="false"
  NAMESPACE_LINE=""
  if [ ${SCOPE} == "cluster" ]; then
    IS_CLUSTER_SCOPED="true"
    NAMESPACE_LINE="namespace: skupper"
  fi

  cat << EOF > "${ktempdir}/manifests/kustomization.yaml"
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
${NAMESPACE_LINE}
resources:
- ../../config/hack/deploy/${SCOPE}
- manifest.yaml
patches:
  - path: remove-helm-management-pod-template-labels.yaml
    target:
      kind: Deployment
      labelSelector: app.kubernetes.io/managed-by=Helm
  - path: remove-helm-management-labels.yaml
    target:
      labelSelector: app.kubernetes.io/managed-by=Helm
EOF

  helm template \
    --namespace skupper \
    skupper-controller ./charts/skupper \
    --set "rbac.clusterScoped=${IS_CLUSTER_SCOPED}" \
    --set "controller.repository=${SKUPPER_CONTROLLER_IMAGE}" \
    --set "controller.tag=${SKUPPER_IMAGE_TAG}" \
    --set "kubeAdaptor.repository=${SKUPPER_KUBE_ADAPTOR_IMAGE}" \
    --set "kubeAdaptor.tag=${SKUPPER_IMAGE_TAG}" \
    --set "skupperRouter.repository=${SKUPPER_ROUTER_IMAGE}" \
    --set "skupperRouter.tag=${SKUPPER_ROUTER_IMAGE_TAG}" > "${ktempdir}/manifests/manifest.yaml"

  "${KUBECTL}" kustomize ${ktempdir}/manifests
}
main "$@"
