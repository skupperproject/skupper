#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

VERSION=2.0.0
CHANNELS="stable-2,stable-v2.0"
DEFAULT_CHANNEL="stable-2"

rm -rf bundle
kubectl kustomize config/manifests | operator-sdk generate bundle -q --overwrite --version $VERSION --use-image-digests --channels $CHANNELS --default-channel $DEFAULT_CHANNEL
operator-sdk bundle validate ./bundle
