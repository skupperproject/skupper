#!/bin/bash

# Check if the script is executed with two arguments
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <controller-version> <router-version>"
    exit 1
fi


VERSION="2.1.1"
APP_VERSION="$1"
ROUTER_VERSION="$2"

# Set chart name and directories
CHART_NAME="skupper"
CRD_DIR="$CHART_NAME/crds"
TEMPLATES_DIR="$CHART_NAME/templates"
DEST_DIR="./charts"
CURRENT_DIR="$PWD"

cd "$DEST_DIR" || exit

mkdir -p "$CRD_DIR"
mkdir -p "$TEMPLATES_DIR"


cat <<EOF >"$CHART_NAME/Chart.yaml"
apiVersion: v2
name: skupper
description: Helm chart for setting up Skupper.
version: $VERSION
appVersion: $APP_VERSION
EOF


cat <<EOF >"$CHART_NAME/values.yaml"
controllerImage: quay.io/skupper/controller:$APP_VERSION
kubeAdaptorImage: quay.io/skupper/kube-adaptor:$APP_VERSION
routerImage: quay.io/skupper/skupper-router:$ROUTER_VERSION

# available options: cluster, namespace
scope: cluster

EOF

cat <<EOF >"$TEMPLATES_DIR/NOTES.txt"
{{- if eq .Values.scope "cluster"}}
===========================================================
  Skupper chart is now installed in the cluster.
  Skupper controller was deployed in the namespace "{{ .Release.Namespace }}".

===========================================================
{{- end }}
{{- if eq .Values.scope "namespace"}}
===========================================================
  Skupper chart is now installed in your current namespace.
===========================================================
{{- end }}
EOF



CRD_SOURCE_DIR="../config/crd/bases"


if [ ! -d "$CRD_SOURCE_DIR" ]; then
    echo "Source directory '$CRD_SOURCE_DIR' does not exist. Exiting."
    exit 1
fi



cp "$CRD_SOURCE_DIR"/* "$CRD_DIR"

CLUSTER_TEMPLATE="$TEMPLATES_DIR/cluster-controller-deployment.yaml"

echo "{{ if eq .Values.scope \"cluster\" }}" > "$CLUSTER_TEMPLATE" # Add Helm conditional block
pushd ${CURRENT_DIR}
./scripts/skupper-deployment-generator.sh cluster ${APP_VERSION} ${ROUTER_VERSION} true >> ${DEST_DIR}/"$CLUSTER_TEMPLATE" # Append kustomize output
popd
if [ $? -eq 0 ]; then
    echo "{{ end }}" >> "$CLUSTER_TEMPLATE"
else
    echo "Failed to generate cluster scope templates. Please check your kustomize configuration."
    exit 1
fi

# Generate namespace scope template
NAMESPACE_TEMPLATE="$TEMPLATES_DIR/namespace-controller-deployment.yaml"
echo "{{ if eq .Values.scope \"namespace\" }}" > "$NAMESPACE_TEMPLATE" # Add Helm conditional block
pushd ${CURRENT_DIR}
./scripts/skupper-deployment-generator.sh namespace ${APP_VERSION} ${ROUTER_VERSION} true >> ${DEST_DIR}/"$NAMESPACE_TEMPLATE" # Append kustomize output
popd
if [ $? -eq 0 ]; then
    echo "{{ end }}" >> "$NAMESPACE_TEMPLATE" # Close Helm conditional block
else
    echo "Failed to generate namespace scope templates. Please check your kustomize configuration."
    exit 1
fi

# Substitute "namespace: <name>" with "namespace: {{ .Release.Namespace }}"
sed -i 's/namespace: [a-zA-Z0-9.-]*/namespace: {{ .Release.Namespace }}/g' "$CLUSTER_TEMPLATE"

sed -i -E 's|quay.io/skupper/controller:[a-zA-Z0-9.-]*|{{ .Values.controllerImage }}|' "$CLUSTER_TEMPLATE"
sed -i -E 's|quay.io/skupper/controller:[a-zA-Z0-9.-]*|{{ .Values.controllerImage }}|' "$NAMESPACE_TEMPLATE"

sed -i -E 's|quay.io/skupper/skupper-router:[a-zA-Z0-9.-]*|{{ .Values.routerImage }}|' "$CLUSTER_TEMPLATE"
sed -i -E 's|quay.io/skupper/skupper-router:[a-zA-Z0-9.-]*|{{ .Values.routerImage }}|' "$NAMESPACE_TEMPLATE"


sed -i 's|quay.io/skupper/kube-adaptor:[a-zA-Z0-9.-]*|{{ .Values.kubeAdaptorImage }}|g' "$CLUSTER_TEMPLATE"
sed -i 's|quay.io/skupper/kube-adaptor:[a-zA-Z0-9.-]*|{{ .Values.kubeAdaptorImage }}|g' "$NAMESPACE_TEMPLATE"


echo "Helm chart directory structure created successfully for '$CHART_NAME' with version=$VERSION and appVersion=$APP_VERSION."