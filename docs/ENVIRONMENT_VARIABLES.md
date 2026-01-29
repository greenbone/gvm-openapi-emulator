# Environment Variables

This document lists the environment variables used by **openapi-sample-emulator** and their purpose.  
Defaults are shown in parentheses.

---

## Core Configuration

| Variable          | Default              | Description                                                                 |
|-------------------|----------------------|-----------------------------------------------------------------------------|
| `SERVER_PORT`     | `8086`               | Port the emulator listens on.                                               |
| `SPEC_PATH`       | `/work/swagger.json` | Path to the OpenAPI/Swagger spec file (JSON).                               |
| `SAMPLES_DIR`     | `/work/sample`       | Directory containing JSON sample response files.                            |
| `LOG_LEVEL`       | `info`               | Logging level (`debug`, `info`, `warn`, `error`).                           |
| `RUNNING_ENV`     | `docker`             | Runtime environment (`docker`, `k8s`, `local`).                             |
| `VALIDATION_MODE` | `required`           | Request validation mode (`none`, `required`).                               |
| `FALLBACK_MODE`   | `openapi_examples`   | Fallback behavior if a sample file is missing (`none`, `openapi_examples`). |
| `DEBUG_ROUTES`    | `false`              | If `true`, prints the resolved route→sample mappings on startup.            |
| `LAYOUT_MODE`     | `auto`               | Sample file layout mode (`auto`, `folders`, `flat`).                        |

---

## Sample Resolution

### `LAYOUT_MODE`

Controls how sample files are resolved:

- `auto` (default): try **folder-based** samples first, then **legacy flat** filenames
- `folders`: only folder-based layout
- `flat`: only legacy flat filenames

Folder-based samples are expected under:

```

SAMPLES_DIR/<path>/<METHOD>[.<state>].json

````

Examples:

- `GET /api/v1/items` → `api/v1/items/GET.json`
- `GET /api/v1/items/{id}` → `api/v1/items/{id}/GET.json`
- Stateful sample → `GET.running.json`

Legacy flat samples are expected as a single filename (derived from the spec’s route mapping).

---

## Stateful Responses

The emulator can simulate APIs where responses change across time or repeated calls (e.g., job/scan status endpoints).

| Variable              | Default   | Description                                                                                       |
|-----------------------|-----------|---------------------------------------------------------------------------------------------------|
| `STATE_FLOW`          | *(empty)* | State flow specification, e.g. `requested,running*9,succeeded`. Empty disables state flow.        |
| `STATE_STEP_SECONDS`  | `0`       | Time-based progression step size (seconds). Used only if `STATE_STEP_CALLS` is `0` or not set.    |
| `STATE_STEP_CALLS`    | `1`       | Call-based progression: advance to the next state every N calls (if `> 0`, it overrides seconds). |
| `STATE_ID_PARAM`      | `id`      | Name of a path parameter to use as the “instance id” in state tracking (e.g., `scan_id`).         |
| `STATE_RESET_ON_LAST` | `false`   | If `true`, resets state after returning the last state once.                                      |

### `STATE_FLOW`

A flow spec is a comma-separated list. `name*N` expands into numbered states:

Example:

```env
STATE_FLOW=requested,running*3,succeeded
````

Expands to:

* `requested`
* `running.1`
* `running.2`
* `running.3`
* `succeeded`

### `STATE_STEP_CALLS` vs `STATE_STEP_SECONDS`

* If `STATE_STEP_CALLS > 0`, the state advances every N requests (per key).
* Otherwise, the state advances every `STATE_STEP_SECONDS` seconds.

### `STATE_ID_PARAM`

If the route includes a matching path param, it becomes part of the state key.

Example:

* Swagger template: `GET /scans/{scan_id}`
* Actual path: `GET /scans/123`
* `STATE_ID_PARAM=scan_id`
* State key becomes: `GET /scans/{scan_id} :: 123`

If the param cannot be extracted, state falls back to route-level state (shared across ids).

---

## Body-Based State Override

Sometimes state transitions are triggered by request payloads (e.g., “start scan”, “stop scan”).
The emulator can override state based on request body contents.

| Variable      | Default      | Description                                                                                      |
|---------------|--------------|--------------------------------------------------------------------------------------------------|
| `BODY_STATES` | `start,stop` | Comma-separated tokens. If the request body contains one token, that token is used as the state. |

Notes:

* Matching is substring-based (`strings.Contains`), and the **first matching token wins**.
* If a token matches, it overrides the state flow for that request.

Example:

```env
BODY_STATES=start,stop
```

Request body:

```json
{
  "action": "start"
}
```

→ state becomes `start`, so the emulator looks for `POST.start.json` (folder mode).

---

## Behavior Notes

### `FALLBACK_MODE`

When a sample file is missing:

* `openapi_examples`: try to return examples from the spec response (if available).
* `none`: return an error response (HTTP 501) with details including the swagger path, legacy filename, layout, and
  state.

### `VALIDATION_MODE`

* `required`: if the spec marks a request body as required, the emulator rejects requests with an empty body (HTTP 400).
* `none`: skips request body presence checks.

### `DEBUG_ROUTES`

When enabled, the emulator prints lines like:

```
GET /items/{id} -> GET__items_{id}.json
POST /items -> POST__items.json
```

**Note:** the right-hand side is the **legacy flat filename** mapping from the spec route table.
If you use folder-based layouts, actual sample files are typically located under `SAMPLES_DIR/<path>/...`.

---

## Sample `.env`

```env
# Server
SERVER_PORT=8086
LOG_LEVEL=info
RUNNING_ENV=docker

# Spec + Samples
SPEC_PATH=/work/swagger.json
SAMPLES_DIR=/work/sample

# Sample resolution
LAYOUT_MODE=auto              # auto | folders | flat

# Fallback / Validation
FALLBACK_MODE=openapi_examples  # none | openapi_examples
VALIDATION_MODE=required        # none | required

# Stateful responses (optional)
STATE_FLOW=requested,running*3,succeeded
STATE_STEP_CALLS=2              # if >0, overrides seconds
STATE_STEP_SECONDS=0
STATE_ID_PARAM=id
STATE_RESET_ON_LAST=false

# Body-based state override (optional)
BODY_STATES=start,stop

# Debug
DEBUG_ROUTES=false
```

---
