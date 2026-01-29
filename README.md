# openapi-sample-emulator

`openapi-sample-emulator` is a small HTTP emulator that returns predefined responses based on an OpenAPI / Swagger specification.

It is mainly intended for local development, integration testing, and CI environments where stable and predictable API responses are required.

---

## Documentation

* [Environment Variables](./docs/ENVIRONMENT_VARIABLES.md) – Configuration options

---

## What it does

* Reads a Swagger / OpenAPI specification
* Matches incoming requests by method and path
* Resolves responses from JSON sample files (folder-based or legacy flat)
* Supports stateful response flows (time-based or call-based)
* Supports request-body-driven state selection
* Optionally falls back to examples defined in the spec
* Can enforce simple request validation (e.g. required body)

---

## Why use it

This tool is useful when:

* You need deterministic, repeatable responses
* You want to test integrations without running real services
* Your API spec has limited or missing examples
* CI tests must be stable and predictable
* You want to simulate **long-running or stateful APIs** (e.g. scans, jobs, tasks)

---

## How responses are chosen

For each request, the emulator resolves responses in this order:

1. **Folder-based sample files** (preferred)
2. **Legacy flat sample files** (optional)
3. **OpenAPI response examples** (if enabled)
4. Otherwise, an error response is returned

The resolution behavior is controlled via `LAYOUT_MODE`.

---

## Sample files (folder-based layout)

The recommended layout is **folder-based**, mirroring the API path structure:

```
SAMPLES_DIR/
└── api/
    └── v1/
        └── items/
            ├── GET.json
            ├── POST.json
            └── {id}/
                └── GET.json
```

### Naming rules

```
<path>/<METHOD>[.<state>].json
```

Examples:

* `GET /api/v1/items` → `api/v1/items/GET.json`
* `POST /scans` → `scans/POST.json`
* `GET /scans/{id}` → `scans/{id}/GET.json`
* Stateful response → `GET.running.json`, `GET.succeeded.json`

Path parameters remain as `{id}`.

---

## Legacy flat sample files (optional)

For backward compatibility, flat files are still supported:

```
METHOD__path_with_slashes_replaced_by_underscores.json
```

Examples:

* `GET /api/v1/items` → `GET__api_v1_items.json`
* `GET /scans/{id}/results` → `GET__scans_{id}_results.json`

Use `LAYOUT_MODE=flat` or `LAYOUT_MODE=auto` to enable this.

---

## Layout modes

```bash
LAYOUT_MODE=auto     # default: folders first, then flat
LAYOUT_MODE=folders  # only folder-based layout
LAYOUT_MODE=flat     # only legacy flat files
```

---

## Stateful response flows

The emulator can simulate **stateful APIs** where the response changes over time or calls.

### Flow definition

```bash
STATE_FLOW=requested,running*4,succeeded
```

This expands to:

```
requested
running.1
running.2
running.3
running.4
succeeded
```

### Progression modes

**Call-based progression (preferred):**

```bash
STATE_STEP_CALLS=2
```

- State advances every 2 requests.

**Time-based progression:**

```bash
STATE_STEP_SECONDS=5
```

- State advances every 5 seconds.

If both are set, `STATE_STEP_CALLS` takes precedence.

### Reset behavior

```bash
STATE_RESET_ON_LAST=true
```

When enabled, the state resets after the last state is returned once.

---

## Body-based state selection

The emulator can override the current state based on request body content.

```bash
BODY_STATES=start,stop
```

If the request body contains one of these tokens, that state is used directly.

Example:

```json
{"action":"start"}
```

- loads `POST.start.json`

This is useful for APIs where state transitions are triggered by request payloads.

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
  (via Swagger 2 → OpenAPI 3 conversion using
  [https://github.com/getkin/kin-openapi](https://github.com/getkin/kin-openapi))

---

## When not to use it

This tool is **not** intended to:

* Generate random or synthetic data
* Fully validate request schemas
* Replace contract-testing tools

---

## License

MIT

---
