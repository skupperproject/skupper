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
kubectl kustomize config/manifests | operator-sdk generate bundle -q --overwrite --version $FULL_VERSION --channels $BUNDLE_CHANNELS --default-channel $BUNDLE_DEFAULT_CHANNEL
mkdir -p config/manifests-overlays/bases
mv bundle/manifests/skupper-operator.clusterserviceversion.yaml config/manifests-overlays/bases
kubectl kustomize config/manifests-overlays > bundle/manifests/skupper-operator.v${FULL_VERSION}.clusterserviceversion.yaml

operator-sdk bundle validate ./bundle

###############################################################################
rm -rf config/manifests-overlays/bases
find ./config/base -name "manager.yaml" -type f -delete
find ./config/manifests -name "skupper-operator.clusterserviceversion.yaml" -type f -delete
find ./config/manifests-overlays -name "*.yaml" -type f -delete

exit 0