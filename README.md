# openapi-sample-emulator

`openapi-sample-emulator` is a small HTTP emulator that returns predefined JSON responses based on an OpenAPI / Swagger specification.

It is mainly intended for local development, integration testing, and CI environments where stable and predictable API responses are required.

---

## What it does

* Reads a Swagger / OpenAPI specification
* Matches incoming requests by method and path
* Returns responses from JSON sample files
* Optionally falls back to examples defined in the spec
* Can enforce simple request validation (e.g. required body)

---

## Why use it

This tool is useful when:

* You need deterministic responses (no random data)
* You want to test integrations without running real services
* Your API spec is Swagger 2.0 and lacks good examples
* CI tests must be repeatable and stable

---

## How responses are chosen

1. If a matching sample file exists, it is returned
2. If not, response examples from the spec are used (if available)
3. Otherwise, a minimal response is generated from the schema

---

## Sample files

Sample files are matched using this naming rule:

```
METHOD__path_with_slashes_replaced_by_underscores.json
```

Examples:

* `GET /api/v1/items` -> `GET__api_v1_items.json`
* `POST /scans` -> `POST__scans.json`
* `GET /scans/{id}/results` -> `GET__scans_{id}_results.json`

Path parameters stay as `{id}`.

---

## Using Makefile

The repository includes a `Makefile` with common targets:

```bash
make build         # Build the emulator binary (e.g., into ./bin/)
make test          # Run unit tests
make cover         # Run unit tests with coverage report
make lint          # Run linters and format the code
make fmt           # Format code
make clean         # Clean bin directory

make docker-build  # Build the Docker image
make docker-run    # Start via docker container remove when stopped
make compose-up    # Start via docker compose
make compose-down  # Stop docker compose
```

---

## Validation

Optional request validation can be enabled:

```bash
VALIDATION_MODE=required
```

Currently supported:

* Required request body (Swagger 2.0 `required: true` body parameters)

---

## When not to use it

This tool is not intended to:

* Generate random mock data
* Fully validate request schemas
* Replace contract-testing tools

---

## License

MIT

---
