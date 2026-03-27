# Skupper CRDs Helm Chart

This Helm chart installs Skupper Custom Resource Definitions (CRDs) for Kubernetes.

## Overview

The skupper-crds chart provides all CRDs required by Skupper. It is designed to be
installed separately from the main Skupper chart, allowing CRDs to persist across
Skupper upgrades and uninstalls.

## Installation

```bash
helm install skupper-crds oci://quay.io/skupper/helm/skupper-crds
```

## Uninstallation

```bash
helm uninstall skupper-crds
```

**Note:** CRDs are annotated with `helm.sh/resource-policy: keep`, which means they
will NOT be deleted when the chart is uninstalled. This is intentional to prevent
accidental data loss. To remove CRDs, delete them manually:

```bash
kubectl get crds -l app.kubernetes.io/name=skupper-crds -o name | xargs kubectl delete
```

## Important: Do Not Downgrade

**Caution:** Do not downgrade this chart to a previous version. Downgrading CRDs can
remove fields from the schema that are in use by existing resources, potentially
causing data loss or unexpected behavior. Always ensure you are upgrading to the
same or a newer version of the skupper-crds chart.

## Developer Notes

The templates directory is generated and not versioned. The canonical source of CRD
definitions is `config/crd`.

To regenerate the templates, run:

```bash
make generate-skupper-crds-chart
```
