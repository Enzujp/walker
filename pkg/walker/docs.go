package walker

import (
	"fmt"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Docs stores metadata declarations separately from the router. Its zero value
// is ready to use. Declare metadata during setup, then call Extract afterwards.
// Docs is not safe for concurrent mutation; use a separate instance per router.
type Docs struct {
	routes map[string]Route
	err    error
}

// RouteOption adds explicit metadata to one route declaration.
type RouteOption func(*Route)

// Describe declares metadata once for a fully qualified method/path pair.
// Errors are retained and returned by Extract, so setup calls need no error plumbing.
// Describe never registers a route or invokes a handler.
func (d *Docs) Describe(method, path string, options ...RouteOption) {
	d.describeRoute(method, path, nil, options)
}

func (d *Docs) describeRoute(method, path string, defaults, options []RouteOption) {
	if d.err != nil {
		return
	}
	base := Route{Method: method, Path: path}
	if !d.applyOptions(&base, defaults) {
		return
	}
	override := Route{Method: method, Path: path}
	if !d.applyOptions(&override, options) {
		return
	}
	r := override
	if len(defaults) > 0 {
		r = mergeRouteMetadata(base, override)
	}
	routes, err := Normalize([]Route{r})
	if err != nil {
		d.err = err
		return
	}
	r = routes[0]
	key := r.Method + " " + r.Path
	if d.routes == nil {
		d.routes = map[string]Route{}
	}
	if _, exists := d.routes[key]; exists {
		d.err = fmt.Errorf("duplicate metadata declaration: %s", key)
		return
	}
	d.routes[key] = r
}

func (d *Docs) applyOptions(route *Route, options []RouteOption) bool {
	for _, option := range options {
		if option == nil {
			d.setError(fmt.Errorf("%s %s: nil metadata option", route.Method, route.Path))
			return false
		}
		option(route)
	}
	return true
}

func (d *Docs) setError(err error) {
	if d != nil && d.err == nil {
		d.err = err
	}
}

// Extract discovers the router's actual routes and attaches matching declarations.
// Metadata for nonexistent routes is an error. Routes without metadata are retained.
func (d *Docs) Extract(router chi.Router) ([]Route, error) {
	if d.err != nil {
		return nil, d.err
	}
	routes, err := Extract(router)
	if err != nil {
		return nil, err
	}
	found := map[string]bool{}
	for i, r := range routes {
		key := r.Method + " " + r.Path
		found[key] = true
		if declared, ok := d.routes[key]; ok {
			routes[i] = cloneRoute(declared)
		}
	}
	var stale []string
	for key := range d.routes {
		if !found[key] {
			stale = append(stale, key)
		}
	}
	if len(stale) > 0 {
		slices.Sort(stale)
		return nil, fmt.Errorf("metadata refers to unregistered routes: %s", strings.Join(stale, ", "))
	}
	return routes, nil
}

func Summary(text string) RouteOption     { return func(r *Route) { r.Summary = text } }
func Description(text string) RouteOption { return func(r *Route) { r.Description = text } }

// Group selects nested Postman folders, for example "API/Users".
func Group(path string) RouteOption { return func(r *Route) { r.Group = path } }

// RequestExample attaches a literal default body. JSONExample can generate a synthetic body.
func RequestExample(value any) RouteOption { return func(r *Route) { r.Body = value } }
func Headers(values ...Header) RouteOption {
	return func(r *Route) { r.Headers = append(r.Headers, values...) }
}
func Query(values ...QueryParam) RouteOption {
	return func(r *Route) { r.Query = append(r.Query, values...) }
}
func PathParam(name, value string) RouteOption {
	return func(r *Route) {
		if r.PathParams == nil {
			r.PathParams = map[string]string{}
		}
		r.PathParams[name] = value
	}
}

// Authentication overrides collection authentication; Auth{Type: "noauth"} opts out.
func Authentication(auth Auth) RouteOption { return func(r *Route) { r.Auth = cloneAuth(&auth) } }

// Examples adds named variants. When present, only these variants are exported;
// the route metadata supplies their shared defaults.
func Examples(values ...RequestVariant) RouteOption {
	return func(r *Route) { r.Examples = append(r.Examples, values...) }
}

// Body is a concise alias for RequestExample.
func Body(value any) RouteOption { return RequestExample(value) }

// PathValue is a concise alias for PathParam.
func PathValue(name, value string) RouteOption { return PathParam(name, value) }

// HeaderValue adds one request header without constructing a Header value.
func HeaderValue(key, value string) RouteOption {
	return Headers(Header{Key: key, Value: value})
}

// QueryValue adds one enabled query parameter without constructing a QueryParam value.
func QueryValue(key, value string) RouteOption {
	return Query(QueryParam{Key: key, Value: value})
}

// Bearer uses Postman's bearer auth helper for this route or group.
func Bearer(token string) RouteOption {
	return Authentication(*BearerAuth(token))
}

// Basic uses Postman's basic auth helper for this route or group.
func Basic(username, password string) RouteOption {
	return Authentication(*BasicAuth(username, password))
}

// APIKey uses Postman's API-key helper. Location must be "header" or "query".
func APIKey(key, value, location string) RouteOption {
	return Authentication(*APIKeyAuth(key, value, location))
}

// NoAuth prevents this route or group from inheriting collection authentication.
func NoAuth() RouteOption { return Authentication(Auth{Type: "noauth"}) }

// BearerAuth constructs collection-level bearer authentication.
func BearerAuth(token string) *Auth { return &Auth{Type: "bearer", Token: token} }

// BasicAuth constructs collection-level basic authentication.
func BasicAuth(username, password string) *Auth {
	return &Auth{Type: "basic", Username: username, Password: password}
}

// APIKeyAuth constructs collection-level API-key authentication.
// Location must be "header" or "query".
func APIKeyAuth(key, value, location string) *Auth {
	return &Auth{Type: "apikey", Key: key, Value: value, In: location}
}

func mergeRouteMetadata(base, override Route) Route {
	result := cloneRoute(base)
	if override.Summary != "" {
		result.Summary = override.Summary
	}
	if override.Description != "" {
		result.Description = override.Description
	}
	if override.Group != "" {
		result.Group = override.Group
	}
	if override.Body != nil {
		result.Body = override.Body
	}
	if len(result.Headers) > 0 || len(override.Headers) > 0 {
		result.Headers = mergeHeaders(result.Headers, override.Headers)
	}
	if len(result.Query) > 0 || len(override.Query) > 0 {
		result.Query = mergeQuery(result.Query, override.Query)
	}
	if len(override.PathParams) > 0 {
		if result.PathParams == nil {
			result.PathParams = map[string]string{}
		}
		for name, value := range override.PathParams {
			result.PathParams[name] = value
		}
	}
	if override.Auth != nil {
		result.Auth = cloneAuth(override.Auth)
	}
	if len(override.Examples) > 0 {
		result.Examples = append([]RequestVariant(nil), override.Examples...)
	}
	result.Method = override.Method
	result.Path = override.Path
	return result
}
