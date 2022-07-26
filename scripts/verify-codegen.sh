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
DIFFROOT_PATHS=("/pkg/apis" "/pkg/generated")
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
  # sanity check
  if [[ -f ${SCRIPT_ROOT}/go.mod ]] && grep -q '^module github.com/skupperproject/skupper$' ${SCRIPT_ROOT}/go.mod; then
    rm -rf "${_tmp}"
  fi
}
trap "cleanup" EXIT SIGINT

cleanup

# adding current files
mkdir -p "${_tmp}/pkg"
for diffroot_path in ${DIFFROOT_PATHS[@]}; do
    cp -r ${SCRIPT_ROOT}${diffroot_path} ${_tmp}/pkg/
done

DO_NOT_UPDATE=true "${SCRIPT_ROOT}/scripts/update-codegen.sh" "${_tmp}"
OUTDATED_PATHS=()
echo "comparing against freshly generated codegen"
for diffroot_path in ${DIFFROOT_PATHS[@]}; do
  ret=0
  diff -Naupr "${SCRIPT_ROOT}${diffroot_path}" "${_tmp}${diffroot_path}" || ret=$?
  if [[ $ret -ne 0 ]]
  then
    OUTDATED_PATHS+=(${diffroot_path})
  fi
done

if [[ ${#OUTDATED_PATHS[@]} -gt 0 ]]; then
    echo "${OUTDATED_PATHS[@]} out of date. Please run scripts/update-codegen.sh"
    exit 1
fi
echo "${DIFFROOT_PATHS[@]} up to date."
