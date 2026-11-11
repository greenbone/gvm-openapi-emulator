# OpenVASD Profile

The OpenVASD profile is a deterministic contract backend for integration tests
that exercise the OpenVASD HTTP boundary without starting a scanner or feed.
Enable it with:

```env
PROFILE=openvasd
OPENVASD_RESET_TOKEN=a-test-only-secret
```

The profile implements health and readiness, feed metadata, VT inventory and
metadata, scan creation and lookup, start/status/stop/delete transitions, and
result retrieval. Every scan has a unique ID and isolated state. Its result is
derived from the submitted target and VT so tests can prove request/result
correlation. Unknown routes, methods, query parameters, scenarios, and invalid
state transitions fail closed.

## Readiness

After startup, the first `GET /health/ready` returns `503`; the next and later
requests return `200`. A reset restores this transition. Alive and started are
independent and always return `200` while the process is serving requests.

All responses carry deterministic service metadata:

```text
api-version: 1
feed-version: 202611110000
authentication: disabled
x-emulator-openapi-revision: 6b0d7ec7e562aff805c8b2f968a8b0e18b5f63b2
x-emulator-fixture-revision: openvasd-issue-41-v1
```

The OpenAPI revision pins the upstream OpenVASD source contract used to build
this fixture. The fixture revision changes when deterministic test behavior or
data changes. Downstream jobs should additionally pin the emulator container by
digest; the repository's main-branch container workflow publishes the image.

`GET /vts` returns deterministic OIDs. `GET /vts?information=1` (and
`information=true`) returns the corresponding OpenVASD VT metadata objects.

## Scan lifecycle

A newly created scan is `stored`. Starting it produces these observable status
responses on successive polls:

```text
requested -> running -> succeeded
```

The `failed-scan` scenario ends in `failed` instead. Starting a requested or
running scan and stopping an already stopped scan are idempotent. Other invalid
transitions return `409`. A requested or running scan cannot be deleted.

## Named failure scenarios

Set `X-Emulator-Scenario` on a request:

| Name               | Behavior                                                   |
| ------------------ | ---------------------------------------------------------- |
| `success`          | Normal deterministic behavior (default).                   |
| `failed-scan`      | A created scan progresses to `failed`.                     |
| `http-400`         | Returns an injected `400` response.                        |
| `http-500`         | Returns an injected `500` response.                        |
| `malformed-body`   | Returns deliberately invalid JSON.                         |
| `malformed-header` | Returns an invalid `api-version` value.                     |
| `timeout`          | Delays by `OPENVASD_TIMEOUT_DELAY` before responding.      |

Unknown scenario names return `400`. `failed-scan` is accepted only while
creating a scan.

## Reset

Reset is disabled unless `OPENVASD_RESET_TOKEN` is non-empty. Keep the endpoint
on a test-only network and call it with the configured token:

```console
curl -X POST \
  -H 'X-Emulator-Reset-Token: a-test-only-secret' \
  -H 'Content-Type: application/json' \
  --data '{"not_ready_responses":1}' \
  http://emulator:8086/_emulator/reset
```

Reset deletes all scans, restores deterministic scan IDs, and configures the
number of subsequent readiness failures. Missing or incorrect authentication
returns `401`; when reset is disabled, the route returns `404`.
