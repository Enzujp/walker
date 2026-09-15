# Development and verification

Use Go 1.24 or later and Python 3 for the CLI smoke checks.

```sh
make build
make test
make check
make smoke
```

`make check` checks Go formatting, runs `go vet`, and runs race-enabled tests. `make smoke` builds the binary and verifies the checked-in exports, stdin support, output streams, and actual process exit codes 0/1/2.

## Refresh examples

When intentionally changing the demo or output format:

```sh
go build -o bin/walker .
./bin/walker extract --demo --format json > example/routes.json
./bin/walker extract --demo --config example/postman-options.json > example/collection.json
```

Review both generated diffs before committing. The smoke test checks that these files match the current implementation.

## Fuzzing

```sh
go test ./pkg/walker -run '^$' -fuzz FuzzPostmanPath -fuzztime 10s -parallel 2
```

This target exercises route-pattern parsing with arbitrary strings. Unit tests separately check regex braces, validation, named parameters, and URL generation. Two workers keep the short run predictable on shared development machines.

## Optional Postman interoperability check

Walker has no Node runtime dependency. For an independent check, install development tools in a temporary directory and validate the demo against Postman's official v2.1 schema and SDK:

```sh
npm install --prefix /tmp/walker-compat --ignore-scripts --no-audit --no-fund \
  --save-exact postman-collection@5.3.1 ajv@8.17.1 ajv-draft-04@1.0.0
curl -fsSL https://schema.postman.com/json/collection/v2.1.0/collection.json \
  -o /tmp/walker-postman-schema.json
NODE_PATH=/tmp/walker-compat/node_modules \
  node scripts/verify_postman.cjs /tmp/walker-postman-schema.json
```

The check verifies schema validity, nested folders, collection auth, a public no-auth endpoint, distinct request-local path values, disabled query parameters, and named body examples. It validates the SDK representation without making network requests to the demo API. This is not a substitute for manually importing into the Postman GUI.

## Compatibility notes

Original method/path/body JSON manifests and the `Extract`, `Normalize`, `JSONExample`, and `Postman` entry points remain supported.

Postman URLs are now structured objects rather than raw strings. Ordinary path parameters use request-local URL variables instead of shared `path_*` collection variables. This prevents different endpoints' example IDs from overwriting one another. Consumers reading generated collection JSON directly should account for this shape change.

Explicit `"body": null` survives manifest round trips and means a JSON null payload. Omitting the body retains the previous no-body/inheritance behavior.

The CLI no longer silently accepts collection-specific flags with JSON output. Keep collection defaults in a separate options file.
