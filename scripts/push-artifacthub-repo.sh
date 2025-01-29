#!/bin/bash

# Check if the user has provided a parameter for the name of the helm chart
if [ -z "$1" ]; then
  echo "Usage: $0 <helm-chart-name>"
  exit 1
fi


NAME=$1

cat <<EOF >"artifacthub-repo.yml"
name: "$NAME"
displayName: "$NAME Helm chart repository"
url: "skupper.io"
publisherID: "skupper-community"
contactEmail: "skupper@googlegroups.com"
EOF

oras push quay.io/skupper/helm/"$NAME":artifacthub.io --config /dev/null:application/vnd.cncf.artifacthub.config.v1+yaml artifacthub-repo.yml:application/vnd.cncf.artifacthub.repository-metadata.layer.v1.yaml
