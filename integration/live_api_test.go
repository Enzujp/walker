//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enzujp/walker/pkg/walker"
	"github.com/go-chi/chi/v5"
)

type user struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type expectedResponse struct {
	Status int `json:"status"`
	JSON   any `json:"json"`
}

// TestLiveAPI exports the very router serving HTTP, then runs the collection
// through Newman's real Postman runtime. Expected responses are independent
// of the generated request document, so serialization mistakes fail the test.
func TestLiveAPI(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Fatal("Node.js and Newman are required; see docs/development.md")
	}
	router, docs := newAPI()
	server := httptest.NewServer(router)
	defer server.Close()
	routes, err := docs.Extract(router)
	if err != nil {
		t.Fatal(err)
	}
	data, err := walker.Postman(routes, walker.Options{
		Name: "Walker live API verification", BaseURL: server.URL,
		Auth:      &walker.Auth{Type: "bearer", Token: "{{token}}"},
		Variables: map[string]string{"token": "test-token", "username": "tester", "password": "test-password", "api_key": "test-key", "trace": "trace-123"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]expectedResponse{
		"GET /health — Health":                           {200, map[string]any{"healthy": true, "authorization": ""}},
		"GET /api/users/{id} — Read user / Ada":          {200, user{ID: "42", Name: "Ada", Email: "ada@example.com"}},
		"GET /api/users/{id} — Read user / Grace":        {200, user{ID: "84", Name: "Grace", Email: "grace@example.com"}},
		"GET /api/users/{id} — Read user / Unauthorized": {401, map[string]string{"error": "unauthorized"}},
		"POST /api/users — Create user / Ada":            {201, map[string]string{"name": "Ada", "email": "new-ada@example.com", "trace": "trace-123"}},
		"POST /api/users — Create user / Grace":          {201, map[string]string{"name": "Grace", "email": "new-grace@example.com", "trace": "trace-override"}},
		"POST /api/users — Create user / Invalid":        {422, map[string]string{"error": "name and valid email required"}},
		"GET /api/search — Search":                       {200, map[string]any{"q": "a&b / café", "tag": []string{"one", "two"}, "debug": false, "trace": "trace-123"}},
		"GET /api/basic — Basic auth":                    {200, map[string]string{"user": "tester"}},
		"GET /api/key-header — API key header":           {200, map[string]string{"auth": "header"}},
		"GET /api/key-query — API key query":             {200, map[string]string{"auth": "query"}},
		"GET /api/files/{name}.{ext} — File":             {200, map[string]string{"name": "quarter report", "ext": "csv"}},
		"GET /api/echo/{value} — Encoded path":           {200, map[string]string{"value": "A B/ café"}},
		"POST /api/null — Null payload / Inherit":        {200, map[string]string{"kind": "object"}},
		"POST /api/null — Null payload / Null":           {200, map[string]string{"kind": "null"}},
	}
	directory := t.TempDir()
	collectionPath := filepath.Join(directory, "collection.json")
	expectedPath := filepath.Join(directory, "expected.json")
	if err := os.WriteFile(collectionPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expectedPath, expectedJSON, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "node", "../scripts/run_live_collection.cjs", collectionPath, expectedPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("live collection run failed: %v\n%s", err, output)
	}
	t.Logf("%s", output)

	// Compare snapshots from actual router registration changes, not hand-written files.
	previous := chi.NewRouter()
	previous.Get("/health", func(http.ResponseWriter, *http.Request) {})
	previous.Get("/legacy", func(http.ResponseWriter, *http.Request) {})
	current := chi.NewRouter()
	current.Get("/health", func(http.ResponseWriter, *http.Request) {})
	current.Post("/users", func(http.ResponseWriter, *http.Request) {})
	before, err := walker.Extract(previous)
	if err != nil {
		t.Fatal(err)
	}
	after, err := walker.Extract(current)
	if err != nil {
		t.Fatal(err)
	}
	changes, err := walker.Diff(before, after)
	if err != nil || !reflect.DeepEqual(changes, walker.Changes{Added: []walker.Endpoint{{Method: "POST", Path: "/users"}}, Removed: []walker.Endpoint{{Method: "GET", Path: "/legacy"}}}) {
		t.Fatalf("router diff: %+v %v", changes, err)
	}
}

func newAPI() (*chi.Mux, *walker.Docs) {
	router := chi.NewRouter()
	api := chi.NewRouter()
	docs := &walker.Docs{}
	var mu sync.Mutex
	users := map[string]user{"42": {ID: "42", Name: "Ada", Email: "ada@example.com"}, "84": {ID: "84", Name: "Grace", Email: "grace@example.com"}}
	write := func(w http.ResponseWriter, status int, value any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(value)
	}
	bearer := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer test-token" {
				write(w, 401, map[string]string{"error": "unauthorized"})
				return
			}
			next(w, r)
		}
	}
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"healthy": true, "authorization": r.Header.Get("Authorization")})
	})
	docs.Describe("GET", "/health", walker.Summary("Health"), walker.Authentication(walker.Auth{Type: "noauth"}))
	api.Get("/users/{id}", bearer(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		value, ok := users[chi.URLParam(r, "id")]
		mu.Unlock()
		if !ok {
			write(w, 404, map[string]string{"error": "not found"})
			return
		}
		write(w, 200, value)
	}))
	docs.Describe("GET", "/api/users/{id}", walker.Summary("Read user"), walker.Group("Users"), walker.PathParam("id", "42"), walker.Examples(
		walker.RequestVariant{Name: "Ada"}, walker.RequestVariant{Name: "Grace", PathParams: map[string]string{"id": "84"}},
		walker.RequestVariant{Name: "Unauthorized", Auth: &walker.Auth{Type: "bearer", Token: "wrong-token"}},
	))
	api.Post("/users", bearer(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			write(w, 415, map[string]string{"error": "expected JSON"})
			return
		}
		var value user
		if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
			write(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if value.Name == "" || !strings.Contains(value.Email, "@") {
			write(w, 422, map[string]string{"error": "name and valid email required"})
			return
		}
		mu.Lock()
		value.ID = fmt.Sprint(len(users) + 100)
		users[value.ID] = value
		mu.Unlock()
		write(w, 201, map[string]string{"name": value.Name, "email": value.Email, "trace": r.Header.Get("X-Trace")})
	}))
	docs.Describe("POST", "/api/users", walker.Summary("Create user"), walker.Group("Users"), walker.Headers(walker.Header{Key: "X-Trace", Value: "{{trace}}"}), walker.Examples(
		walker.RequestVariant{Name: "Ada", Body: map[string]string{"name": "Ada", "email": "new-ada@example.com"}},
		walker.RequestVariant{Name: "Grace", Body: map[string]string{"name": "Grace", "email": "new-grace@example.com"}, Headers: []walker.Header{{Key: "x-trace", Value: "trace-override"}}},
		walker.RequestVariant{Name: "Invalid", Body: map[string]string{"name": "Invalid"}},
	))
	api.Get("/search", bearer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		write(w, 200, map[string]any{"q": q.Get("q"), "tag": q["tag"], "debug": q.Has("debug"), "trace": r.Header.Get("X-Trace")})
	}))
	docs.Describe("GET", "/api/search", walker.Summary("Search"), walker.Headers(walker.Header{Key: "X-Trace", Value: "{{trace}}"}), walker.Query(
		walker.QueryParam{Key: "q", Value: "a&b / café"}, walker.QueryParam{Key: "tag", Value: "one"}, walker.QueryParam{Key: "tag", Value: "two"}, walker.QueryParam{Key: "debug", Value: "true", Disabled: true},
	))
	api.Get("/basic", func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "tester" || p != "test-password" {
			write(w, 401, map[string]string{"error": "bad basic auth"})
			return
		}
		write(w, 200, map[string]string{"user": u})
	})
	docs.Describe("GET", "/api/basic", walker.Summary("Basic auth"), walker.Authentication(walker.Auth{Type: "basic", Username: "{{username}}", Password: "{{password}}"}))
	api.Get("/key-header", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "test-key" {
			write(w, 401, map[string]string{"error": "bad API key"})
			return
		}
		write(w, 200, map[string]string{"auth": "header"})
	})
	docs.Describe("GET", "/api/key-header", walker.Summary("API key header"), walker.Authentication(walker.Auth{Type: "apikey", Key: "X-API-Key", Value: "{{api_key}}", In: "header"}))
	api.Get("/key-query", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "test-key" {
			write(w, 401, map[string]string{"error": "bad API key"})
			return
		}
		write(w, 200, map[string]string{"auth": "query"})
	})
	docs.Describe("GET", "/api/key-query", walker.Summary("API key query"), walker.Authentication(walker.Auth{Type: "apikey", Key: "api_key", Value: "{{api_key}}", In: "query"}))
	api.Get("/files/{name}.{ext}", bearer(func(w http.ResponseWriter, r *http.Request) {
		name, _ := url.PathUnescape(chi.URLParam(r, "name"))
		write(w, 200, map[string]string{"name": name, "ext": chi.URLParam(r, "ext")})
	}))
	docs.Describe("GET", "/api/files/{name}.{ext}", walker.Summary("File"), walker.PathParam("name", "quarter report"), walker.PathParam("ext", "csv"))
	api.Get("/echo/{value}", bearer(func(w http.ResponseWriter, r *http.Request) {
		value, _ := url.PathUnescape(chi.URLParam(r, "value"))
		write(w, 200, map[string]string{"value": value})
	}))
	docs.Describe("GET", "/api/echo/{value}", walker.Summary("Encoded path"), walker.PathParam("value", "A B/ café"))
	api.Post("/null", bearer(func(w http.ResponseWriter, r *http.Request) {
		var value any
		if err := json.NewDecoder(r.Body).Decode(&value); err != nil {
			write(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		kind := "object"
		if value == nil {
			kind = "null"
		}
		write(w, 200, map[string]string{"kind": kind})
	}))
	docs.Describe("POST", "/api/null", walker.Summary("Null payload"), walker.RequestExample(map[string]string{"default": "value"}), walker.Examples(walker.RequestVariant{Name: "Inherit"}, walker.RequestVariant{Name: "Null", Body: json.RawMessage("null")}))
	router.Mount("/api", api)
	return router, docs
}
