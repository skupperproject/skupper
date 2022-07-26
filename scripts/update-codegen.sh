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

GOPATH_ORIG="${GOPATH}"
SCRIPT_ROOT=$(cd `dirname "${BASH_SOURCE[0]}"`/.. && pwd)
GOPATH_NEW="${SCRIPT_ROOT}/_go_tmp"
GOPATH="${GOPATH_NEW}"
TMP_DEST="${1:-}"
mkdir ${GOPATH_NEW}

cleanup() {
    if [[ -n "${GOPATH_NEW}" && -d "${GOPATH_NEW}" ]]; then
        chmod -R 755 ${GOPATH_NEW}
        rm -rf "${GOPATH_NEW}"
    fi
}
trap "cleanup" EXIT SIGINT

API_VERSION=`grep k8s.io/apimachinery go.mod | awk '{print $NF}'`
go get -d k8s.io/code-generator@${API_VERSION}

bash ${GOPATH}/pkg/mod/k8s.io/code-generator@${API_VERSION}/generate-groups.sh "scheme,deepcopy,client,informer,lister" \
    github.com/skupperproject/skupper/pkg/generated/client github.com/skupperproject/skupper/pkg/apis \
    skupper:v1alpha1 \
    --go-header-file ./scripts/boilerplate.go.txt \
    "$@"

DO_NOT_UPDATE=${DO_NOT_UPDATE:-false}
if ! ${DO_NOT_UPDATE}; then
    cp -r ${GOPATH}/src/github.com/skupperproject/skupper/pkg/generated ./pkg/
    cp -r ${GOPATH}/src/github.com/skupperproject/skupper/pkg/apis ./pkg/
elif [[ -n "${TMP_DEST}" && -d "${TMP_DEST}" ]]; then
    echo "Copying into ${TMP_DEST}"
    cp -r ${GOPATH}/src/github.com/skupperproject/skupper/pkg/generated ${TMP_DEST}/pkg
    cp -r ${GOPATH}/src/github.com/skupperproject/skupper/pkg/apis ${TMP_DEST}/pkg
fi
