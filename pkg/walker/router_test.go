package walker

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRouterRegistersAndDocumentsOnce(t *testing.T) {
	api := NewRouter(HeaderValue("Accept", "application/json"), Bearer("{{token}}"))
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }

	api.Get("/health", handler, NoAuth())
	api.Route("/users", func(users *Router) {
		users.Get("/", handler, Summary("List users"), QueryValue("limit", "20"))
		users.Get("/{id}", handler, PathValue("id", "42"))
		users.Post("/", handler, Body(map[string]string{"name": "Ada"}))
	}, Group("Users"))

	routes, err := api.Routes()
	if err != nil {
		t.Fatal(err)
	}
	want := []Route{
		{Method: "GET", Path: "/health", Headers: []Header{{Key: "Accept", Value: "application/json"}}, Auth: &Auth{Type: "noauth"}},
		{Method: "GET", Path: "/users/", Summary: "List users", Group: "Users", Headers: []Header{{Key: "Accept", Value: "application/json"}}, Query: []QueryParam{{Key: "limit", Value: "20"}}, Auth: &Auth{Type: "bearer", Token: "{{token}}"}},
		{Method: "POST", Path: "/users/", Group: "Users", Body: map[string]string{"name": "Ada"}, Headers: []Header{{Key: "Accept", Value: "application/json"}}, Auth: &Auth{Type: "bearer", Token: "{{token}}"}},
		{Method: "GET", Path: "/users/{id}", Group: "Users", Headers: []Header{{Key: "Accept", Value: "application/json"}}, PathParams: map[string]string{"id": "42"}, Auth: &Auth{Type: "bearer", Token: "{{token}}"}},
	}
	if !reflect.DeepEqual(routes, want) {
		got, _ := json.MarshalIndent(routes, "", "  ")
		expected, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("got:\n%s\nwant:\n%s", got, expected)
	}

	for _, path := range []string{"/health", "/users/", "/users/42"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("GET %s: status %d", path, response.Code)
		}
	}
}

func TestRouterRoutesWithoutMetadataNeedNoDeclarations(t *testing.T) {
	api := NewRouter()
	api.Get("/one", func(http.ResponseWriter, *http.Request) {})
	api.Post("/two", func(http.ResponseWriter, *http.Request) {})
	routes, err := api.Routes()
	want := []Route{{Method: "GET", Path: "/one"}, {Method: "POST", Path: "/two"}}
	if err != nil || !reflect.DeepEqual(routes, want) {
		t.Fatalf("got %+v, %v; want %+v", routes, err, want)
	}
}

func TestWrapExistingRouter(t *testing.T) {
	chiRouter := chi.NewRouter()
	chiRouter.Get("/existing", func(http.ResponseWriter, *http.Request) {})
	api := Wrap(chiRouter, Group("Wrapped"))
	api.Get("/documented", func(http.ResponseWriter, *http.Request) {}, Summary("Added through Walker"))
	routes, err := api.Routes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Summary != "Added through Walker" || routes[0].Group != "Wrapped" || !reflect.DeepEqual(routes[1], Route{Method: "GET", Path: "/existing"}) {
		t.Fatalf("unexpected wrapped routes: %+v", routes)
	}
	if api.Chi() != chiRouter {
		t.Fatal("Chi did not return the wrapped router")
	}
}

func TestRouterDefaultsOverrideAndDoNotMutateParent(t *testing.T) {
	api := NewRouter(HeaderValue("X-Scope", "root"), QueryValue("lang", "en"), Group("Root"))
	admin := api.Defaults(HeaderValue("X-Scope", "admin"), QueryValue("limit", "10"), Group("Admin"))
	handler := func(http.ResponseWriter, *http.Request) {}
	admin.Get("/admin", handler, QueryValue("lang", "fr"))
	api.Get("/public", handler)

	routes, err := api.Routes()
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].Group != "Admin" || routes[0].Headers[0].Value != "admin" || !reflect.DeepEqual(routes[0].Query, []QueryParam{{Key: "lang", Value: "fr"}, {Key: "limit", Value: "10"}}) {
		t.Fatalf("admin defaults: %+v", routes[0])
	}
	if routes[1].Group != "Root" || routes[1].Headers[0].Value != "root" || !reflect.DeepEqual(routes[1].Query, []QueryParam{{Key: "lang", Value: "en"}}) {
		t.Fatalf("parent defaults mutated: %+v", routes[1])
	}
}

func TestRouterHTTPMethodsMiddlewareMountAndPostman(t *testing.T) {
	api := NewRouter()
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "yes")
			next.ServeHTTP(w, r)
		})
	})
	handler := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	api.Connect("/connect", handler)
	api.Delete("/delete", handler)
	api.Head("/head", handler)
	api.Options("/options", handler)
	api.Patch("/patch", handler)
	api.Put("/put", handler)
	api.Trace("/trace", handler)
	api.MethodFunc(http.MethodGet, "/method-func", handler)
	api.Method(http.MethodPost, "/method", http.HandlerFunc(handler), Summary("Report"))
	api.With(func(next http.Handler) http.Handler { return next }).Get("/with", handler)
	mounted := chi.NewRouter()
	mounted.Get("/child", handler)
	api.Mount("/mounted", mounted)

	routes, err := api.Routes()
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 11 {
		t.Fatalf("got %d routes: %+v", len(routes), routes)
	}
	response := httptest.NewRecorder()
	api.ServeHTTP(response, httptest.NewRequest(http.MethodPut, "/put", nil))
	if response.Code != http.StatusNoContent || response.Header().Get("X-Middleware") != "yes" {
		t.Fatalf("middleware/handler failed: %d %+v", response.Code, response.Header())
	}
	collection, err := api.Postman(Options{Name: "Inline API"})
	if err != nil || !bytes.Contains(collection, []byte(`"name": "Inline API"`)) || !bytes.Contains(collection, []byte(`"summary"`)) && !bytes.Contains(collection, []byte("Report")) {
		t.Fatalf("collection: %s, %v", collection, err)
	}
}

func TestRouterRegistrationErrorsAreDeferred(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *Router
		want  string
	}{
		{"nil wrap", func() *Router { return Wrap(nil) }, "router must not be nil"},
		{"nil route callback", func() *Router { r := NewRouter(); r.Route("/x", nil); return r }, "callback"},
		{"nil option", func() *Router {
			r := NewRouter()
			r.Get("/x", func(http.ResponseWriter, *http.Request) {}, nil)
			return r
		}, "nil metadata option"},
		{"nil handler", func() *Router { r := NewRouter(); r.Get("/x", nil); return r }, "handler"},
		{"nil mount", func() *Router { r := NewRouter(); r.Mount("/x", nil); return r }, "handler"},
		{"duplicate", func() *Router {
			r := NewRouter()
			h := func(http.ResponseWriter, *http.Request) {}
			r.Get("/x", h, Summary("one"))
			r.Get("/x", h, Summary("two"))
			return r
		}, "duplicate"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			router := test.setup()
			if _, err := router.Routes(); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("got %v, want %q", err, test.want)
			}
		})
	}
	var nilRouter *Router
	if _, err := nilRouter.Routes(); err == nil {
		t.Fatal("nil Router accepted")
	}
}

func TestJoinRoutePath(t *testing.T) {
	tests := map[string]string{
		"|/":           "/",
		"|/users":      "/users",
		"/api|":        "/api",
		"/api|/":       "/api/",
		"/api/|/users": "/api/users",
	}
	for input, want := range tests {
		prefix, pattern, _ := strings.Cut(input, "|")
		if got := joinRoutePath(prefix, pattern); got != want {
			t.Errorf("joinRoutePath(%q, %q) = %q, want %q", prefix, pattern, got, want)
		}
	}
}
