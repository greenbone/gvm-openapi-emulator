# Agent Control Profile

The Agent Control profile is a deterministic, stateful backend for gvmd-facing
Agent Control integration tests. It extends the API document and fixtures added
by PR #44; it does not introduce a parallel Agent Control model.

Enable it with:

```env
PROFILE=agent-control
SPEC_PATH=./examples/agent-folder/swagger.json
SAMPLES_DIR=./examples/agent-folder/sample
FALLBACK_MODE=none
AGENT_CONTROL_RESET_TOKEN=a-test-only-secret
```

`make run-agent` sets these paths and the explicit profile. `make
docker-run-agent` runs the same profile from the local image.
[`agent-control.http`](../agent-control.http) contains runnable inventory,
mutation, configuration, support-bundle, scan, failure, and reset examples.

## Fail-closed boundary

The profile never calls the generic sample/OpenAPI fallback. Requests fail as
follows:

- `400` for malformed bodies, unsupported query parameters, duplicate query
  values, unknown scenario names, unknown update fields, and invalid state;
- `401` when a protected agent configuration request lacks a bearer token or a
  reset token is invalid;
- `404` for unknown resources and routes;
- `405` for a known path with the wrong method;
- `422` when an atomic agent update or deletion names an unknown agent;
- `501` for an operation present in the pinned Swagger but deliberately outside
  the issue #49 profile surface.

Enrollment, heartbeat, certificates, agent mTLS, installers, packages, and
packaged-agent behavior are intentionally not emulated. Their known Swagger
operations return `501`; they never receive a generated success response.

## Deterministic state

The declared seed is loaded from:

- `sample/api/v1/admin/agents/GET.json`;
- `sample/api/v1/admin/scan-agent-config/GET.json`;
- `sample/scans/{id}/results/GET.json` for the OpenVASD-shaped scan result seed.

Agent inventory is isolated per process. `PATCH /api/v1/admin/agents` (and the
versioned `/agent-control/v2/api` alias) applies the API's documented map of
agent IDs atomically, so this is the grouped-mutation contract—Agent Control
does not define a separate agent-group resource. Updates to authorization,
automatic update behavior, timing values, and per-agent configuration are
visible on later reads. Deletion is atomic as well. A later inventory `GET`
therefore supplies gvmd synchronization with the updated state instead of
replaying the original fixture.

Inventory pagination accepts `page` and `per_page`, both positive integers;
`per_page` is capped at 1000. The response remains the Swagger-defined JSON
array and includes `x-total-count`, `x-page`, and `x-per-page` headers.
`GET /scans` uses the same parameters and headers for isolated scan state.

`GET` and `PUT /api/v1/admin/scan-agent-config` provide read/update/read of the
`agent_defaults` and `agent_control_defaults` sections. Unknown sections and
invalid `update_to_latest` values return `400`.

## Support bundles

`GET /agent-control/v2/api/agents/{agentid}/support-bundle` accepts:

- `days=1..365` (default `7`);
- `encryption=true|false` (default `true`).

Only active, authorized agents produce a bundle. Other known agent states
return `409`; missing agents return `404`. The deterministic binary response
has `content-type: application/octet-stream`, a safe `content-disposition`
filename, and an `x-emulator-support-bundle-revision` counter. Reset restores
the counter.

## Agent-backed scans

`POST /scans` accepts the checked-in Agent Control scan schema. Every target
host must resolve uniquely to an active, authorized seeded agent by agent ID,
hostname, or IP address. Results from `GET /scans/{id}/results` are derived from
the PR #44 result fixture, use the same fields consumed from the OpenVASD
contract, and explicitly identify the scan and agent that produced them. A scan
keeps its own snapshot, so a later inventory update or deletion cannot change
its results.

Scan IDs start at `00000000-0000-4000-a000-000000000001`. Reset restores the
counter and removes all scans.

## Named failure scenarios

Set `X-Emulator-Scenario` on a known operation:

| Name | Status and behavior |
| --- | --- |
| `success` | Normal deterministic behavior; this is the default. |
| `authentication` | `401` with an explicit JSON error. |
| `unavailable` | `503` with an explicit JSON error. |
| `malformed` | `200` with deliberately malformed JSON. |
| `conflict` | `409` with an explicit JSON error. |
| `rate-limit` | `429` with `Retry-After: 1`. |
| `timeout` | Waits for `AGENT_CONTROL_TIMEOUT_DELAY`, then returns `504`. |

Injection cannot mask an unknown route, unsupported method, or invalid query.
Unknown scenario names return `400`.

## Reset and provenance

Reset is disabled unless `AGENT_CONTROL_RESET_TOKEN` is non-empty. Keep it on a
test-only network:

```console
curl -X POST \
  -H 'X-Emulator-Reset-Token: a-test-only-secret' \
  -H 'Content-Type: application/json' \
  --data '{}' \
  http://localhost:8086/_emulator/agent-control/reset
```

Every response exposes:

```text
api-version: 2
x-emulator-openapi-revision: b3367ab0648cba4e3c6c709f796cf694cd5ac1b5
x-emulator-fixture-revision: agent-control-issue-49-v1
```

`GET /_emulator/agent-control/provenance` returns these pins, the PR #44
baseline commit, and the seed paths as JSON. Exact Git-object and SHA-256 input
pins are recorded in
[`examples/agent-folder/PROVENANCE.md`](../examples/agent-folder/PROVENANCE.md).
Downstream jobs should additionally pin the emulator image by digest.
