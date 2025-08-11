#!/usr/bin/env bash

set -em

source "${SCRIPTS_DIR}/lib/utils"
source "${SCRIPTS_DIR}/lib/debug_functions"

print_env SETTINGS
declare_kubeconfig

on_ctx() {
    "$2" --context "$1" "${@:3}"
}

# Create namespaces
on_ctx west kubectl create namespace west
kubectl config set-context west --namespace west
on_ctx east kubectl create namespace east
kubectl config set-context east --namespace east

# Initialize and link
on_ctx west ./skupper init --enable-console --enable-flow-collector
on_ctx east ./skupper init
on_ctx west ./skupper token create ${DAPPER_OUTPUT}/secret.token
on_ctx east ./skupper link create ${DAPPER_OUTPUT}/secret.token

# Deploy and expose services
on_ctx west kubectl create deployment frontend --image quay.io/skupper/hello-world-frontend
on_ctx west kubectl expose deployment/frontend --port 8080 --type LoadBalancer
on_ctx east kubectl create deployment backend --image quay.io/skupper/hello-world-backend --replicas 3
on_ctx east ./skupper expose deployment/backend --port 8080

# Wait for service to become healthy
svc_ip=$(on_ctx west kubectl get service/frontend -o jsonpath={.status.loadBalancer.ingress[0].ip})
with_retries 30 sleep_on_fail 5s curl "http://${svc_ip}:8080/api/health"

echo "Success! You can access the web UI at http://${svc_ip}:8080"
