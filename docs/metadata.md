# Route metadata

Metadata is explicit. Walker discovers endpoint identities from Chi; it does not infer authentication, body types, or query parameters from arbitrary handlers.

## Declare metadata

`walker.Docs` has no constructor requirement. Call `Describe` beside route registration, then call `docs.Extract(router)` after all routes have been registered.

```go
var docs walker.Docs
docs.Describe("GET", "/api/users/{id}",
    walker.Summary("Get a user"),
    walker.Description("Returns the selected user."),
    walker.Group("API/Users"),
    walker.PathParam("id", "42"),
)
```

Descriptions must use the full method/path, even when written inside `r.Route` or beside a mounted router. `/users` and `/users/` are different paths. HTTP methods are normalized to uppercase.

`Describe` retains the first error, including invalid metadata or duplicate declarations. `Extract` returns it before producing routes. Stale declarations list every unmatched endpoint in stable order. Undocumented routes remain in the export.

## Available options

| Option | Purpose |
|---|---|
| `Summary(text)` | Short label appended to the method/path request name. |
| `Description(text)` | Request documentation, including multiline text. |
| `Group("API/Users")` | Nested collection folders. Explicit groups override automatic grouping. |
| `RequestExample(value)` | Default literal JSON body. |
| `Headers(Header{...}, ...)` | Request headers. |
| `Query(QueryParam{...}, ...)` | Query parameters, including repeated keys and disabled entries. |
| `PathParam("id", "42")` | Example for a declared Chi path parameter. |
| `Authentication(Auth{...})` | Route authentication override. |
| `Examples(RequestVariant{...}, ...)` | Named request variants. |

Use `Route` directly when building a JSON manifest or when an application already has its own metadata registry.

## Authentication

Authentication can be configured at the collection, route, or named-example level. A nil auth value inherits from its parent. An explicit `noauth` opts out, useful for login or health endpoints.

```go
walker.Auth{Type: "bearer", Token: "{{token}}"}
walker.Auth{Type: "basic", Username: "{{username}}", Password: "{{password}}"}
walker.Auth{Type: "apikey", Key: "X-API-Key", Value: "{{api_key}}", In: "header"}
walker.Auth{Type: "apikey", Key: "api_key", Value: "{{api_key}}", In: "query"}
walker.Auth{Type: "noauth"}
```

Only fields relevant to the selected type are accepted. API keys require `in` to be `header` or `query`. Bearer auth requires a token expression; basic auth requires a username and permits an empty password.

Walker rejects a manual header/query parameter that conflicts with the effective auth helper. To provide an `Authorization` header yourself, select `noauth` explicitly. Auth metadata configures exported requests; it does not add or enforce authentication in your Go server.

## Inheritance and named variants

When a route contains named examples, each becomes a separate Postman request. The default route is not exported as an additional request. Add `RequestVariant{Name: "Default"}` to include a variant that inherits all defaults.

| Field | Example behavior |
|---|---|
| Body | Non-nil example body replaces the route body. Otherwise inherit. |
| Headers | Merge by case-insensitive key; example values replace route values. |
| Query | For each key supplied by the example, replace all route values for that key. Preserve repeated values in declaration order. |
| Path parameters | Merge by exact parameter name; example values override route values. |
| Auth | Example overrides route; route overrides collection. `noauth` disables inheritance. |
| Description | Append the example description after the route description. |
| Group and summary | Shared by the route's examples. |

An empty override list retains inherited entries. To suppress an inherited query key, override it with a disabled query parameter. Removing inherited headers or omitting an inherited body is not currently supported; put optional bodies/headers on individual variants instead. `json.RawMessage("null")` (or `"body": null` in a manifest) explicitly sends a JSON null body; an absent body and a null body are different.

JSON bodies automatically receive `Content-Type: application/json` unless an explicit Content-Type already exists. Custom media types are preserved. Duplicate headers within one declaration are rejected; header casing alone does not make keys distinct.

Examples are sorted by name. Endpoints sort by path and method. Headers sort by normalized key. Query keys sort while keeping repeated values in their original order. Folder names sort lexically, with subfolders before requests.

## Path values and variables

Path parameter names must match the route exactly. Metadata for a nonexistent parameter is an error. Supported names use letters, digits, underscores, dots, and hyphens, beginning with a letter or underscore.

Whole-segment path parameters become Postman URL variables such as `:id`. Their examples are local to the request: `/users/{id}` and `/orders/{id}` can have different values. Literal path example values are percent-encoded, while `{{variable}}` expressions remain available for substitution.

Embedded parameters such as `/files/{name}.{ext}` use collection variables with reserved `walker_path_` names. Their names derive deterministically from method, path, variant name, and parameter name. Their values do not leak across requests. This is a compatibility fallback for Postman's path-variable resolver.

`base_url` and `walker_path_*` are reserved collection variable names. Set the base URL through `Options.BaseURL` or `--base-url`. Other variables can be supplied through `Options.Variables`, a config file, or `--variable KEY=VALUE`.

References such as `{{token}}` found in exported requests or authentication create empty collection variables unless a value was supplied. Supported variable names follow the same naming rule as path parameters. Postman dynamic variables, such as `{{$guid}}`, are passed through in ordinary headers/bodies without defining a collection variable; they are not specially preserved by query/path encoding.

## Synthetic bodies

`walker.JSONExample(value)` builds JSON from the supplied Go type, ignoring the instance's values:

```go
sample, err := walker.JSONExample(CreateUserRequest{})
// Handle err, then attach the encoded JSON:
docs.Describe("POST", "/users", walker.RequestExample(json.RawMessage(sample)))
```

Field visibility, JSON tags, embedded fields, `omitempty`, and `,string` follow `encoding/json`. Recursive references terminate with zero values and recursion depth is bounded. Interfaces become null, maps are empty, and byte slices follow JSON's base64 representation. Custom marshalers operate on synthetic values. Unsupported types return errors.

Generated values are illustrative. Walker does not validate email formats, regex constraints, business rules, or response contracts. Use literal examples when those details matter.
