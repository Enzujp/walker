# CLI and manifest reference

## Extract

```text
walker extract (--demo | --input PATH) [flags]

--input, -i       JSON route manifest path; - reads stdin
--demo           Export the bundled Chi router
--format, -f     postman (default) or json
--config         Postman options JSON file
--name           Collection name (default: Walker API)
--base-url       Absolute HTTP(S) base URL (default: http://localhost:8080)
--bearer-token   Collection bearer token or placeholder, e.g. '{{token}}'
--variable       KEY=VALUE collection variable; repeat for multiple keys
--group-by-path  Group ungrouped routes by their first static path segment
```

Explicit flags override matching config fields. CLI variables merge into config variables, replacing matching keys. Repeating a key in multiple `--variable` flags is an error. Variable values may include `=`. Config files must be JSON objects; unknown fields and trailing documents are rejected.

Collection flags apply only to Postman output. Using them with `--format json` is an error, so settings cannot silently disappear from a snapshot. JSON output retains route metadata; collection settings live separately in the config file.

Base URLs may include a path prefix. Credentials, queries, and fragments are not allowed. Explicit `group` metadata wins over `--group-by-path`; root and parameter-first paths remain at the collection root when they have no explicit group.

## Manifest example

```json
[
  {
    "method": "GET",
    "path": "/users/{id}",
    "summary": "Get a user",
    "description": "Read one user by ID.",
    "group": "API/Users",
    "path_params": {"id": "42"},
    "headers": [{"key": "Accept", "value": "application/json"}],
    "query": [
      {"key": "expand", "value": "profile", "description": "Related resource"},
      {"key": "debug", "value": "true", "disabled": true}
    ],
    "auth": {"type": "bearer", "token": "{{token}}"},
    "examples": [
      {"name": "Ada", "path_params": {"id": "42"}},
      {"name": "Grace", "path_params": {"id": "84"}}
    ]
  },
  {
    "method": "POST",
    "path": "/users",
    "body": {"name": "Ada", "email": "ada@example.com"}
  }
]
```

Each named example supports `name`, `description`, `body`, `headers`, `query`, `path_params`, and `auth`. See [inheritance rules](metadata.md#inheritance-and-named-variants).

The top-level array may be empty. Null, duplicate method/path pairs, unknown fields (including nested metadata fields), malformed JSON, and trailing JSON documents are rejected. Large numeric body values retain their precision when read from JSON.

An explicit `"body": null` sends JSON null; omitting `body` means no body or inheritance from the route. In Go, use `json.RawMessage("null")` to express an explicit null body. As with standard Go JSON decoding, duplicate JSON object keys are not independently rejected; the last decoded value wins. Prefer one occurrence of each key.

## Postman config

```json
{
  "name": "Users API",
  "base_url": "https://api.example.com/v1",
  "auth": {"type": "bearer", "token": "{{token}}"},
  "variables": {"token": "", "request_id": "walker-demo"},
  "group_by_path": true
}
```

A nil/omitted route auth inherits collection auth. Use `{"type":"noauth"}` for public endpoints. Store real credentials in your local Postman environment, not committed config.

## Diff

```text
walker diff PREVIOUS CURRENT [--format text|json] [--fail-on-removed]
```

Both inputs are Walker route manifests, not Postman collections. One input may be `-` for stdin; both cannot be stdin. Both are validated before any report is written.

Text output lists added endpoints, then removed endpoints, sorting each section by path and method. When there are no changes, it prints `No endpoint changes.`

JSON output always contains arrays, including for empty results:

```json
{
  "added": [{"method": "POST", "path": "/users"}],
  "removed": [{"method": "GET", "path": "/users/{id}"}]
}
```

Only normalized method and exact path determine identity. A changed path or method is a removal plus an addition. Summaries, bodies, auth, folders, and examples are ignored for comparison, but their metadata must still be valid.

Exit codes: **0** for a successful comparison that passes policy, **1** for operational/input errors, **2** for removals when `--fail-on-removed` is set. The report is written before returning code 2; the policy message goes to stderr. Output-write errors take precedence and return code 1.

Use the built binary in CI. `go run` does not preserve every child process exit code.

## Output files

Successful extraction writes only JSON to stdout. Operational errors go to stderr. Generation and validation finish before output is written, though a failing output stream can still receive partial bytes.

Shell redirection truncates an existing destination before Walker starts. To replace a tracked export safely, generate to a temporary file and rename it only after success:

```sh
./bin/walker extract --input routes.json > collection.json.tmp && mv collection.json.tmp collection.json
```
