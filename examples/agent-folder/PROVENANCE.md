# Agent Control fixture provenance

The Agent Control API document and samples were introduced by gvm-openapi-emulator
PR #44 at implementation commit
`9709bd5153f466fbd181e98b8bd96af1655848dd`. Issue #49 retains those files as
the authoritative input to the stateful profile instead of introducing a second
Agent Control model.

The upstream input is pinned by both its Git object ID and its SHA-256 digest:

| Input at PR #44 | Git object | SHA-256 |
| --- | --- | --- |
| `swagger.json` | `b3367ab0648cba4e3c6c709f796cf694cd5ac1b5` | `9593da2fb85e3e922adf3a70192b9c393be70b0834579fe1e54196bcbd1e9f64` |
| `sample/api/v1/admin/agents/GET.json` | `3aa1e2689473d6c4bb3a6a97868a9ab7ae9641d2` | `3a7737dbc6ed6344d7ee23cd97c3a621614db53bb72e073acf604efac54d590b` |
| `sample/api/v1/admin/scan-agent-config/GET.json` | `4e12ebf6ebfadc4fe442442db3f95f7ff69725bf` | `29e31fa2cb09ee4954e474d44847b9ab634f24ea140690d620e375bd01d15bf4` |
| `sample/scans/{id}/results/GET.json` | `293d884a2fe610566e45ff908476a58371a040f3` | `1874e83212b633d2434938850d25d81130928a87815aabe44fb00dae5b3a3f60` |

The profile revision is `agent-control-issue-49-v1`. It is exposed in every
response as `x-emulator-fixture-revision`; the original API Git object is
exposed as `x-emulator-openapi-revision`. The same values are present in the
Swagger `info` extensions and returned by
`GET /_emulator/agent-control/provenance`.

The issue #49 profile adds deterministic behavior around the pinned inputs:
pagination, read-after-write state, default configuration updates, support
bundles, reset, named failures, and scan/result correlation. These behaviors
are emulator contract extensions and do not claim to model enrollment, mTLS,
heartbeat, certificate issuance, or packaged-agent behavior.
