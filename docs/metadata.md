# Route metadata

Metadata is explicit. Walker discovers endpoint identities from Chi; it does not infer authentication, body types, or query parameters from arbitrary handlers.

## Register once

`walker.Router` combines Chi registration and optional metadata, removing repeated method and path strings:

```go
api := walker.NewRouter()
api.Get("/health", healthHandler) // Zero Walker metadata.

api.Route("/users", func(users *walker.Router) {
    users.Get("/{id}", getUser, walker.PathValue("id", "42"))
    users.Post("/", createUser, walker.Body(CreateUserRequest{
        Name: "Ada", Email: "ada@example.com",
    }))
}, walker.Group("Users"))

routes, err := api.Routes()
collection, err := api.Postman(walker.Options{Name: "Users API"})
```

The nested callback uses relative paths. Walker calculates full paths, so moving a route group does not leave separate metadata pointing at its old location. Group defaults apply to every route inside the callback. `Defaults(...)` creates another view over the same router with extra shared metadata:

```go
admin := api.Defaults(walker.Group("Admin"), walker.Bearer("{{admin_token}}"))
admin.Get("/admin/audit", auditHandler)
admin.Get("/admin/users", usersHandler)
```

Route options override defaults. Headers merge by case-insensitive name, query values replace inherited values with the same key, path values merge by name, and route auth overrides default auth. Routes and groups retain the first setup error and return it from `Routes` or `Postman`.

`NewRouter` implements `http.Handler` and supports normal methods, middleware, nested routes, and mounts. `Chi()` exposes the underlying `chi.Router` for less-common Chi features. Routes registered directly through `Chi()` are discovered but do not receive inline metadata.

Use `walker.Wrap(existingChiRouter)` when the application already owns a router. Register new routes through the returned wrapper. The original `walker.Docs.Describe` API remains available for applications that cannot change registration calls; it requires fully qualified paths and checks stale declarations during extraction.

`/users` and `/users/` remain distinct paths because Chi treats them as distinct.

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

Concise aliases cover common declarations: `Body`, `PathValue`, `HeaderValue`, `QueryValue`, `Bearer`, `Basic`, `APIKey`, and `NoAuth`. The longer struct-based helpers remain useful when descriptions, disabled query values, or named variants are needed.

Use `Route` directly when building a JSON manifest or when an application already has its own metadata registry.

## Authentication

Authentication can be configured at the collection, route, or named-example level. A nil auth value inherits from its parent. An explicit `noauth` opts out, useful for login or health endpoints.

```go
walker.Options{Auth: walker.BearerAuth("{{token}}")}
walker.Options{Auth: walker.BasicAuth("{{username}}", "{{password}}")}
walker.Options{Auth: walker.APIKeyAuth("X-API-Key", "{{api_key}}", "header")}

api.Get("/public", publicHandler, walker.NoAuth())
api.Get("/admin", adminHandler, walker.Bearer("{{admin_token}}"))
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
api.Post("/users", createUser, walker.Body(json.RawMessage(sample)))
```

Field visibility, JSON tags, embedded fields, `omitempty`, and `,string` follow `encoding/json`. Recursive references terminate with zero values and recursion depth is bounded. Interfaces become null, maps are empty, and byte slices follow JSON's base64 representation. Custom marshalers operate on synthetic values. Unsupported types return errors.

Generated values are illustrative. Walker does not validate email formats, regex constraints, business rules, or response contracts. Use literal examples when those details matter.
