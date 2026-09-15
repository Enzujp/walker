# Endpoint changes in CI

Walker compares snapshots of registered endpoints. It does not call your API or detect every incompatible behavior change. Authentication/body/schema changes are not compared.

## 1. Give your application an export command

Create a small command inside your application's module, for example `cmd/export-routes`. Construct your application's router and metadata declarations, call `docs.Extract(router)`, then encode the result:

```go
routes, err := docs.Extract(router)
if err != nil {
    log.Fatal(err)
}
if err := json.NewEncoder(os.Stdout).Encode(routes); err != nil {
    log.Fatal(err)
}
```

Keep router construction separate from opening an HTTP listener. If your application requires services during router setup, provide its normal test dependencies or refactor construction so exporting routes does not require production services. Include the same feature flags and route configuration when comparing versions.

## 2. Commit an initial snapshot

```sh
mkdir -p api
go run ./cmd/export-routes > api/routes.json
```

Review this first snapshot. Commit it as the comparison baseline. The snapshot may contain explicit metadata, so use synthetic request examples and credential placeholders.

## 3. Compare against the pull request's base

In your application's GitHub Actions workflow, check out enough history to read the base commit. Build Walker from a pinned reviewed release/commit once one is published. The example below assumes a compatible `walker` binary is already installed on PATH.

```yaml
permissions:
  contents: read

steps:
  - uses: actions/checkout@v4
    with:
      fetch-depth: 0
  - uses: actions/setup-go@v5
    with:
      go-version-file: go.mod
  # Install a pinned Walker version here before running the comparison.
  - name: Compare registered endpoints
    env:
      BASE_SHA: ${{ github.event.pull_request.base.sha }}
    run: |
      git show "${BASE_SHA}:api/routes.json" > /tmp/previous-routes.json
      go run ./cmd/export-routes > /tmp/current-routes.json
      walker diff /tmp/previous-routes.json /tmp/current-routes.json --fail-on-removed
```

This fragment is for `pull_request` events; `BASE_SHA` is not available on ordinary pushes. The baseline must already exist in the base commit. A missing baseline should fail so the team can establish it deliberately.

When changing routes, also regenerate and commit `api/routes.json` for future comparisons. Compare the committed snapshot with a fresh export in CI to prevent it from drifting. Generate both through the same encoder/formatter if using a byte-level check.

## Reports and policy

Use `--format json` to save machine-readable reports. Exit code **2** means the comparison completed and found removals under `--fail-on-removed`. Code **1** means the comparison failed to run correctly. Both fail a normal CI step; additions alone pass.

An intentional removal requires an explicit review decision in your project's release process. Walker does not infer intent or automatically update the baseline.

Renaming `{id}` to `{userID}`, changing a regex, or adding/removing a trailing slash is treated conservatively as a removal and an addition. The tool does not claim these changes are always breaking. A response-shape change with the same method/path is not detected.

## Runnable example in this repository

```sh
make build
./bin/walker diff example/diff/previous.json example/diff/current.json
./bin/walker diff example/diff/previous.json example/diff/current.json --fail-on-removed
```

The second command intentionally exits with code 2. `make smoke` verifies that behavior, along with successful comparisons and operational errors, through the real executable.
