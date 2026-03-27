#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

readonly KUBECTL=${KUBECTL:-kubectl}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

main () {
  # Recreate templates directory
  rm -rf "${REPO_ROOT}/charts/skupper-crds/templates"
  mkdir -p "${REPO_ROOT}/charts/skupper-crds/templates"

  # Use kustomize to label and annotate chart CRDs
  "${KUBECTL}" kustomize "${REPO_ROOT}/config/hack/helm/skupper-crds" > "${REPO_ROOT}/charts/skupper-crds/templates/crds.yaml"

  # Generate NOTES.txt
  crd_names=$(grep "^  name:" "${REPO_ROOT}/charts/skupper-crds/templates/crds.yaml" | awk '{print $2}' | sort | sed 's/^/ - /')
  cat << EOF > "${REPO_ROOT}/charts/skupper-crds/templates/NOTES.txt"
Skupper CRDs have been installed.

The following CustomResourceDefinitions are now available:
${crd_names}

To verify the installation:
  kubectl get crds -l app.kubernetes.io/name=skupper-crds

Note: CRDs are annotated with helm.sh/resource-policy: keep and will persist
after helm uninstall. To remove them, delete manually:
  kubectl get crds -l app.kubernetes.io/name=skupper-crds -o name | xargs kubectl delete
EOF

  echo "Generated skupper-crds chart templates at ${REPO_ROOT}/charts/skupper-crds/templates"
}

main "$@"
