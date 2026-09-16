package walker

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Router registers Chi handlers and optional Walker metadata in one place.
// Routes without options need no metadata at all. Router is an http.Handler,
// so it can be passed directly to http.Server or httptest.NewServer.
//
// Router is intended for application setup and is not safe for concurrent
// registration. Serving requests after setup follows Chi's concurrency rules.
type Router struct {
	router   chi.Router
	root     chi.Router
	docs     *Docs
	prefix   string
	defaults []RouteOption
}

var _ http.Handler = (*Router)(nil)

// NewRouter creates a documented Chi router. Defaults apply to every route.
func NewRouter(defaults ...RouteOption) *Router {
	router := chi.NewRouter()
	return newRouter(router, router, &Docs{}, "", defaults)
}

// Wrap adds Walker's concise registration API to an existing Chi router.
// Register routes through the returned Router so metadata stays in sync.
func Wrap(router chi.Router, defaults ...RouteOption) *Router {
	return newRouter(router, router, &Docs{}, "", defaults)
}

func newRouter(router, root chi.Router, docs *Docs, prefix string, defaults []RouteOption) *Router {
	result := &Router{router: router, root: root, docs: docs, prefix: prefix}
	result.defaults = append([]RouteOption(nil), defaults...)
	if isNil(router) {
		docs.setError(fmt.Errorf("router must not be nil"))
	}
	return result
}

// Chi returns the underlying router for integrations that need Chi-specific
// capabilities not exposed by this wrapper. Routes registered directly on the
// returned value are still discovered, but cannot receive inline metadata.
func (r *Router) Chi() chi.Router { return r.router }

// ServeHTTP makes Router usable anywhere an http.Handler is accepted.
func (r *Router) ServeHTTP(w http.ResponseWriter, request *http.Request) {
	if r == nil || isNil(r.router) {
		http.Error(w, "walker: nil router", http.StatusInternalServerError)
		return
	}
	r.router.ServeHTTP(w, request)
}

// Routes extracts all registered routes and attaches inline metadata.
func (r *Router) Routes() ([]Route, error) {
	if r == nil || r.docs == nil || isNil(r.root) {
		return nil, fmt.Errorf("router must not be nil")
	}
	return r.docs.Extract(r.root)
}

// Postman extracts this router and generates a collection in one call.
func (r *Router) Postman(options Options) ([]byte, error) {
	routes, err := r.Routes()
	if err != nil {
		return nil, err
	}
	return Postman(routes, options)
}

// Defaults returns a view that inherits additional metadata defaults. It uses
// the same Chi router and documentation registry; it does not create a subrouter.
func (r *Router) Defaults(options ...RouteOption) *Router {
	if r == nil {
		return (*Router)(nil)
	}
	defaults := append([]RouteOption(nil), r.defaults...)
	defaults = append(defaults, options...)
	return newRouter(r.router, r.root, r.docs, r.prefix, defaults)
}

// Route creates a nested Chi router. Defaults apply to every route registered
// inside the callback. The callback receives paths relative to pattern while
// Walker records the fully qualified path automatically.
func (r *Router) Route(pattern string, fn func(*Router), defaults ...RouteOption) {
	if !r.ready("route", pattern) {
		return
	}
	if fn == nil {
		r.docs.setError(fmt.Errorf("route %q: callback must not be nil", pattern))
		return
	}
	r.router.Route(pattern, func(child chi.Router) {
		options := append([]RouteOption(nil), r.defaults...)
		options = append(options, defaults...)
		fn(newRouter(child, r.root, r.docs, joinRoutePath(r.prefix, pattern), options))
	})
}

// Use appends middleware to this router.
func (r *Router) Use(middlewares ...func(http.Handler) http.Handler) {
	if r.ready("use middleware", "") {
		r.router.Use(middlewares...)
	}
}

// With returns a router view with inline middleware, matching chi.Router.With.
func (r *Router) With(middlewares ...func(http.Handler) http.Handler) *Router {
	if !r.ready("apply middleware", "") {
		return r
	}
	return newRouter(r.router.With(middlewares...), r.root, r.docs, r.prefix, r.defaults)
}

// Mount attaches an http.Handler. Walker discovers mounted Chi routes when the
// handler exposes chi.Routes; attach their metadata where that router is built.
func (r *Router) Mount(pattern string, handler http.Handler) {
	if !r.ready("mount", pattern) {
		return
	}
	if isNil(handler) {
		r.docs.setError(fmt.Errorf("mount %s: handler must not be nil", joinRoutePath(r.prefix, pattern)))
		return
	}
	r.router.Mount(pattern, handler)
}

// Method registers any HTTP method with optional request metadata.
func (r *Router) Method(method, pattern string, handler http.Handler, options ...RouteOption) {
	if !r.ready(method, pattern) {
		return
	}
	if isNil(handler) {
		r.docs.setError(fmt.Errorf("%s %s: handler must not be nil", method, joinRoutePath(r.prefix, pattern)))
		return
	}
	r.describe(method, pattern, options)
	if r.docs.err != nil {
		return
	}
	r.router.Method(method, pattern, handler)
}

// MethodFunc is Method for an http.HandlerFunc.
func (r *Router) MethodFunc(method, pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.Method(method, pattern, handler, options...)
}

func (r *Router) Connect(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodConnect, pattern, handler, options...)
}
func (r *Router) Delete(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodDelete, pattern, handler, options...)
}
func (r *Router) Get(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodGet, pattern, handler, options...)
}
func (r *Router) Head(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodHead, pattern, handler, options...)
}
func (r *Router) Options(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodOptions, pattern, handler, options...)
}
func (r *Router) Patch(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodPatch, pattern, handler, options...)
}
func (r *Router) Post(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodPost, pattern, handler, options...)
}
func (r *Router) Put(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodPut, pattern, handler, options...)
}
func (r *Router) Trace(pattern string, handler http.HandlerFunc, options ...RouteOption) {
	r.MethodFunc(http.MethodTrace, pattern, handler, options...)
}

func (r *Router) describe(method, pattern string, options []RouteOption) {
	if len(r.defaults) == 0 && len(options) == 0 {
		return
	}
	r.docs.describeRoute(method, joinRoutePath(r.prefix, pattern), r.defaults, options)
}

func (r *Router) ready(action, pattern string) bool {
	if r == nil || r.docs == nil {
		return false
	}
	if isNil(r.router) {
		r.docs.setError(fmt.Errorf("cannot %s %q: router must not be nil", action, pattern))
		return false
	}
	return true
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func joinRoutePath(prefix, pattern string) string {
	if prefix == "" {
		if pattern == "" {
			return "/"
		}
		return pattern
	}
	if pattern == "" || pattern == "/" {
		if pattern == "/" && !strings.HasSuffix(prefix, "/") {
			return prefix + "/"
		}
		return prefix
	}
	return strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(pattern, "/")
}
