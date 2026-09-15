# Walker

[![CI](https://github.com/Enzujp/walker/actions/workflows/ci.yml/badge.svg)](https://github.com/Enzujp/walker/actions/workflows/ci.yml)

**Generate Postman collections from Chi APIs and review endpoint changes in CI.**

Walker reads the routes registered on a Go [Chi](https://github.com/go-chi/chi) router. Attach request metadata beside your route registrations, export a collection, and compare route snapshots before release. No HTTP listener is required for extraction.

## What you get

- **Usable requests:** authentication, headers, query parameters, path examples, and JSON bodies.
- **Named request variants:** shared route defaults with example-specific overrides.
- **Organized collections:** explicit nested folders or automatic grouping by the first path segment.
- **Checked metadata:** misspelled endpoints, nonexistent path parameters, duplicate declarations, and invalid configuration fail clearly.
- **Endpoint diffs:** stable text or JSON reports, with a CI exit code for removals.
- **Repeatable exports:** deterministic ordering, no random IDs or generated timestamps.

## Try it

Requires **Go 1.24 or later**. Python 3 is used only by development smoke checks.

```sh
git clone https://github.com/Enzujp/walker.git
cd walker
go build -o bin/walker .
./bin/walker extract --demo --config example/postman-options.json > collection.json
```

Import `collection.json` into Postman. Set the collection's `base_url` and `token` values for your API. The demo contains four endpoints and six requests, including named examples. It constructs a router but does not run a server or store users.

You can also import the checked-in [demo collection](example/collection.json).

After this version is published, install the CLI with `go install github.com/enzujp/walker@latest`.

## Integrate beside your routes

Use `walker.Docs` to attach metadata without replacing Chi or changing handler signatures. Its zero value is ready to use.

```go
package main

import (
    "log"
    "net/http"
    "os"

    "github.com/enzujp/walker/pkg/walker"
    "github.com/go-chi/chi/v5"
)

func main() {
    router := chi.NewRouter()
    var docs walker.Docs

    router.Get("/users/{id}", func(http.ResponseWriter, *http.Request) {})
    docs.Describe("GET", "/users/{id}",
        walker.Summary("Get a user"),
        walker.Group("Users/Read"),
        walker.PathParam("id", "42"),
        walker.Query(walker.QueryParam{Key: "expand", Value: "profile"}),
        walker.Headers(walker.Header{Key: "Accept", Value: "application/json"}),
    )

    router.Post("/users", func(http.ResponseWriter, *http.Request) {})
    docs.Describe("POST", "/users",
        walker.Summary("Create a user"),
        walker.Group("Users/Write"),
        walker.Examples(
            walker.RequestVariant{Name: "Ada", Body: map[string]string{
                "name": "Ada", "email": "ada@example.com",
            }},
            walker.RequestVariant{Name: "Grace", Body: map[string]string{
                "name": "Grace", "email": "grace@example.com",
            }},
        ),
    )

    routes, err := docs.Extract(router)
    if err != nil {
        log.Fatal(err)
    }
    collection, err := walker.Postman(routes, walker.Options{
        Name: "Users API",
        Auth: &walker.Auth{Type: "bearer", Token: "{{token}}"},
    })
    if err != nil {
        log.Fatal(err)
    }
    if _, err := os.Stdout.Write(append(collection, '\n')); err != nil {
        log.Fatal(err)
    }
}
```

`Describe` retains setup errors and returns them from `docs.Extract`. Declare each method/path once and use the **full path**, including mounted prefixes and trailing slashes. Routes without metadata are still exported. Metadata referring to an unregistered route is an error.

For discovery without metadata, `walker.Extract(router)` remains available. See the [working demo router](example/router.go) and [metadata reference](docs/metadata.md).

## Export from the CLI

The CLI accepts a route manifest; it cannot automatically load another application's router from a source directory. Export your application's router through the library, then use the CLI to convert or compare snapshots.

```sh
# Generate a route snapshot, including explicit metadata.
./bin/walker extract --demo --format json > routes.json

# Convert a snapshot with collection defaults.
./bin/walker extract --input routes.json --config example/postman-options.json

# Configure common settings directly; quote placeholders to keep them literal.
./bin/walker extract --input routes.json --name "Users API" \
  --base-url https://api.example.com --bearer-token '{{token}}' \
  --group-by-path

# Read a snapshot from stdin.
cat routes.json | ./bin/walker extract --input -
```

A manifest is a JSON array. Existing method/path/body manifests still work. Additional fields describe headers, query parameters, auth, path examples, groups, and variants. See [the manifest and CLI reference](docs/cli.md).

## Detect removed endpoints

```sh
./bin/walker diff example/diff/previous.json example/diff/current.json
```

```text
ADDED    POST /users
REMOVED  GET /users/{id}
```

To enforce a removal policy:

```sh
./bin/walker diff previous.json current.json --fail-on-removed --format json
```

| Exit code | Meaning |
|---|---|
| `0` | Comparison succeeded; the requested policy passed. Without `--fail-on-removed`, removals are allowed. |
| `1` | Invalid input, arguments, configuration, or I/O failure. |
| `2` | Comparison succeeded, but `--fail-on-removed` found removed endpoints. The report is still written to stdout. |

Diffs compare **HTTP method and exact route path**. A method change, parameter rename, regex change, or trailing-slash change appears as a removal plus an addition. Metadata-only changes do not affect the report. This is endpoint-change detection, not complete API compatibility analysis.

See [the CI guide](docs/ci.md) for generating snapshots from your application and comparing against a pull request's base commit.

## Design and limits

Walker separates router discovery, metadata validation, example generation, Postman serialization, and endpoint comparison. There is no mutable global route registry. Metadata containers are copied; arbitrary body values remain caller-owned.

- Chi v5 is supported. Construct the router fully before extraction. Do not mutate the router, `Docs`, or body values concurrently with export.
- Chi retains route patterns and handlers, not request types. Authentication and request metadata must be supplied explicitly.
- Postman collections and JSON manifests are supported. OpenAPI, response schemas, automatic auth discovery, and handler execution are outside the current scope.
- `{id}` and `{id:regex}` are converted to Postman parameters; regex constraints are discarded. Missing path examples use `example`. Wildcards require manual replacement.
- Normal path examples are scoped to each request. Embedded parameters such as `/files/{name}.{ext}` use deterministic, request-specific collection variables because Postman's native path parameters do not resolve every Chi pattern.
- `JSONExample` creates synthetic bodies from Go types. It respects standard JSON field rules, bounds recursion, and reports unsupported kinds. Use explicit examples for validated/custom types. See [example-generation details](docs/metadata.md#synthetic-bodies).
- Supplied bodies, credentials, and variable values are exported verbatim. Keep real secrets and customer data out of committed snapshots; use placeholders such as `{{token}}`.

## Development

```sh
make build
make test
make check
make smoke
```

Tests cover metadata validation and inheritance, nested routes, deterministic output, request variants, path handling, CLI streams, and process exit codes. CI runs formatting, static checks, race-enabled tests, build, and smoke checks. Development also includes a Postman schema/SDK compatibility check.

See [development and verification](docs/development.md) and [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[MIT](LICENSE) © 2026 Enzujp.
