# Skupper Monitoring Stack

Installs a preconfigured
[kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
instance with Skupper specific scrape configuration and Grafana dashboard.
Intended to serve as a reference for operators monitoring Skupper running in
Kubernetes and as a tool for developers working on Skupper in Kubernetes to
consistently evaluate trends in component behavior.

**Not intended for release**

## Usage

To deploy the Skupper Monitoring Stack to a namespace using Helm, printing the
useful kube-prometheus-stack subchart notes which can be useful.

```
helm install --render-subchart-notes skupper-monitoring .
```
