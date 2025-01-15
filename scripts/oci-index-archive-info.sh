#!/usr/bin/env bash

REGISTRY=${REGISTRY:-quay.io/skupper}
IMAGE_TAG=${IMAGE_TAG:-"v2-dev"}
ARCHIVES_PATH=${ARCHIVES_PATH:-./oci-archives}

skopeo::digest (){
  skopeo inspect oci-archive:"$1" \
    | jq -r ".Digest"
}

skopeo::digest::for::arch (){
  skopeo inspect --raw oci-archive:"$1" \
    | jq -r \
	".manifests[] | select(.platform.architecture == \"$2\" and .platform.os == \"linux\") | .digest"
}

echo "index:"
for archive in "${ARCHIVES_PATH}"/*; do
  if [ -f "$archive" ]; then
    fn=$(basename -- "$archive")
    imagename="${fn%.*}"
    digest=$(skopeo::digest "$archive" "$x")
    if [ -n "$digest" ]; then
      printf "  - %s/%s:%s@%s\n" "$REGISTRY" "$imagename" "$IMAGE_TAG" "$digest"
    fi
  fi
done
for x in "$@"; do
  echo "$x:"
  for archive in "${ARCHIVES_PATH}"/*; do
    if [ -f "$archive" ]; then
      fn=$(basename -- "$archive")
      imagename="${fn%.*}"
	  digest=$(skopeo::digest::for::arch "$archive" "$x")
	  if [ -n "$digest" ]; then
        printf "  - %s/%s:%s@%s\n" "$REGISTRY" "$imagename" "$IMAGE_TAG" "$digest"
      fi
    fi
  done
done

