export PRODUCT_VERSION="2.0"
export FULL_VERSION=2.0.0
export SKUPPER_RELEASE_COMMIT=" "


export IMAGE_REGISTRY="quay.io"
export REPOSITORY_NAME="skupper"
export SKUPPER_IMAGE_TAG="2.0.0"
export SKUPPER_ROUTER_IMAGE_TAG="3.2.0"

export MANIFESTS_DIR="bundle/manifests"
export METADATA_DIR="bundle/metadata"
export REPLACED_VERSION=2.0.0-rc0
export OLM_SKIP_RANGE=">2.0.0-rc0 <2.0.0"
export OPERATOR_MATURITY=alpha

# The versioning scheme available in the openshift versions label accepts:
#     Min version (v4.6)
#     Range (v4.6-v4.8)
#     A specific version (=v4.7)
export MIN_KUBE_VERSION="1.25.0"
export SUPPORTED_OCP_VERSIONS="v4.12-v4.18"
export GOLANG_VERSION="1.22"
export OPERATOR_SDK_VERSION="1.35.0"
export BUNDLE_DEFAULT_CHANNEL="stable-2"
export BUNDLE_CHANNELS="stable-2,stable-2.0"
