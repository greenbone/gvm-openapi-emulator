# Environment Variables

This document lists the environment variables used by **openapi-sample-emulator** and their purpose.
Defaults are shown in parentheses.

---

## Core Configuration

| Variable          | Default              | Description                                                                 |
| ----------------- | -------------------- | --------------------------------------------------------------------------- |
| `SERVER_PORT`     | `8086`               | Port the emulator listens on.                                               |
| `SPEC_PATH`       | `/work/swagger.json` | Path to the OpenAPI / Swagger spec file (JSON).                             |
| `SAMPLES_DIR`     | `/work/sample`       | Directory containing JSON sample response files.                            |
| `LOG_LEVEL`       | `info`               | Logging level (`debug`, `info`, `warn`, `error`).                           |
| `RUNNING_ENV`     | `docker`             | Runtime environment (`docker`, `k8s`, `local`).                             |
| `VALIDATION_MODE` | `required`           | Request validation mode (`none`, `required`).                               |
| `FALLBACK_MODE`   | `openapi_examples`   | Fallback behavior if a sample file is missing (`none`, `openapi_examples`). |
| `DEBUG_ROUTES`    | `false`              | If `true`, prints resolved route - sample mappings on startup.              |
| `LAYOUT_MODE`     | `auto`               | Sample file layout mode (`auto`, `folders`, `flat`).                        |

---

## Scenario Configuration (Stateful APIs)

Stateful behavior is defined using **explicit `scenario.json` files** placed next to endpoint samples.

Legacy env-based state flow configuration has been **removed**.

| Variable            | Default         | Description                                                |
| ------------------- | --------------- | ---------------------------------------------------------- |
| `SCENARIO_ENABLED`  | `true`          | Enables scenario-based response resolution.                |
| `SCENARIO_FILENAME` | `scenario.json` | Name of the scenario file to look for in endpoint folders. |

### Behavior

When scenarios are enabled:

1. The emulator checks for `<endpoint>/<SCENARIO_FILENAME>`
2. If found, responses are resolved via the scenario definition
3. Otherwise, normal sample resolution applies

Scenarios are evaluated **per endpoint and per key** (e.g. `{id}`).

---

## Sample Resolution

### `LAYOUT_MODE`

Controls how non-scenario sample files are resolved:

* `auto` (default): **folder-based - legacy flat**
* `folders`: only folder-based layout
* `flat`: only legacy flat filenames

Folder-based samples:

```
SAMPLES_DIR/<path>/<METHOD>[.<state>].json
```

Examples:

* `GET /api/v1/items` - `api/v1/items/GET.json`
* `GET /api/v1/items/{id}` - `api/v1/items/{id}/GET.json`
* Stateful sample - `GET.running.1.json`

Legacy flat samples:

```
METHOD__path_with_slashes_replaced_by_underscores.json
```

Example:

```
GET__api_v1_items_{id}.json
```

---

## Validation

### `VALIDATION_MODE`

Controls basic request validation.

| Value      | Behavior                                                          |
| ---------- | ----------------------------------------------------------------- |
| `required` | Rejects requests with missing required request bodies (HTTP 400). |
| `none`     | Disables request body presence checks.                            |

Supported specs:

* OpenAPI 3.x – `requestBody.required: true`
* Swagger 2.0 – `in: body` with `required: true`
  (via conversion using `github.com/getkin/kin-openapi`)

---

## Fallback Behavior

### `FALLBACK_MODE`

Controls behavior when no sample file is found.

| Value              | Behavior                                                        |
| ------------------ | --------------------------------------------------------------- |
| `openapi_examples` | Returns response examples from the OpenAPI spec (if available). |
| `none`             | Returns an error response (HTTP 501) with detailed diagnostics. |

---

## Debugging

### `DEBUG_ROUTES`

When enabled, the emulator prints resolved route mappings on startup, e.g.:

```
GET /items/{id} -> GET__items_{id}.json
POST /items     -> POST__items.json
```

**Note:** the right-hand side reflects **legacy flat filenames** derived from the OpenAPI route table.
If you use folder-based layouts or scenarios, actual files are located under `SAMPLES_DIR/<path>/...`.

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
LAYOUT_MODE=auto           # auto | folders | flat

# Scenario support
SCENARIO_ENABLED=true
SCENARIO_FILENAME=scenario.json

# Fallback / Validation
FALLBACK_MODE=openapi_examples  # none | openapi_examples
VALIDATION_MODE=required        # none | required

# Debug
DEBUG_ROUTES=false
```

---

### Note (important)

If `SCENARIO_ENABLED=true` and a `scenario.json` exists for an endpoint, **all state behavior is driven exclusively by that file**.
No environment variable can override scenario logic.

---
