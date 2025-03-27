#!/bin/bash
#
# render templates for the SKUPPER_OPERATOR component
#
set -e
set -x
set -u
set -o pipefail

source ./scripts/render_bundle_vars.sh

SKUPPER_ROUTER_SHA=""
CONTROLLER_SHA=""
KUBE_ADAPTOR_SHA=""
CLI_SHA=""
NETWORK_OBSERVER_SHA=""

export VERSION=${VERSION:-${PRODUCT_VERSION}}

###############################################################################
###############################################################################
echo
echo Retrieving builds...
echo
CONTROLLER_SHA=$(skopeo inspect --format "{{.Digest}}" docker://${IMAGE_REGISTRY}/${REPOSITORY_NAME}/controller:${SKUPPER_IMAGE_TAG})
SKUPPER_ROUTER_SHA=$(skopeo inspect --format "{{.Digest}}" docker://${IMAGE_REGISTRY}/${REPOSITORY_NAME}/skupper-router:${SKUPPER_ROUTER_IMAGE_TAG})
KUBE_ADAPTOR_SHA=$(skopeo inspect --format "{{.Digest}}" docker://${IMAGE_REGISTRY}/${REPOSITORY_NAME}/kube-adaptor:${SKUPPER_IMAGE_TAG})
CLI_SHA=$(skopeo inspect --format "{{.Digest}}" docker://${IMAGE_REGISTRY}/${REPOSITORY_NAME}/cli:${SKUPPER_IMAGE_TAG})
NETWORK_OBSERVER_SHA=$(skopeo inspect --format "{{.Digest}}" docker://${IMAGE_REGISTRY}/${REPOSITORY_NAME}/network-observer:${SKUPPER_IMAGE_TAG})

if [[ -n "${CONTROLLER_SHA}" ]]; then
  export CONTROLLER_SHA
else
  error "Error retrieving controller image SHA"
fi
if [[ -n "${SKUPPER_ROUTER_SHA}" ]]; then
  export SKUPPER_ROUTER_SHA
else
  error "Error retrieving skupper-router image SHA"
fi
if [[ -n "${KUBE_ADAPTOR_SHA}" ]]; then
  export KUBE_ADAPTOR_SHA
else
  error "Error retrieving kube-adaptor image SHA"
fi
if [[ -n "${CLI_SHA}" ]]; then
  export CLI_SHA
else
  error "Error retrieving cli image SHA"
fi
if [[ -n "${NETWORK_OBSERVER_SHA}" ]]; then
  export NETWORK_OBSERVER_SHA
else
  error "Error retrieving network-observer image SHA"
fi

###############################################################################
for file in $(find ./config -name "*.yaml.in"); do
  echo "Processing file: $file"
  envsubst < ${file} > ${file%.in}
done

###############################################################################
rm -rf bundle
kubectl kustomize config/manifests/operator-csv | operator-sdk generate bundle -q --overwrite --version $FULL_VERSION --channels $BUNDLE_CHANNELS --default-channel $BUNDLE_DEFAULT_CHANNEL
mkdir -p config/manifests/operator-csv/overlays/bases
mv bundle/manifests/skupper-operator.clusterserviceversion.yaml config/manifests/operator-csv/overlays/bases
kubectl kustomize config/manifests/operator-csv/overlays > bundle/manifests/skupper-operator.v${FULL_VERSION}.clusterserviceversion.yaml

operator-sdk bundle validate ./bundle

###################################
kubectl kustomize ./config/manifests/cluster-scope > skupper-cluster-scope.yaml
kubectl kustomize ./config/manifests/namespace-scope > skupper-namespace-scope.yaml
###############################################################################
rm -rf config/manifests/operator-csv/overlays/bases
find ./config/manifests/operator-csv/bases -name "*.yaml" -type f -delete
find ./config/manifests/operator-csv/overlays -name "*.yaml" -type f -delete
find ./config/manifests/cluster-scope/bases -name "*.yaml" -type f -delete
find ./config/manifests/namespace-scope/bases -name "*.yaml" -type f -delete
#find ./config/manifests/operator-csv -name "skupper-operator.clusterserviceversion.yaml" -type f -delete
exit 0