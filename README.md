# openapi-sample-emulator

`openapi-sample-emulator` is a lightweight HTTP emulator that serves predefined responses based on an OpenAPI / Swagger specification.

It is designed for **local development**, **integration testing**, and **CI environments** where deterministic and predictable API behavior is required.

---

## Documentation

* [Environment Variables](./docs/ENVIRONMENT_VARIABLES.md) – Configuration options

---

## What it does

* Reads an OpenAPI / Swagger specification
* Matches incoming requests by HTTP method and path
* Resolves responses from JSON sample files (folder-based or legacy flat)
* Supports **stateful APIs** using explicit `scenario.json` definitions
* Supports **step-based** and **time-based** state progression
* Optionally falls back to examples defined in the OpenAPI spec
* Can enforce basic request validation (e.g. required request body)

---

## Why use it

This tool is useful when:

* You need **deterministic, repeatable responses**
* You want to test integrations without running real services
* Your API spec has limited or missing examples
* CI tests must be stable and predictable
* You want to simulate **long-running or stateful APIs**
  (e.g. scans, jobs, tasks, workflows)

---

## How responses are resolved

For each request, the emulator resolves responses in the following order:

1. **Scenario-based responses** (`scenario.json`, if present)
2. **Folder-based sample files**
3. **Legacy flat sample files** (optional)
4. **OpenAPI response examples** (if enabled)
5. Otherwise, an error response is returned

The resolution behavior is controlled via `LAYOUT_MODE`.

---

## Folder-based sample layout (recommended)

The recommended layout mirrors the API path structure:

```
SAMPLES_DIR/
  api/
    v1/
      items/
        GET.json
        POST.json
        {id}/
          GET.json
```

### Naming rules

```
<path>/<METHOD>[.<state>].json
```

Examples:

* `GET /api/v1/items` - `api/v1/items/GET.json`
* `POST /scans` - `scans/POST.json`
* `GET /scans/{id}` - `scans/{id}/GET.json`

Path parameters remain as `{id}`.

---

## Stateful APIs with `scenario.json`

Stateful behavior is defined **explicitly per endpoint** using a `scenario.json` file placed in that endpoint’s folder.

### Example folder

```
scans/{id}/status/
  scenario.json
  GET.requested.json
  GET.running.1.json
  GET.running.2.json
  GET.running.3.json
  GET.running.4.json
  GET.succeeded.json
```

---

## Step-based scenarios (recommended)

Each matching request advances the state by one step.

### Example `scenario.json`

```json
{
  "version": 1,
  "mode": "step",
  "key": { "pathParam": "id" },
  "sequence": [
    { "state": "requested", "file": "GET.requested.json" },
    { "state": "running.1", "file": "GET.running.1.json" },
    { "state": "running.2", "file": "GET.running.2.json" },
    { "state": "running.3", "file": "GET.running.3.json" },
    { "state": "running.4", "file": "GET.running.4.json" },
    { "state": "succeeded", "file": "GET.succeeded.json" }
  ],
  "behavior": {
    "advanceOn": [{ "method": "GET" }],
    "resetOn": [{ "method": "DELETE", "path": "/scans/{id}" }],
    "repeatLast": true
  }
}
```

**Behavior:**

* First `GET` - `requested`
* Each subsequent `GET` advances the state
* After the last step, the state remains `succeeded` (`repeatLast: true`)
* `DELETE /scans/{id}` resets the scenario for that `id`

This mode is **deterministic and CI-friendly**.

### Looping step scenarios (optional)

If you want the sequence to repeat from the beginning:

```json
"behavior": {
  "advanceOn": [{ "method": "GET" }],
  "repeatLast": false,
  "loop": true
}
```

---

## Time-based scenarios (optional)

State progression is based on **elapsed seconds** since the scenario starts.

### Example `scenario.json`

```json
{
  "version": 1,
  "mode": "time",
  "key": { "pathParam": "id" },
  "timeline": [
    { "afterSec": 0, "state": "requested", "file": "GET.requested.json" },
    { "afterSec": 2, "state": "running.1", "file": "GET.running.1.json" },
    { "afterSec": 7, "state": "succeeded", "file": "GET.succeeded.json" }
  ],
  "behavior": {
    "startOn": [{ "method": "GET" }],
    "resetOn": [{ "method": "DELETE", "path": "/scans/{id}" }],
    "repeatLast": true
  }
}
```

**Notes:**

* `afterSec` means “effective from this second onward”.
* With `repeatLast: true`, once the last milestone is reached it stays there.
* `startOn` controls when the timer starts. If omitted, the timer starts on first access.

### Looping time scenarios (important)

If you enable looping:

```json
"behavior": { "loop": true }
```

Make sure the final state is observable for more than an instant.

**Recommended pattern:** add a “hold” window at the end:

```json
"timeline": [
  { "afterSec": 0, "state": "requested", "file": "GET.requested.json" },
  { "afterSec": 2, "state": "running.1", "file": "GET.running.1.json" },
  { "afterSec": 7, "state": "succeeded", "file": "GET.succeeded.json" },
  { "afterSec": 9, "state": "succeeded", "file": "GET.succeeded.json" }
]
```

This keeps `succeeded` active for 2 seconds before the loop restarts, which is easier to observe in polling clients.

Time-based mode is useful for demos or UI testing, but may be less suitable for CI due to timing.

---

## Legacy flat sample files (optional)

For backward compatibility, flat files are still supported:

```
METHOD__path_with_slashes_replaced_by_underscores.json
```

Examples:

* `GET /api/v1/items` - `GET__api_v1_items.json`
* `GET /scans/{id}/results` - `GET__scans_{id}_results.json`

Enable via:

```bash
LAYOUT_MODE=flat
# or
LAYOUT_MODE=auto
```

---

## Layout modes

```bash
LAYOUT_MODE=auto     # default: scenario -> folders -> flat
LAYOUT_MODE=folders  # only folder-based layout
LAYOUT_MODE=flat     # only legacy flat files
```

---

## Validation

Optional request validation can be enabled:

```bash
VALIDATION_MODE=required
```

Currently supported:

* Required request body

If the API spec marks a request body as required, requests with an empty body are rejected with **HTTP 400**.

Supported specs:

* OpenAPI 3.x – `requestBody.required: true`
* Swagger 2.0 – `in: body` with `required: true`
  (via Swagger 2 to OpenAPI 3 conversion using
  [https://github.com/getkin/kin-openapi](https://github.com/getkin/kin-openapi))

---

## When not to use it

This tool is **not intended** to:

* Generate random or synthetic data
* Fully validate request schemas
* Replace contract-testing tools

---

## License

MIT

---
