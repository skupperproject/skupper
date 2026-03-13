# Service Scale Test

This scenario stress-tests Skupper by creating a large number of services via Listener/Connector pairs across two sites. It focuses on scale along these dimensions:

- Number of Listeners (west)
- Number of Connectors (east)

It provisions two sites (`west` and `east`), deploys a single backend app on the `east` site, and then creates N service pairs that route to the same backend (distinguished by routing keys and unique listener ports). A smoke test verifies functionality on one service; readiness checks validate the full set.

## Structure

```
service-scale/
├── inventory/
│   ├── hosts.yml
│   ├── group_vars/
│   │   └── all.yml
│   └── host_vars/
│       ├── west.yml
│       └── east.yml
├── resources/
│   ├── west/
│   │   └── site.yml
│   └── east/
│       ├── site.yml
│       └── backend.yml
└── test.yml
```

## Parameters (defaults in `inventory/group_vars/all.yml`)

- `service_count`: number of Listener/Connector pairs
- `service_name_prefix` (default: `svc`): service/CR name prefix
- `routing_key_prefix` (default: `svc`): routing key prefix
- `base_listener_port` (default: 10080): port base for west Listeners (`base + i`)
- `backend_app_label` (default: `backend`): label selector for backend pods
- `backend_port` (default: 8080): backend container port
- `smoke_test_index` (default: 1): which service to curl
- `ready_retries`/`ready_delay`: readiness polling

To override, update `inventory/group_vars/all.yml` or pass `-e var=value`.

## Acceptance Criteria

- Functional:
  - All `service_count` Connectors and Listeners reach status “Ready”.
  - Smoke request from `west` to `svc-<smoke_test_index>` returns success (via `e2e.tests.run_curl`).
- Non-functional:
  - All resources become Ready within the allotted retries/delay window (configurable via `ready_retries` and `ready_delay`).

## Prerequisites

- Skupper v2 controller installed cluster-wide
- Two accessible Kubernetes contexts (west/east) or a single cluster with both namespaces
- `kubeconfig` paths set in `inventory/host_vars/west.yml` and `inventory/host_vars/east.yml`

## Run

From `skupper/tests`:

```bash
make create-venv FORCE=true
make test TEST="service-scale"
```

Override scale:

```bash
make test TEST="service-scale" EXTRA_VARS="-e service_count=500 -e ready_retries=900 -e ready_delay=2"
```

Skip teardown:

```bash
make test TEST="service-scale" EXTRA_VARS="-e skip_teardown=true"
```

## Notes

- All services route to the same backend Deployment on `east`. This validates control-plane scale without multiplying app pods.
- Each Listener uses a unique port (`base_listener_port + i`) and unique `routingKey` (`svc-i`).

