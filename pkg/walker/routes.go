// Package walker exports registered Chi routes as JSON or Postman collections.
// Construct the router before extraction; no HTTP listener is started.
package walker

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
)

// Route describes an HTTP endpoint and optional, explicitly supplied request metadata.
type Route struct {
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Summary     string            `json:"summary,omitempty"`
	Description string            `json:"description,omitempty"`
	Group       string            `json:"group,omitempty"`
	Body        any               `json:"body,omitempty"`
	Headers     []Header          `json:"headers,omitempty"`
	Query       []QueryParam      `json:"query,omitempty"`
	PathParams  map[string]string `json:"path_params,omitempty"`
	Auth        *Auth             `json:"auth,omitempty"`
	Examples    []RequestVariant  `json:"examples,omitempty"`
}

// Extract walks an initialized Chi router, including mounted routers.
// Chi does not retain request types; attach examples to the returned routes explicitly.
// Do not mutate the router concurrently with extraction.
func Extract(router chi.Router) ([]Route, error) {
	if router == nil || (reflect.ValueOf(router).Kind() == reflect.Pointer && reflect.ValueOf(router).IsNil()) {
		return nil, fmt.Errorf("router must not be nil")
	}
	routes := make([]Route, 0)
	err := chi.Walk(router, func(method, path string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes = append(routes, Route{Method: method, Path: path})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk routes: %w", err)
	}
	return Normalize(routes)
}

// Normalize validates, copies metadata containers, and sorts routes by path and method.
// Body values remain caller-owned; do not mutate them concurrently with export.
// Duplicate method/path pairs are rejected rather than silently overwritten.
func Normalize(routes []Route) ([]Route, error) {
	result := make([]Route, 0, len(routes))
	seen := make(map[string]bool, len(routes))
	for i, route := range routes {
		route.Method = strings.ToUpper(strings.TrimSpace(route.Method))
		if route.Method == "" || strings.IndexFunc(route.Method, func(r rune) bool {
			return !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", r))
		}) >= 0 {
			return nil, fmt.Errorf("route %d: invalid HTTP method %q", i+1, route.Method)
		}
		if !strings.HasPrefix(route.Path, "/") || strings.IndexFunc(route.Path, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return nil, fmt.Errorf("route %d: path must start with / and contain no whitespace", i+1)
		}
		key := route.Method + " " + route.Path
		if seen[key] {
			return nil, fmt.Errorf("duplicate route: %s", key)
		}
		seen[key] = true
		if err := validateMetadata(route); err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		route = cloneRoute(route)
		result = append(result, route)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Path == result[j].Path {
			return result[i].Method < result[j].Method
		}
		return result[i].Path < result[j].Path
	})
	return result, nil
}
