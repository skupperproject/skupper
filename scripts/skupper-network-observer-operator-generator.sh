#! /usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly SKUPPER_IMAGE_TAG=${1:-2.1.1}

readonly OPERATOR_SDK=${OPERATOR_SDK:-operator-sdk}
readonly KUBECTL=${KUBECTL:-kubectl}
readonly MIN_KUBE_VERSION=${MIN_KUBE_VERSION:-1.25.0}

readonly BUNDLE_VERSION=${BUNDLE_VERSION:-2.1.1}
readonly PREVIOUS_BUNDLE_VERSION=${PREVIOUS_BUNDLE_VERSION:-2.1.0}
readonly BUNDLE_CHANNELS=${BUNDLE_CHANNELS:-"stable-2,stable-v2.0"}
readonly BUNDLE_DEFAULT_CHANNEL=${BUNDLE_DEFAULT_CHANNEL:-stable-2}

readonly REPLACED_VERSION=network-observer-operator.v${PREVIOUS_BUNDLE_VERSION}
readonly OLM_SKIP_RANGE='">2.0.0 <2.0.1"'

readonly SKUPPER_IMAGE_REGISTRY=${SKUPPER_IMAGE_REGISTRY:-quay.io/skupper}
readonly PROMETHEUS_IMAGE_TAG=${PROMETHEUS_IMAGE_TAG:-v2.55.1}
readonly OAUTH_PROXY_IMAGE_TAG=${OAUTH_PROXY_IMAGE_TAG:-4.18.0}
readonly NGINX_IMAGE_TAG=${NGINX_IMAGE_TAG:-1.27.3-alpine}

readonly IMG=${SKUPPER_IMAGE_REGISTRY}/network-observer-operator:${SKUPPER_IMAGE_TAG}
readonly BUNDLE_IMG=${SKUPPER_IMAGE_REGISTRY}/network-observer-operator-bundle:v${BUNDLE_VERSION}

readonly SKUPPER_NETWORK_OBSERVER_SHA=${SKUPPER_NETWORK_OBSERVER_SHA:-$(skopeo inspect --format "{{.Digest}}" docker://${SKUPPER_IMAGE_REGISTRY}/network-observer:${SKUPPER_IMAGE_TAG})}
readonly PROMETHEUS_SHA=${PROMETHEUS_SHA:-$(skopeo inspect --format "{{.Digest}}" docker://quay.io/prometheus/prometheus:${PROMETHEUS_IMAGE_TAG})}
readonly OAUTH_PROXY_SHA=${OAUTH_PROXY_SHA:-$(skopeo inspect --format "{{.Digest}}" docker://quay.io/openshift/origin-oauth-proxy:${OAUTH_PROXY_IMAGE_TAG})}
readonly NGINX_SHA=${NGINX_SHA:-$(skopeo inspect --format "{{.Digest}}" docker://mirror.gcr.io/nginxinc/nginx-unprivileged:${NGINX_IMAGE_TAG})}

readonly SKUPPER_NETWORK_OBSERVER_IMAGE=${SKUPPER_NETWORK_OBSERVER_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/network-observer@${SKUPPER_NETWORK_OBSERVER_SHA}}
readonly PROMETHEUS_IMAGE=${PROMETHEUS_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/prometheus/prometheus@${PROMETHEUS_SHA}}
readonly OAUTH_PROXY_IMAGE=${OAUTH_PROXY_IMAGE:-${SKUPPER_IMAGE_REGISTRY}/openshift/origin-oauth-proxy@${OAUTH_PROXY_SHA}}
readonly NGINX_IMAGE=${NGINX_IMAGE:-mirror.gcr.io/nginxinc/nginx-unprivileged@${OAUTH_PROXY_SHA}}

DEBUG=${DEBUG:=false}

ensure::operator-sdk() {
	if ! command -v "${OPERATOR_SDK}" > /dev/null 2>&1; then
		echo "${OPERATOR_SDK} not found";
		echo "See https://sdk.operatorframework.io/ for installation and usage.";
		exit 1
	fi
}

skupper::network-observer-bundle::kustomize-manager() {
		cat << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
images:
- name: controller
  newName: ${SKUPPER_IMAGE_REGISTRY}/network-observer-operator
  newTag: ${SKUPPER_IMAGE_TAG}
resources:
- manager.yaml
EOF
}

skupper::network-observer-bundle::kustomize-default() {
		cat << EOF
# Add runAsUser to deployment to pass default openshift admission controllers
- path: runas_user_patch.yaml
  target:
    kind: Deployment
# Add missing cluster roles for manager to provision non-default resources
- path: clusterrole_patch.yaml
  target:
    kind: ClusterRole
    name: manager-role
EOF
}

skupper::network-observer-bundle::clusterrole_patch() {
		cat << EOF
- op: add
  path: /rules/-
  value:
    apiGroups:
      - ""
    resources:
      - serviceaccounts
    verbs:
      - "*"
- op: add
  path: /rules/-
  value:
    apiGroups:
      - "route.openshift.io"
    resources:
      - routes
    verbs:
      - "*"
- op: add
  path: /rules/-
  value:
    apiGroups:
      - "networking.k8s.io"
    resources:
      - ingress
    verbs:
      - "*"
- op: add
  path: /rules/-
  value:
    apiGroups:
      - "rbac.authorization.k8s.io"
    resources:
      - rolebindings
      - roles
    verbs:
      - "*"
- op: add
  path: /rules/-
  value:
    apiGroups:
      - batch
    resources:
      - jobs
    verbs:
      - "*"
- op: add
  path: /rules/-
  value:
    apiGroups:
      - skupper.io
    resources:
      - certificates
    verbs:
      - "*"
EOF
}

skupper::network-observer-bundle::runas_user_patch() {
		cat << EOF
# Set runAsUser 1001 - from helm operator sdk image
#- op: add
#  path: /spec/template/spec/containers/0/securityContext/runAsUser
#  value: 1001
- op: add
  path: /spec/template/spec/containers/0/securityContext/runAsNonRoot
  value: true
- op: add
  path: /spec/template/spec/containers/0/securityContext/seccompProfile
  value:
    type: RuntimeDefault
EOF
}

skupper::network-observer-bundle::kustomize-related-images() {
		cat << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- bases/network-observer-operator.clusterserviceversion.yaml

patches:
  - path: patch-related-images.yaml
EOF
}

skupper::network-observer-bundle::related-images() {
		cat << EOF
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: network-observer-operator.v${BUNDLE_VERSION}
  namespace: placeholder
spec:
  relatedImages:
    - image: ${SKUPPER_NETWORK_OBSERVER_IMAGE}
      name: skupper_network_observer_image    
    - image: ${PROMETHEUS_IMAGE}
      name: ose-prometheus
    - image: ${OAUTH_PROXY_IMAGE}
      name: ose-oauth-proxy
    - image: ${NGINX_IMAGE}
      name: nginx-unprivileged
EOF
}

skupper::network-observer-bundle::clusterserviceversion() {
		cat << EOF
apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  annotations:
    capabilities: Seamless Upgrades
    categories: Integration & Delivery, Networking, Streaming & Messaging
    certified: 'false'
    containerImage: quay.io/skupper/network-observer:${SKUPPER_IMAGE_TAG}
    description: Skupper Network Obsever Operator provides the ability to monitor a service network
    olm.skipRange: ${OLM_SKIP_RANGE}
    operators.operatorframework.io/builder: operator-sdk-v1.17.0+git
    operators.operatorframework.io/project_layout: go.kubebuilder.io/v3
    repository: https://github.com/skuppproject/skupper
    support: Skupper Project
  name: network-observer-operator.v${BUNDLE_VERSION}
  namespace: placeholder
  labels:
    operatorframework.io/os.linux: supported
    operatorframework.io/arch.amd64: supported
    operatorframework.io/arch.s390x: supported
spec:
  apiservicedefinitions: {}
  description: Skupper enables communication between services running in different network locations.
  displayName: Skupper
  icon:
  - base64data: PHN2ZyBpZD0iTGF5ZXJfMSIgZGF0YS1uYW1lPSJMYXllciAxIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAxMDI0IDEwMjQiPjxkZWZzPjxzdHlsZT4uY2xzLTF7ZmlsbDojMzUzNTM1O30uY2xzLTJ7ZmlsbDojMzg1ODZjO30uY2xzLTN7ZmlsbDojZmZmO30uY2xzLTR7ZmlsbDojZDVjNWI3O308L3N0eWxlPjwvZGVmcz48dGl0bGU+c2t1cHBlcl9pY29uX3JnYl9kZWZhdWx0PC90aXRsZT48cGF0aCBjbGFzcz0iY2xzLTEiIGQ9Ik05OTkuOTEsNDQ2LjQxbC0xMy42LTEuMzVjLTEuNzUtLjI2LTIyLjYtMy42OS01My4zNS0yMS44MS0xOC44MS0xMS4wOC0zNy42LTI1LjQyLTU1Ljg1LTQyLjYxLTIzLTIxLjY0LTQ1LjE1LTQ3Ljg5LTY1LjktNzhBNjAzLjIsNjAzLjIsMCwwLDAsNjI1LjgyLDEyOC40QzU0My43NCw3OS4yNyw0ODEuNDYsNjguOTMsNDY0LjMzLDY2LjkzYTMzMC44NSwzMzAuODUsMCwwLDAtMzcuODQtMi4zNEEyMjcuMjIsMjI3LjIyLDAsMCwwLDM5MCw2Ny4zN0MzNTYuNjcsNzIuNzYsMzI4LjUsODYsMzA2LjI0LDEwNi43OWMtMjAuNDYsMTkuMDktMzUuNjMsNDQuMzItNDUuMSw3NS05LjcxLDMxLjQ4LTEzLjU5LDY5LjY2LTExLjU0LDExMy40OCwyLDQyLjk0LTIuMjUsOTEuODEtMTIuMzEsMTQxLjMyYTcyOS4zMSw3MjkuMzEsMCwwLDEtNDUuNzUsMTQ0Ljg2Yy0yMC42MSw0Ny4wNy00NC43Miw4Ny44NC03MS42NSwxMjEuMTgtMjcuNDIsMzMuOTQtNTYuMjMsNTguMjEtODUuNjQsNzIuMTNsLTEyLjUyLDUuOTNMMSw3OTAuNWw3LjgyLDIxLjU1LDQuNzMsMTNjMS4zMywzLjY3LDEzLjg2LDM2LjY0LDQzLjU0LDY0LjY5LDguNDEsOCwxOS45NCwxOC4xNCwzNC41NiwyNi40MywxNy40Miw5Ljg5LDM1LjYsMTQuOSw1NCwxNC45YTEwNS4zNCwxMDUuMzQsMCwwLDAsMTUuNi0xLjE4YzU4Ljg4LTguODcsMTI2LjI3LTMwLjY2LDE3NC4yNS00OC4xNGExNDMuMjksMTQzLjI5LDAsMCwwLDE0LjU4LDE4LjdjMzUuOTUsMzkuMTEsOTMuNzEsNTguOTQsMTcxLjY1LDU4Ljk0LDM5LDAsODQtNSwxMzMuNTktMTQuOTFDNzIzLjYxLDkzMC44OCw3NzkuNTQsOTA5LDgyMS42LDg3OS4zNWMzNi4yNS0yNS41Miw2Mi4zNi01Ni42Nyw3Ny42Mi05Mi42YTE5MC4yMiwxOTAuMjIsMCwwLDAsMTUtODAuMjQsNTguNzksNTguNzksMCwwLDAsMjcuMzUsN2MxMy44NiwwLDIzLTUuMzcsMjYuMzQtNy42OGw2Ljc3LTQuNzNhOS4xNiw5LjE2LDAsMCwwLDMuODEtNi4xM2wxLjI1LTguMTdjLjMzLTIuMTQsMi42OS0xOS4zLTEuMzEtNDMuMzUsMS0xLjA1LDItMi4xLDMtMy4yLDE0LjYzLTE2LjU5LDI1LTM4LjgzLDMxLjgzLTY4LDUuNjItMjQuMTIsOC42Mi01Mi4zNiw5LjE4LTg2LjM1bC4yMy0xNCwuMzgtMjMuMloiLz48cGF0aCBjbGFzcz0iY2xzLTIiIGQ9Ik05ODMuNDgsNDcwLjFjLS45NC0uMDktOTUtMTAuOTMtMTkzLTE1My4xOUE1NzcuNTUsNTc3LjU1LDAsMCwwLDYxMi44OCwxNTBDNTM1LjExLDEwMy40Nyw0NzcuMjgsOTMuODEsNDYxLjQxLDkyYy0yNS42OC0zLTQ3LjczLTIuOS02Ny40My4yOC0yOC4yNyw0LjU4LTUyLDE1LjY3LTcwLjU1LDMzLTE3LjIsMTYtMzAuMDYsMzcuNTktMzguMjEsNjQtOC44NSwyOC42OC0xMi4zNyw2NC0xMC40NSwxMDQuODcsMi4xLDQ1LTIuMzIsOTYtMTIuOCwxNDcuNTJhNzU0LjU5LDc1NC41OSwwLDAsMS00Ny4zNSwxNDkuOTVjLTIxLjUxLDQ5LjEyLTQ2Ljc5LDkxLjgyLTc1LjEzLDEyNi45QzEwOS42Nyw3NTUuMzgsNzcuODksNzgyLDQ1LDc5Ny41M2wtMTIuNTEsNS45Myw0LjcyLDEzYTE1Ny44NCwxNTcuODQsMCwwLDAsMzcuMTcsNTVjMTcuNzQsMTYuNzcsNDUuNDQsMzkuMjMsODMuMTMsMzMuNTUsNTMuODEtOC4xMSwxMTctMjgsMTY2LTQ1LjY2YTE2NC45LDE2NC45LDAsMCwxLTEyLTQ1Ljc0Yy03LjM2LDYtMTQuNDIsMTEtMjAuNTcsMTUuNDQtNC43MiwzLjM3LTkuNjEsNi44Ni0xMS42LDguNzVhMzYuNzEsMzYuNzEsMCwwLDEtMjUuMjcsMTAuMjhjLTE2LjQ2LDAtMzAuOS0xMC44Mi0zOC42My0yOS02LjMyLTE0LjgzLTcuNjEtMzMuMTEtMy42NS01MS41LDQuNy0yMS44NCwxNi44LTQzLjM5LDM1LTYyLjMyLDI5Ljg1LTMxLjA4LDQxLjc2LTU4LjgsNTAuNDYtNzksNS42Ny0xMy4yMSwxMC41Ny0yNC42MiwxOS4xNS0zMy4xMSw5LjQ3LTkuMzksMjIuODQtMTMuOTUsNDAuODgtMTMuOTVhMTY4LjI0LDE2OC4yNCwwLDAsMSwzMC44OSwzLjQxYzI0LjEtMjUuMDcsNTAuNjktMzAuODIsMTAxLjI1LTQxLjc2LDE3LjcyLTMuODQsMzkuNzktOC42MSw2Ni43NS0xNS4yMyw4MS4zMS0xOS45NSwxMzIuMS0yNC4xNCwxNjAuMzgtMjQuMTQsNy44MywwLDE0LjkyLjMxLDIxLjE0LjkzLDQuODMtMi43NiwxMy4xLTcuNjEsMjctMTYsMTQuODktOSwyOC0xNSw0My4zMS0xNSwyNC40NywwLDQzLjA5LDE0LjQ2LDgwLjE5LDQzLjI3bC4wOS4wN2M2LjA3LDQuNzEsMTIuOTQsMTAuMDUsMjAuNDUsMTUuODEsMzQuNTEsMjYuNDUsNTIuMzcsNTcuMSw2MS42LDgyLjUsMTguNjItMjkuMDgsMjUuOTItNzQuNiwyNi43OS0xMjcuNDhsLjIzLTE0WiIvPjxwYXRoIGNsYXNzPSJjbHMtMyIgZD0iTTUyMi4yOSwxNjEuMDhhOTMuNTQsOTMuNTQsMCwwLDEsMjkuODEsMi41OCw4OC43Myw4OC43MywwLDAsMSwyNS40LDEwLjc5LDc1LjY0LDc1LjY0LDAsMCwxLDE5LjIxLDE3LDYxLjM2LDYxLjM2LDAsMCwxLDExLjE4LDIxLjksNTQuMTEsNTQuMTEsMCwwLDEsMS45MSwxNS4wNyw1MS45NCw1MS45NCwwLDAsMS0yLjMxLDE0LjQ5LDU0LjgxLDU0LjgxLDAsMCwxLTYuMjUsMTMuNDYsNjEuNCw2MS40LDAsMCwxLTEwLDEyLDEyLjIsMTIuMiwwLDAsMC0yLjMyLDMsMTQsMTQsMCwwLDAtMS40MSwzLjYxLDE2LjMxLDE2LjMxLDAsMCwwLS40NCw0LDE3LjY3LDE3LjY3LDAsMCwwLC41OSw0LjIzbDMuODgsMTQuNTlhMTguMjcsMTguMjcsMCwwLDEsLjU0LDYuNCwxNi41NSwxNi41NSwwLDAsMS0xLjYyLDUuNzgsMTQuNDYsMTQuNDYsMCwwLDEtMy41MSw0LjU1LDEyLjg3LDEyLjg3LDAsMCwxLTUuMTIsMi42NmwtMzYuMDYsOS4yNmExNC43MywxNC43MywwLDAsMS02LjMzLjIsMTYuMjMsMTYuMjMsMCwwLDEtNS45LTIuMzgsMTgsMTgsMCwwLDEtNC43Ny00LjU3LDE4LjgxLDE4LjgxLDAsMCwxLTIuOTQtNi4zbC00LTE1LjczYTE4LjYyLDE4LjYyLDAsMCwwLTEuNzEtNC4zMiwxOC4xNiwxOC4xNiwwLDAsMC0yLjctMy42OCwxNy44NSwxNy44NSwwLDAsMC0zLjUtMi44NywxNi45MSwxNi45MSwwLDAsMC00LjE2LTEuODYsODkuODgsODkuODgsMCwwLDEtMTguMTMtNy41QTc5LjkxLDc5LjkxLDAsMCwxLDQ3NiwyNjYuMjdhNjguMzcsNjguMzcsMCwwLDEtMTItMTQuMzdBNTkuNTQsNTkuNTQsMCwwLDEsNDU2LjgzLDIzNWE1Mi45NCw1Mi45NCwwLDAsMSwuMzYtMjcuMzUsNTcsNTcsMCwwLDEsMTMuMjEtMjMuMTIsNzAuODEsNzAuODEsMCwwLDEsMjIuNzctMTYuMTZBODUuODcsODUuODcsMCwwLDEsNTIyLjI5LDE2MS4wOFoiLz48cGF0aCBjbGFzcz0iY2xzLTMiIGQ9Ik02NDcuMzUsMjc4bDEwLjEzLDEzLjc3YTE0LjQ1LDE0LjQ1LDAsMCwxLDIuNzUsOC41MiwxMi41MywxMi41MywwLDAsMS0uNzEsNC4yMUE5LjgxLDkuODEsMCwwLDEsNjU3LjQsMzA4bC0zMCwzMS4zM2ExNC43MiwxNC43MiwwLDAsMC0zLjczLDcuMzVBMTguNTcsMTguNTcsMCwwLDAsNjI0LDM1NWExNy41MywxNy41MywwLDAsMCwzLjg2LDcuMjUsMTMsMTMsMCwwLDAsNi44MSw0LjA2bDM4LjksOC42NmExMS4yNSwxMS4yNSwwLDAsMSw0LjUsMi4xOSwxNC43NiwxNC43NiwwLDAsMSwzLjQ3LDQsMTgsMTgsMCwwLDEsMi4xMSw1LjI1LDE5LjIyLDE5LjIyLDAsMCwxLC40Myw2LDE3LjY0LDE3LjY0LDAsMCwxLS44NCw0LjE2LDE1LjM1LDE1LjM1LDAsMCwxLTEuNjksMy41MywxMy4yMiwxMy4yMiwwLDAsMS0yLjM5LDIuNzQsMTAuNjksMTAuNjksMCwwLDEtMi45NSwxLjc5LDkuNCw5LjQsMCwwLDEtMS4yNy40MSwxMCwxMCwwLDAsMS0xLjMzLjIzLDguOTMsOC45MywwLDAsMS0xLjM3LDAsOS42Niw5LjY2LDAsMCwxLTEuNDEtLjE2bC04My0xNS45YTEzLjE0LDEzLjE0LDAsMCwwLTEuNjQtLjIxLDEyLjY1LDEyLjY1LDAsMCwwLTEuNjMsMCwxMi4zOCwxMi4zOCwwLDAsMC0xLjYyLjIyLDEyLjc1LDEyLjc1LDAsMCwwLTEuNTguNDMsMTAuODIsMTAuODIsMCwwLDAtMS41NC42MywxMi42OSwxMi42OSwwLDAsMC0xLjQ4LjgzLDE1LjEzLDE1LjEzLDAsMCwwLTEuNCwxLDE0Ljc5LDE0Ljc5LDAsMCwwLTEuMywxLjIxbC03OS4xOCw4Mi43M2ExNi40MywxNi40MywwLDAsMS0yLjY4LDIuMjgsMTYuODUsMTYuODUsMCwwLDEtMS40NS44NywxNC43OCwxNC43OCwwLDAsMS01LjQ4LDEuNjgsMTMuNTEsMTMuNTEsMCwwLDEtNC0uMjIsMTIuMzksMTIuMzksMCwwLDEtNi44Ny00LjA2bC0yLjM2LTIuNzJhMTcuNDMsMTcuNDMsMCwwLDEtMy41MS02LjQ4LDIwLjI4LDIwLjI4LDAsMCwxLS43My03LjQyQTIyLjE4LDIyLjE4LDAsMCwxLDQ3Niw0NDYuMTZsNDEuNjgtNDAuODlhMTguOCwxOC44LDAsMCwwLDUuMS04LjgzLDE5LjQ2LDE5LjQ2LDAsMCwwLDAtOS41MywxNy45LDE3LjksMCwwLDAtNC40Ni04LjExLDE1Ljg0LDE1Ljg0LDAsMCwwLTguNDMtNC41MmwtNTcuNTUtMTFhMTQuNDEsMTQuNDEsMCwwLDEtNS40LTIuMjUsMTQuNjMsMTQuNjMsMCwwLDEtNi4xLTkuMzUsMTQuMzMsMTQuMzMsMCwwLDEsLjA5LTUuOWwxLjU4LTcuMTlhMTQuNzcsMTQuNzcsMCwwLDEsMS41MS00LDE0LjUyLDE0LjUyLDAsMCwxLDIuNDktMy4yNiwxNC42OCwxNC42OCwwLDAsMSw3LTMuODhjLjUyLS4xMiwxLS4yMSwxLjU4LS4yOGExNC4xNywxNC4xNywwLDAsMSwxLjYtLjEsMTIuOCwxMi44LDAsMCwxLDEuNjMuMDgsMTEuOTIsMTEuOTIsMCwwLDEsMS42NC4yN2wxMDQuNjUsMjMuM2ExMy45NCwxMy45NCwwLDAsMCwxLjY4LjI2LDExLjY1LDExLjY1LDAsMCwwLDEuNjcsMCwxMi44MSwxMi44MSwwLDAsMCwxLjY0LS4xNSwxNC43NywxNC43NywwLDAsMCwxLjYtLjM2LDEyLjM0LDEyLjM0LDAsMCwwLDEuNTQtLjU2LDEyLjY0LDEyLjY0LDAsMCwwLDEuNDctLjc0LDEyLjksMTIuOSwwLDAsMCwxLjM4LS45NCwxMy41LDEzLjUsMCwwLDAsMS4yOS0xLjExWiIvPjxwYXRoIGNsYXNzPSJjbHMtNCIgZD0iTTk1MS41OCw2ODIuNVM5NDgsNjg1LDk0MS41Niw2ODVjLTEwLjkyLDAtMjkuOTEtNy40Mi01Mi42LTQ3LjY0aDBjLTM3LjM3LTY4LjIyLTc1LjY3LTgyLjc4LTc3LjMyLTgzLjM4YTQuNDgsNC40OCwwLDAsMC0zLDguNDRjLjM4LjE0LDM4LjgyLDE0Ljc0LDc1LjU1LDg0Ljg2bDAsLjA5Yy00LjkxLDguNC05LjQ4LDExLjMxLTkuNDgsMTEuMzEsMTUuNDUsMjQuMDYsNTQuODcsMjAyLTIyNC45NCwyNTcuODUtNTAsMTAtOTIuMzQsMTQuMzctMTI4LDE0LjM3LTE2My43NywwLTE4Ny45NS05Mi4yNy0xODIuNDMtMTU3Ljg4YTQ2LjYzLDQ2LjYzLDAsMCwwLTExLjY2LTUuNjdjNC43Mi04LjI0LDEyLjExLTIyLjMsMjMuMy00Ni4xNCw3LjEtMTUuMTMsMTAuMTctMzIsOS4xMy01MGE0LjUyLDQuNTIsMCwwLDAtMS4zOS0zLDQuMzksNC4zOSwwLDAsMC0zLjMyLTEuMjEsNC40OCw0LjQ4LDAsMCwwLTQuMTksNC43NGMxLDE2Ljc4LTEuNzQsMzEuNzEtOC4yOSw0NS42Ny0xNS4xOCwzMi4zNS0yMy4xMiw0Ni4xNS0yNi42NSw1MS42N2wwLDBjLTE5LjM5LDI0LjEtNDYsMzguMTEtNTYuNDMsNDhhOC4yLDguMiwwLDAsMS01LjY5LDIuNDhjLTE1LjI0LDAtMzEuOS00Ny41NiwxMy4yNC05NC41Nyw1MS01My4wNyw1NS42Mi05OC4yOSw2OS4xMS0xMTEuNjYsNC4yOS00LjI0LDEyLjEzLTUuNjksMjAuODQtNS42OSwxOC43MywwLDQxLjQ1LDYuNjksNDEuNDUsNi42OSwyOC4zNS0zOC43Nyw1MC44OC0zMy4yNywxNjQuMjMtNjEuMDksNzguODMtMTkuMzUsMTI3LjE2LTIzLjMzLDE1My41OS0yMy4zMywxOC40NiwwLDI2LjIyLDEuOTUsMjYuMjIsMS45NWgwYy4xOCwwLDIuNjgtLjU4LDM2LjcxLTIxLjE4LDExLjc0LTcuMTEsMjAuMTItMTAuOTEsMjguNTYtMTAuOTEsMTcuNDgsMCwzNS4xOSwxNi4zLDgzLjQxLDUzLjI3Qzk2Myw2MDcuODksOTUxLjU4LDY4Mi41LDk1MS41OCw2ODIuNVoiLz48cGF0aCBjbGFzcz0iY2xzLTEiIGQ9Ik04MzIuNDcsNjg4LjQ2czUyLTk3LTUyLjg3LTEyMS4xN1M2NDYuMTIsNjE0LjcsNjU5LjY4LDY1OWMwLDAtNDYtMS40MS01NS40NywxMy43MiwwLDAtMzEuNDYtMTAzLjkzLTE2Ny0zMS44MkMzNTcuMzQsNjgzLjM4LDM4NC45NCw3NjQuMTksNDE5LDc4Mi41MWM0Ljg3LDIuNjIsNy44Nyw5LjQ5LDIuNywyMC4xMWE1Ni4xNyw1Ni4xNywwLDAsMC01LjE0LDMyLjQ4YzQuMTQsMzAuOTUsMzMuNDgsNDQuNjMsMTA5LjI2LDM0LjNxMTEtMS41LDIxLjE5LTMuOGwxLjU4LS4zN3EyLjQ5LS41Nyw0Ljk0LTEuMThhMjUzLDI1MywwLDAsMCwxMTgtNjcuMzJjMy45My04LjEyLDQuNDItMjIuNS43NS0zMi40MS02LjU2LTE3LjY4LTI0Ljk0LTI2LjE0LTI1LjEzLTI2LjIybDAtLjA5Yy04LjUyLTQuNDktMTguNDMtNy43MS0yNS41Mi0zLjg4LTEzLjIzLDcuMTYtMjMuNjcsMTYuMjQtMjUuNTUsMS40NC0uODgtNi45MiwxLjI5LTQxLjkzLDQyLjkyLTUzLjM2czU5Ljc1LDguMjEsNjIsMTguMzVjMS41NCw2Ljg4LTIuMTksMjAuNjQtMTEuODcsMjEuMTctNS4yNS4yOC03LjI3LDQuNi05LjExLDkuNzZhOC43LDguNywwLDAsMCwxLDgsNjEuNTgsNjEuNTgsMCwwLDEsMTAuMzEsMTcuNzNjNC44OSwxMy4xOSw0Ljg2LDI3LjUsMCw0Mi42N0ExNTYuOTEsMTU2LjkxLDAsMCwwLDc3Mi43Miw4MDZsMS45My0uMzgsMi42NC0uNTVhMTY3LDE2NywwLDAsMCwzMS44My0xMC40NUM4NzIsNzY2Ljg2LDg0MS40MSw2OTUuMzEsODMyLjQ3LDY4OC40NloiLz48cGF0aCBjbGFzcz0iY2xzLTEiIGQ9Ik03NDMuNDksODMyLjA1YTE4MC41MywxODAuNTMsMCwwLDEtNjMuMS0xMS40MywyNzkuMjksMjc5LjI5LDAsMCwxLTM4LjYsMzAuMDksMjc0LjYyLDI3NC42MiwwLDAsMS03MC4yMiwzMi40OWMxNi45LDExLjM2LDQ4Ljc2LDIwLjQ1LDEwNi4yOSwzLjE0LDU1LjgxLTE2Ljc4LDc4LjY5LTM5LDg3Ljk0LTU1LjdBMTc5Ljg5LDE3OS44OSwwLDAsMSw3NDMuNDksODMyLjA1WiIvPjxwYXRoIGNsYXNzPSJjbHMtMyIgZD0iTTU3MS4xNiw2NzMuMzFjLTkuNzktMjUuMDctNDAuOTQtMzcuMTktNzUuMTItMzEuNjhhNTcuNjIsNTcuNjIsMCwxLDEtNjUuNDUsMzYuMjVjLTE0LjQ4LDE3LjQ5LTIwLjEzLDM4LjI4LTEzLjA3LDU2LjM4LDEyLDMwLjc0LDU2LjEyLDQyLDk4LjU0LDI1LjE4UzU4My4xNiw3MDQuMDUsNTcxLjE2LDY3My4zMVoiLz48cGF0aCBjbGFzcz0iY2xzLTMiIGQ9Ik03NzYsNjc2LjExYTUwLjUsNTAuNSwwLDAsMS0zMS4xMi05MC4yOGMtMjguNTEsMi41My01MS4yMiwyMC43OC01My4xNSw0NC42OC0yLjIzLDI3LjQ5LDIzLjg4LDUyLDU4LjMxLDU0Ljg0LDIwLjg5LDEuNjksNDAtNSw1Mi4zNy0xNi43QTUwLjIzLDUwLjIzLDAsMCwxLDc3Niw2NzYuMTFaIi8+PC9zdmc+
    mediatype: image/svg+xml
  installModes:
  - supported: false
    type: OwnNamespace
  - supported: false
    type: SingleNamespace
  - supported: false
    type: MultiNamespace
  - supported: true
    type: AllNamespaces
  keywords:
  - skupper
  - service
  - mesh
  - van
  links:
  - name: Skupper Network Observer Operator
    url: https://github.com/skupperproject/skupper
  maintainers:
  - email: skupper@googlegroups.com
    name: Skupper Community
  maturity: stable
  minKubeVersion: ${MIN_KUBE_VERSION}
  provider:
    name: Skupper Project
    url: https://skupper.io
  replaces: ${REPLACED_VERSION}
  selector: {}
  version: ${BUNDLE_VERSION}
EOF
}

main () {
  ensure::operator-sdk

	ktempdir=$(mktemp -d --tmpdir=./)
	if [ "${DEBUG}" != "true" ]; then
		trap 'rm -rf $ktempdir' EXIT
	fi

  CUR_DIR=$(pwd)
  cd ${ktempdir}

  # generate project scaffold
  "${OPERATOR_SDK}" init \
	--plugins helm \
	--domain skupper.io \
	--project-name network-observer-operator \
	--group observability \
	--version v2alpha1 \
	--kind NetworkObserver \
  --helm-chart ../charts/network-observer/;

  # generate kustomize csv input
  mkdir -p config/manifests/bases 
  skupper::network-observer-bundle::clusterserviceversion > "config/manifests/bases/network-observer-operator.clusterserviceversion.yaml"  

  # generate kustomize manager image
  skupper::network-observer-bundle::kustomize-manager > "config/manager/kustomization.yaml"

  # generate kustomize default and patches
  skupper::network-observer-bundle::kustomize-default >> "config/default/kustomization.yaml"
  skupper::network-observer-bundle::clusterrole_patch > "config/default/clusterrole_patch.yaml"
  skupper::network-observer-bundle::runas_user_patch > "config/default/runas_user_patch.yaml"

  # generate bundle
  "${KUBECTL}" kustomize "config/manifests" | "${OPERATOR_SDK}" generate bundle -q --version ${BUNDLE_VERSION} --channels ${BUNDLE_CHANNELS} --default-channel ${BUNDLE_DEFAULT_CHANNEL}

  # append related images
  mkdir -p config/manifests/overlays/bases
  mv bundle/manifests/network-observer-operator.clusterserviceversion.yaml config/manifests/overlays/bases  
  skupper::network-observer-bundle::related-images > config/manifests/overlays/patch-related-images.yaml
  skupper::network-observer-bundle::kustomize-related-images > config/manifests/overlays/kustomization.yaml
  "${KUBECTL}" kustomize config/manifests/overlays > bundle/manifests/network-observer-operator.v${BUNDLE_VERSION}.clusterserviceversion.yaml

  # validate bundle
  "${OPERATOR_SDK}" bundle validate ./bundle 

  # build and push images using generated makefile
  make docker-buildx IMG=${IMG} PLATFORMS=linux/arm64,linux/amd64
  make bundle-build bundle-push BUNDLE_IMG=${BUNDLE_IMG}

  # setup directory for bundle and deploy content
  rm -rf ${CUR_DIR}/network-observer-operator
  mkdir -p ${CUR_DIR}/network-observer-operator
  cp -R bundle ${CUR_DIR}/network-observer-operator/bundle
  cp bundle.Dockerfile ${CUR_DIR}/network-observer-operator/bundle
  mkdir -p ${CUR_DIR}/network-observer-operator/deploy
  cp config/samples/observability_v2alpha1_networkobserver.yaml ${CUR_DIR}/network-observer-operator/deploy/skupper-network-observer-cr-sample.yaml

  cd ${CUR_DIR}
}
main "$@"
