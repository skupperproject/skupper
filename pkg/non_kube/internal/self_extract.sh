#!/bin/sh

set -Ceu

TAR_CONTENT_START=$(awk '/^__TARBALL_CONTENT__$/ {print NR+1; exit 0;}' $0)
TMP_DIR=$(mktemp -d /tmp/skupper-bundle.XXXXX)
CUR_DIR=$(pwd)

cleanup() {
  [[ -d "${TMP_DIR}" ]] && rm -rf "${TMP_DIR}"
  cd ${CUR_DIR}
}

trap cleanup EXIT
tail -n+${TAR_CONTENT_START} $0 | tar zxf - -C ${TMP_DIR}
cd ${TMP_DIR}

