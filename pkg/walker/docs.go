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
	if d.err != nil {
		return
	}
	r := Route{Method: method, Path: path}
	for _, option := range options {
		if option == nil {
			d.err = fmt.Errorf("%s %s: nil metadata option", method, path)
			return
		}
		option(&r)
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
