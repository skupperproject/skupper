#!/usr/bin/env bash

# Copyright 2021 The Skupper Authors.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd `dirname "${BASH_SOURCE[0]}"`/.. && pwd)
DIFFROOT_PKG_PATHS=("apis" "generated")
TMP_BASE="${SCRIPT_ROOT}/_tmp"
GENERATED_PKG_DIR="${TMP_BASE}/github.com/skupperproject/skupper/pkg"

cleanup() {
  # sanity check
  if [[ -f ${SCRIPT_ROOT}/go.mod ]] && grep -q '^module github.com/skupperproject/skupper$' ${SCRIPT_ROOT}/go.mod; then
    rm -rf "${TMP_BASE}"
  fi
}
trap "cleanup" EXIT SIGINT

# adding current files
mkdir -p "${GENERATED_PKG_DIR}"
for diffroot_path in ${DIFFROOT_PKG_PATHS[@]}; do
    cp -r ${SCRIPT_ROOT}/pkg/${diffroot_path} ${GENERATED_PKG_DIR}
done

"${SCRIPT_ROOT}/scripts/update-codegen.sh" "${TMP_BASE}" || true
OUTDATED_PATHS=()
echo "comparing against freshly generated codegen"
for diffroot_path in ${DIFFROOT_PKG_PATHS[@]}; do
  ret=0
  diff -Naupr "${SCRIPT_ROOT}/pkg/${diffroot_path}" "${GENERATED_PKG_DIR}/${diffroot_path}" || ret=$?
  if [[ $ret -ne 0 ]]
  then
    OUTDATED_PATHS+=(${diffroot_path})
  fi
done

if [[ ${#OUTDATED_PATHS[@]} -gt 0 ]]; then
    echo "${OUTDATED_PATHS[@]} out of date. Please run scripts/update-codegen.sh"
    exit 1
fi
echo "${DIFFROOT_PKG_PATHS[@]} up to date."
