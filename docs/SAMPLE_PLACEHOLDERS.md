# Sample Placeholders

With `PROFILE=generic`, use `{{generate-uuid}}` as a complete JSON string value in a sample response:

```json
{
  "scan_id": "{{generate-uuid}}"
}
```

Each placeholder generates a fresh UUID v4 on every request. This also works in nested objects, arrays, and response
envelope bodies. Sample files remain unchanged.

Only UUID generation is supported. Generated UUIDs are not stored or reused across requests. Placeholders apply to
sample response bodies, not headers, OpenAPI fallback examples, or the stateful OpenVASD backend.
